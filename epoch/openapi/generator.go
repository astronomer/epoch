package openapi

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
)

// SchemaGenerator generates versioned OpenAPI specs from Epoch type registry and migrations
type SchemaGenerator struct {
	config      *SchemaGeneratorConfig
	typeParser  *TypeParser
	transformer *VersionTransformer
	writer      *Writer

	// Cache of generated schemas per version per type
	schemaCache map[string]map[reflect.Type]*openapi3.SchemaRef
}

// NewSchemaGenerator creates a new schema generator
func NewSchemaGenerator(config SchemaGeneratorConfig) *SchemaGenerator {
	if config.OutputFormat == "" {
		config.OutputFormat = "yaml"
	}

	// Default SchemaNameMapper to identity function
	if config.SchemaNameMapper == nil {
		config.SchemaNameMapper = func(name string) string { return name }
	}

	return &SchemaGenerator{
		config:      &config,
		typeParser:  NewTypeParser(),
		transformer: NewVersionTransformer(config.VersionBundle),
		writer:      NewWriter(config.OutputFormat),
		schemaCache: make(map[string]map[reflect.Type]*openapi3.SchemaRef),
	}
}

// GenerateVersionedSpecs generates OpenAPI specs for all versions in the version bundle
// It takes a base spec (typically the HEAD version from swag) and generates versioned variants
func (sg *SchemaGenerator) GenerateVersionedSpecs(baseSpec *openapi3.T) (map[string]*openapi3.T, error) {
	result := make(map[string]*openapi3.T)

	// Generate spec for HEAD version
	headVersion := sg.config.VersionBundle.GetHeadVersion()
	headSpec, err := sg.GenerateSpecForVersion(baseSpec, headVersion)
	if err != nil {
		return nil, fmt.Errorf("failed to generate HEAD spec: %w", err)
	}
	result[headVersion.String()] = headSpec

	// Generate specs for all other versions
	for _, version := range sg.config.VersionBundle.GetVersions() {
		spec, err := sg.GenerateSpecForVersion(baseSpec, version)
		if err != nil {
			return nil, fmt.Errorf("failed to generate spec for version %s: %w", version.String(), err)
		}
		result[version.String()] = spec
	}

	return result, nil
}

// GenerateSpecForVersion generates an OpenAPI spec for a specific version
// Uses smart transform: transforms existing schemas, generates missing ones
func (sg *SchemaGenerator) GenerateSpecForVersion(baseSpec *openapi3.T, version *epoch.Version) (*openapi3.T, error) {
	// Clone the base spec
	spec := sg.cloneSpec(baseSpec)

	// Ensure components exist
	if spec.Components == nil {
		spec.Components = &openapi3.Components{}
	}
	if spec.Components.Schemas == nil {
		spec.Components.Schemas = openapi3.Schemas{}
	}

	// Get all registered types
	types := sg.getRegisteredTypes()

	// Process each registered type
	for _, typ := range types {
		if err := sg.processTypeForVersion(baseSpec, spec, typ, version); err != nil {
			return nil, err
		}
	}

	return spec, nil
}

// processTypeForVersion handles a single type with smart transform logic
func (sg *SchemaGenerator) processTypeForVersion(
	baseSpec *openapi3.T,
	spec *openapi3.T,
	typ reflect.Type,
	version *epoch.Version,
) error {
	goTypeName := typ.Name()

	// Map to schema name in spec (e.g., "versionedapi.UpdateExampleRequest")
	mappedSchemaName := sg.config.SchemaNameMapper(goTypeName)

	// Try to find existing schema in base spec
	existingSchema := sg.findSchemaInSpec(baseSpec, mappedSchemaName)

	if existingSchema != nil {
		// TRANSFORM PATH: Schema exists, transform it in place

		// Determine correct direction based on type's role (request vs response)
		direction := sg.getDirectionForType(typ)

		transformedSchema, err := sg.transformer.TransformSchemaForVersion(
			existingSchema, typ, version, direction)
		if err != nil {
			return fmt.Errorf("failed to transform schema %s: %w", mappedSchemaName, err)
		}

		// Replace with same name (preserves endpoint references)
		spec.Components.Schemas[mappedSchemaName] = openapi3.NewSchemaRef("", transformedSchema)
	} else {
		// FALLBACK PATH: Schema doesn't exist, generate from scratch

		// Determine correct direction based on type's role (request vs response)
		direction := sg.getDirectionForType(typ)

		generatedSchema, err := sg.GetSchemaForType(typ, version, direction)
		if err != nil {
			return fmt.Errorf("failed to generate schema for %s: %w", goTypeName, err)
		}

		// Use versioned name for generated schemas
		schemaKey := goTypeName
		if !version.IsHead {
			schemaKey += sg.getVersionSuffix(version)
		}
		spec.Components.Schemas[schemaKey] = openapi3.NewSchemaRef("", generatedSchema)
	}

	return nil
}

// findSchemaInSpec looks for a schema by name in the base spec
// Returns cloned schema if found, nil if not found
func (sg *SchemaGenerator) findSchemaInSpec(spec *openapi3.T, schemaName string) *openapi3.Schema {
	if spec == nil || spec.Components == nil || spec.Components.Schemas == nil {
		return nil
	}

	schemaRef, exists := spec.Components.Schemas[schemaName]
	if !exists || schemaRef.Value == nil {
		return nil
	}

	// Return a clone to avoid modifying the original
	return CloneSchema(schemaRef.Value)
}

