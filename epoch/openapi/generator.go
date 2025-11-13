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
func (sg *SchemaGenerator) GenerateSpecForVersion(baseSpec *openapi3.T, version *epoch.Version) (*openapi3.T, error) {
	// Clone the base spec
	spec := sg.cloneSpec(baseSpec)

	// Ensure components exist (schemas already copied via cloneSpec)
	if spec.Components == nil {
		spec.Components = &openapi3.Components{}
	}
	if spec.Components.Schemas == nil {
		spec.Components.Schemas = openapi3.Schemas{}
	}

	// Track managed base schema names (to remove from non-HEAD versions)
	managedBaseNames := make(map[string]bool)

	// Get all registered types
	types := sg.getRegisteredTypes()
	versionSuffix := sg.getVersionSuffix(version)

	// Generate schemas for each type with smart merging
	// Note: We use SchemaDirectionResponse for all types. The transformer will
	// automatically use the appropriate operations (request or response) based on
	// what's defined for each type in the version changes.
	for _, typ := range types {
		typeName := typ.Name()
		managedBaseNames[typeName] = true

		// Generate schema with smart merging (uses base schema if available)
		schema, err := sg.generateSchemaForTypeAndVersion(baseSpec, typ, version)
		if err != nil {
			return nil, fmt.Errorf("failed to generate schema for %s: %w", typeName, err)
		}

		if schema != nil {
			spec.Components.Schemas[typeName+versionSuffix] = openapi3.NewSchemaRef("", schema)
		}
	}

	// Handle nested type components with smart merging
	for name, schemaRef := range sg.typeParser.GetComponents() {
		managedBaseNames[name] = true
		componentName := name
		if !version.IsHead {
			componentName += sg.getVersionSuffix(version)
		}

		// Merge with base schema if it exists (keeps extra properties from base)
		if baseSchema := sg.getBaseSchema(baseSpec, name); baseSchema != nil && schemaRef.Value != nil {
			if baseSchema.Properties == nil {
				baseSchema.Properties = make(map[string]*openapi3.SchemaRef)
			}
			// Overlay generated properties onto base schema properties
			for k, v := range schemaRef.Value.Properties {
				baseSchema.Properties[k] = v
			}
			spec.Components.Schemas[componentName] = openapi3.NewSchemaRef("", baseSchema)
		} else {
			spec.Components.Schemas[componentName] = schemaRef
		}
	}

	// Remove managed base schema names from non-HEAD versions
	// (HEAD keeps bare names, old versions only have versioned names)
	if !version.IsHead {
		for baseName := range managedBaseNames {
			delete(spec.Components.Schemas, baseName)
		}
	}

	return spec, nil
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
		for _, itemType := range endpoint.NestedArrays {
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

// getBaseSchema retrieves and clones a schema from the base spec
// Returns nil if schema doesn't exist
func (sg *SchemaGenerator) getBaseSchema(baseSpec *openapi3.T, schemaName string) *openapi3.Schema {
	if baseSpec.Components == nil || baseSpec.Components.Schemas == nil {
		return nil
	}
	if schemaRef, exists := baseSpec.Components.Schemas[schemaName]; exists && schemaRef.Value != nil {
		return CloneSchema(schemaRef.Value)
	}
	return nil
}

// generateSchemaForTypeAndVersion generates a schema for a type, using base schema if available
// Note: We always use SchemaDirectionResponse. The transformer will automatically apply the
// appropriate operations (request or response) based on what's defined for the type.
func (sg *SchemaGenerator) generateSchemaForTypeAndVersion(
	baseSpec *openapi3.T, typ reflect.Type, version *epoch.Version,
) (*openapi3.Schema, error) {
	// Try to get base schema first (preserves descriptions from swag)
	if baseSchema := sg.getBaseSchema(baseSpec, typ.Name()); baseSchema != nil {
		// Use base schema and apply transformations
		return sg.transformer.TransformSchemaForVersion(baseSchema, typ, version, SchemaDirectionResponse)
	}

	// Fallback: generate from scratch using reflection
	return sg.GetSchemaForType(typ, version, SchemaDirectionResponse)
}

// WriteVersionedSpecs writes all versioned specs to files
// filenamePattern should contain %s for version, e.g., "docs/api_%s.yaml"
func (sg *SchemaGenerator) WriteVersionedSpecs(specs map[string]*openapi3.T, filenamePattern string) error {
	return sg.writer.WriteVersionedSpecs(specs, filenamePattern)
}
