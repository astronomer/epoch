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

		// Resolve any $refs in the existing schema using other schemas from base spec
		resolvedSchema := sg.resolveRefsFromBaseSpec(existingSchema, baseSpec)

		// Determine correct direction based on type's role (request vs response)
		direction := sg.getDirectionForType(typ)

		transformedSchema, err := sg.transformer.TransformSchemaForVersion(
			resolvedSchema, typ, version, direction)
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

		schemaKey := goTypeName
		spec.Components.Schemas[schemaKey] = openapi3.NewSchemaRef("", generatedSchema)
	}

	return nil
}

// resolveRefsFromBaseSpec resolves $refs using schemas from the base spec
func (sg *SchemaGenerator) resolveRefsFromBaseSpec(schema *openapi3.Schema, baseSpec *openapi3.T) *openapi3.Schema {
	if schema == nil || baseSpec == nil || baseSpec.Components == nil || baseSpec.Components.Schemas == nil {
		return schema
	}

	// Clone the schema first
	result := CloneSchema(schema)

	// Resolve $refs in properties
	if result.Properties != nil {
		for propName, propRef := range result.Properties {
			if propRef.Ref != "" {
				// It's a $ref - resolve it from base spec
				componentName := strings.TrimPrefix(propRef.Ref, "#/components/schemas/")
				if comp, ok := baseSpec.Components.Schemas[componentName]; ok && comp.Value != nil {
					// Recursively resolve nested $refs
					resolvedProp := sg.resolveRefsFromBaseSpec(comp.Value, baseSpec)
					result.Properties[propName] = openapi3.NewSchemaRef("", resolvedProp)
				}
			} else if propRef.Value != nil {
				// Recursively resolve nested objects
				resolvedProp := sg.resolveRefsFromBaseSpec(propRef.Value, baseSpec)
				result.Properties[propName] = openapi3.NewSchemaRef("", resolvedProp)
			}
		}
	}

	// Resolve $refs in array items
	if result.Items != nil {
		if result.Items.Ref != "" {
			componentName := strings.TrimPrefix(result.Items.Ref, "#/components/schemas/")
			if comp, ok := baseSpec.Components.Schemas[componentName]; ok && comp.Value != nil {
				resolvedItems := sg.resolveRefsFromBaseSpec(comp.Value, baseSpec)
				result.Items = openapi3.NewSchemaRef("", resolvedItems)
			}
		} else if result.Items.Value != nil {
			resolvedItems := sg.resolveRefsFromBaseSpec(result.Items.Value, baseSpec)
			result.Items = openapi3.NewSchemaRef("", resolvedItems)
		}
	}

	return result
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

	// Get all components for resolving $refs
	components := sg.typeParser.GetComponents()

	// If it's a $ref, we need to get the actual schema from components
	var baseSchema *openapi3.Schema
	if schemaRef.Ref != "" {
		// Extract component name from $ref
		componentName := strings.TrimPrefix(schemaRef.Ref, "#/components/schemas/")
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

	// Resolve all $refs inline to avoid unresolved reference issues
	// when generating versioned schemas from scratch
	resolvedSchema := sg.resolveRefsInSchema(baseSchema, components)

	// Apply version transformations
	transformedSchema, err := sg.transformer.TransformSchemaForVersion(resolvedSchema, typ, version, direction)
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

// resolveRefsInSchema recursively resolves all $ref references to inline schemas
func (sg *SchemaGenerator) resolveRefsInSchema(schema *openapi3.Schema, components map[string]*openapi3.SchemaRef) *openapi3.Schema {
	if schema == nil {
		return nil
	}

	// Clone the schema to avoid modifying original
	result := CloneSchema(schema)

	// Resolve $refs in properties
	if result.Properties != nil {
		for propName, propRef := range result.Properties {
			if propRef.Ref != "" {
				// It's a $ref - resolve it
				componentName := strings.TrimPrefix(propRef.Ref, "#/components/schemas/")
				if comp, ok := components[componentName]; ok && comp.Value != nil {
					// Recursively resolve nested $refs
					resolvedProp := sg.resolveRefsInSchema(comp.Value, components)
					result.Properties[propName] = openapi3.NewSchemaRef("", resolvedProp)
				}
			} else if propRef.Value != nil {
				// Recursively resolve nested objects
				resolvedProp := sg.resolveRefsInSchema(propRef.Value, components)
				result.Properties[propName] = openapi3.NewSchemaRef("", resolvedProp)
			}
		}
	}

	// Resolve $refs in array items
	if result.Items != nil {
		if result.Items.Ref != "" {
			componentName := strings.TrimPrefix(result.Items.Ref, "#/components/schemas/")
			if comp, ok := components[componentName]; ok && comp.Value != nil {
				resolvedItems := sg.resolveRefsInSchema(comp.Value, components)
				result.Items = openapi3.NewSchemaRef("", resolvedItems)
			}
		} else if result.Items.Value != nil {
			resolvedItems := sg.resolveRefsInSchema(result.Items.Value, components)
			result.Items = openapi3.NewSchemaRef("", resolvedItems)
		}
	}

	return result
}

// getRegisteredTypes extracts all types from the endpoint registry
// including nested types discovered from struct analysis
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
			sg.collectNestedTypes(endpoint.RequestType, typeMap, &types)
		}

		if endpoint.ResponseType != nil {
			if !typeMap[endpoint.ResponseType] {
				typeMap[endpoint.ResponseType] = true
				types = append(types, endpoint.ResponseType)
			}
			sg.collectNestedTypes(endpoint.ResponseType, typeMap, &types)
		}

		for _, itemType := range endpoint.ResponseNestedArrays {
			if !typeMap[itemType] {
				typeMap[itemType] = true
				types = append(types, itemType)
			}
		}

		for _, objType := range endpoint.ResponseNestedObjects {
			if !typeMap[objType] {
				typeMap[objType] = true
				types = append(types, objType)
			}
		}

		for _, itemType := range endpoint.RequestNestedArrays {
			if !typeMap[itemType] {
				typeMap[itemType] = true
				types = append(types, itemType)
			}
		}

		for _, objType := range endpoint.RequestNestedObjects {
			if !typeMap[objType] {
				typeMap[objType] = true
				types = append(types, objType)
			}
		}
	}

	return types
}

// collectNestedTypes discovers and collects all nested types from a struct type
func (sg *SchemaGenerator) collectNestedTypes(rootType reflect.Type, typeMap map[reflect.Type]bool, types *[]reflect.Type) {
	if rootType == nil {
		return
	}

	// Dereference pointer
	if rootType.Kind() == reflect.Ptr {
		rootType = rootType.Elem()
	}

	// Only analyze struct types
	if rootType.Kind() != reflect.Struct {
		return
	}

	// Use AnalyzeStructFields to discover nested types
	nestedInfos := epoch.AnalyzeStructFields(rootType, "", nil)

	for _, info := range nestedInfos {
		if !typeMap[info.Type] {
			typeMap[info.Type] = true
			*types = append(*types, info.Type)
		}
	}
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