// GetSchemaForType generates a schema for a specific type at a specific version and direction
func (sg *SchemaGenerator) GetSchemaForType(
	typ reflect.Type,
	version *epoch.Version,
	direction SchemaDirection,
) (*openapi3.Schema, error) {
	// Check cache
	versionKey := version.String()
	if schemas, ok := sg.schemaCache[versionKey]; ok {
		if cached, ok := schemas[typ]; ok && cached.Value != nil {
			return cached.Value, nil
		}
	}

	// Parse the HEAD version schema first
	sg.typeParser.Reset() // Reset to get fresh schema
	schemaRef, err := sg.typeParser.ParseType(typ)
	if err != nil {
		return nil, fmt.Errorf("failed to parse type: %w", err)
	}

	// If it's a $ref, we need to get the actual schema from components
	var baseSchema *openapi3.Schema
	if schemaRef.Ref != "" {
		// Extract component name from $ref
		componentName := strings.TrimPrefix(schemaRef.Ref, "#/components/schemas/")
		components := sg.typeParser.GetComponents()
		if comp, ok := components[componentName]; ok && comp.Value != nil {
			baseSchema = comp.Value
		} else {
			return nil, fmt.Errorf("component %s not found", componentName)
		}
	} else {
		baseSchema = schemaRef.Value
	}

	if baseSchema == nil {
		return nil, fmt.Errorf("no schema found for type %s", typ.Name())
	}

	// Apply version transformations
	transformedSchema, err := sg.transformer.TransformSchemaForVersion(baseSchema, typ, version, direction)
	if err != nil {
		return nil, fmt.Errorf("failed to transform schema: %w", err)
	}

	// Cache the result
	if sg.schemaCache[versionKey] == nil {
		sg.schemaCache[versionKey] = make(map[reflect.Type]*openapi3.SchemaRef)
	}
	sg.schemaCache[versionKey][typ] = openapi3.NewSchemaRef("", transformedSchema)

	return transformedSchema, nil
}

// getRegisteredTypes extracts all types from the endpoint registry
func (sg *SchemaGenerator) getRegisteredTypes() []reflect.Type {
	typeMap := make(map[reflect.Type]bool)
	var types []reflect.Type

	// Get all endpoints from registry
	endpoints := sg.config.TypeRegistry.GetAll()

	for _, endpoint := range endpoints {
		if endpoint.RequestType != nil {
			if !typeMap[endpoint.RequestType] {
				typeMap[endpoint.RequestType] = true
				types = append(types, endpoint.RequestType)
			}
		}

		if endpoint.ResponseType != nil {
			if !typeMap[endpoint.ResponseType] {
				typeMap[endpoint.ResponseType] = true
				types = append(types, endpoint.ResponseType)
			}
		}

		// Also collect nested array types
		for _, itemType := range endpoint.ResponseNestedArrays {
			if !typeMap[itemType] {
				typeMap[itemType] = true
				types = append(types, itemType)
			}
		}
	}

	return types
}

// getVersionSuffix returns a suffix for versioned schema names
// e.g., "V20240101" for date "2024-01-01"
func (sg *SchemaGenerator) getVersionSuffix(version *epoch.Version) string {
	if version.IsHead {
		return ""
	}

	// Remove hyphens and special characters from version string
	versionStr := version.String()
	versionStr = strings.ReplaceAll(versionStr, "-", "")
	versionStr = strings.ReplaceAll(versionStr, ".", "")
	versionStr = strings.ReplaceAll(versionStr, "v", "")

	// Add prefix
	if sg.config.ComponentNamePrefix != "" {
		return fmt.Sprintf("%sV%s", sg.config.ComponentNamePrefix, versionStr)
	}

	return fmt.Sprintf("V%s", versionStr)
}

// cloneSpec creates a shallow clone of an OpenAPI spec
// We clone to avoid modifying the original base spec
func (sg *SchemaGenerator) cloneSpec(original *openapi3.T) *openapi3.T {
	clone := &openapi3.T{
		OpenAPI:      original.OpenAPI,
		Info:         original.Info,
		Servers:      original.Servers,
		Paths:        original.Paths,
		Security:     original.Security,
		Tags:         original.Tags,
		ExternalDocs: original.ExternalDocs,
	}

	// Clone components, preserving schemas from base spec
	clone.Components = &openapi3.Components{
		Schemas:         sg.copySchemas(original.Components),
		Parameters:      original.Components.Parameters,
		Headers:         original.Components.Headers,
		RequestBodies:   original.Components.RequestBodies,
		Responses:       original.Components.Responses,
		SecuritySchemes: original.Components.SecuritySchemes,
		Examples:        original.Components.Examples,
		Links:           original.Components.Links,
		Callbacks:       original.Components.Callbacks,
	}

	return clone
}

// copySchemas safely copies schemas from original components
func (sg *SchemaGenerator) copySchemas(originalComponents *openapi3.Components) openapi3.Schemas {
	schemas := openapi3.Schemas{}
	if originalComponents != nil && originalComponents.Schemas != nil {
		for name, schemaRef := range originalComponents.Schemas {
			schemas[name] = schemaRef
		}
	}
	return schemas
}

// WriteVersionedSpecs writes all versioned specs to files
// filenamePattern should contain %s for version, e.g., "docs/api_%s.yaml"
func (sg *SchemaGenerator) WriteVersionedSpecs(specs map[string]*openapi3.T, filenamePattern string) error {
	return sg.writer.WriteVersionedSpecs(specs, filenamePattern)
}

// getDirectionForType determines if a type is used as request or response
// by checking the endpoint registry
func (sg *SchemaGenerator) getDirectionForType(typ reflect.Type) SchemaDirection {
	// Check endpoint registry to determine type's role
	for _, endpoint := range sg.config.TypeRegistry.GetAll() {
		if endpoint.RequestType == typ {
			return SchemaDirectionRequest
		}
		if endpoint.ResponseType == typ {
			return SchemaDirectionResponse
		}
	}

	// Default to response if unknown (safer default)
	return SchemaDirectionResponse
}
