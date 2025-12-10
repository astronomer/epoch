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

	// Nested type registry for two-pass generation
	// Maps version -> type -> component name
	nestedTypeRegistry map[string]map[reflect.Type]string

	// Track which types need component schemas generated
	typesToGenerate map[string][]reflect.Type
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
		config:             &config,
		typeParser:         NewTypeParser(),
		transformer:        NewVersionTransformer(config.VersionBundle),
		writer:             NewWriter(config.OutputFormat),
		nestedTypeRegistry: make(map[string]map[reflect.Type]string),
		typesToGenerate:    make(map[string][]reflect.Type),
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
// Implements two-pass approach: collect all types first, then generate with refs
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

	// PASS 1: Collect all types that need component schemas (including nested types)
	for _, typ := range types {
		sg.collectNestedTypesForGeneration(typ, version)
	}

	// PASS 2: Generate component schemas for all nested types in three sub-passes
	// Sub-pass 2a: Parse and store base schemas (no refs, no transformations)
	versionKey := version.String()
	baseSchemas := make(map[reflect.Type]*openapi3.Schema)

	for _, nestedType := range sg.typesToGenerate[versionKey] {
		// Parse the base schema
		sg.typeParser.Reset()
		schemaRef, err := sg.typeParser.ParseType(nestedType)
		if err != nil {
			return nil, fmt.Errorf("failed to parse nested type %s: %w", nestedType.Name(), err)
		}

		// Get the base schema
		var baseSchema *openapi3.Schema
		if schemaRef.Ref != "" {
			components := sg.typeParser.GetComponents()
			componentName := strings.TrimPrefix(schemaRef.Ref, "#/components/schemas/")
			if comp, ok := components[componentName]; ok && comp.Value != nil {
				baseSchema = comp.Value
			}
		} else {
			baseSchema = schemaRef.Value
		}

		if baseSchema != nil {
			baseSchemas[nestedType] = CloneSchema(baseSchema)
		}
	}

	// Sub-pass 2b: Add all base schemas to components (so refs can resolve in PASS 4)
	for nestedType, schema := range baseSchemas {
		componentName := sg.getComponentNameForType(versionKey, nestedType)
		if componentName == "" {
			continue
		}

		// Add the base schema to components
		spec.Components.Schemas[componentName] = openapi3.NewSchemaRef("", schema)
	}

	// Sub-pass 2c: Apply transformations to all schemas (refs will be replaced in PASS 4)
	for nestedType, schema := range baseSchemas {
		componentName := sg.getComponentNameForType(versionKey, nestedType)
		if componentName == "" {
			continue
		}

		// For nested types, apply BOTH request and response transformations to create
		// a superset schema that works in both contexts. This handles cases where the
		// same nested type appears in both request and response types.
		transformedSchema := CloneSchema(schema)

		// Apply request transformations
		transformedSchema, err := sg.transformer.TransformSchemaForVersion(transformedSchema, nestedType, version, SchemaDirectionRequest)
		if err != nil {
			return nil, fmt.Errorf("failed to transform nested schema %s (request): %w", nestedType.Name(), err)
		}

		// Apply response transformations on top
		transformedSchema, err = sg.transformer.TransformSchemaForVersion(transformedSchema, nestedType, version, SchemaDirectionResponse)
		if err != nil {
			return nil, fmt.Errorf("failed to transform nested schema %s (response): %w", nestedType.Name(), err)
		}

		// Update the component with the transformed schema
		spec.Components.Schemas[componentName] = openapi3.NewSchemaRef("", transformedSchema)
	}

	// PASS 3: Process top-level registered types (without ref replacement yet)
	for _, typ := range types {
		if err := sg.processTypeForVersion(baseSpec, spec, typ, version); err != nil {
			return nil, err
		}
	}

	// PASS 4: Now that all components exist, replace nested schemas with refs in ALL schemas
	for componentName, schemaRef := range spec.Components.Schemas {
		if schemaRef == nil || schemaRef.Value == nil {
			continue
		}

		// Replace any remaining inline schemas with refs
		if err := sg.replaceNestedSchemasWithRefsGeneric(schemaRef.Value, spec); err != nil {
			return nil, fmt.Errorf("failed to replace refs in %s: %w", componentName, err)
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
	// Skip slice/array types - they don't become named schemas
	// Their element types are handled separately in getRegisteredTypes()
	if typ.Kind() == reflect.Slice || typ.Kind() == reflect.Array {
		return nil
	}

	goTypeName := typ.Name()

	// Map to schema name in spec (e.g., "versionedapi.UpdateExampleRequest")
	mappedSchemaName := sg.config.SchemaNameMapper(goTypeName)

	// Try to find existing schema in base spec
	existingSchema := sg.findSchemaInSpec(baseSpec, mappedSchemaName)

	if existingSchema != nil {
		// TRANSFORM PATH: Schema exists, transform it in place

		// Clone the schema
		clonedSchema := CloneSchema(existingSchema)

		// DON'T replace refs here - will be done in PASS 4 after all components exist

		// Determine correct direction based on type's role (request vs response)
		direction := sg.getDirectionForType(typ)

		// Apply transformations
		transformedSchema, err := sg.transformer.TransformSchemaForVersion(
			clonedSchema, typ, version, direction)
		if err != nil {
			return fmt.Errorf("failed to transform schema %s: %w", mappedSchemaName, err)
		}

		// Replace with same name (preserves endpoint references)
		spec.Components.Schemas[mappedSchemaName] = openapi3.NewSchemaRef("", transformedSchema)
	} else {
		// FALLBACK PATH: Schema doesn't exist in base spec

		schemaKey := goTypeName

		if _, exists := spec.Components.Schemas[schemaKey]; exists {
			// Already generated as a nested type in PASS 2, skip to avoid duplication
			return nil
		}

		// Determine correct direction based on type's role (request vs response)
		direction := sg.getDirectionForType(typ)

		// Generate schema (without refs - will be done in PASS 4)
		generatedSchema, err := sg.generateSchemaWithoutRefs(typ, version, direction)
		if err != nil {
			return fmt.Errorf("failed to generate schema for %s: %w", goTypeName, err)
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

// getRegisteredTypes extracts all types from the endpoint registry
// including nested types discovered from struct analysis
func (sg *SchemaGenerator) getRegisteredTypes() []reflect.Type {
	typeMap := make(map[reflect.Type]bool)
	var types []reflect.Type

	// Get all endpoints from registry
	endpoints := sg.config.TypeRegistry.GetAll()

	for _, endpoint := range endpoints {
		if endpoint.RequestType != nil {
			reqType := endpoint.RequestType
			// For slice/array types, register the element type instead
			if reqType.Kind() == reflect.Slice || reqType.Kind() == reflect.Array {
				elemType := reqType.Elem()
				if elemType.Kind() == reflect.Ptr {
					elemType = elemType.Elem()
				}
				// Only add element type if it's a struct (not primitives)
				if elemType.Kind() == reflect.Struct && !typeMap[elemType] {
					typeMap[elemType] = true
					types = append(types, elemType)
				}
				sg.collectNestedTypes(elemType, typeMap, &types)
			} else {
				// Existing logic for non-array types
				if !typeMap[reqType] {
					typeMap[reqType] = true
					types = append(types, reqType)
				}
				sg.collectNestedTypes(reqType, typeMap, &types)
			}
		}

		if endpoint.ResponseType != nil {
			respType := endpoint.ResponseType
			// For slice/array types, register the element type instead
			if respType.Kind() == reflect.Slice || respType.Kind() == reflect.Array {
				elemType := respType.Elem()
				if elemType.Kind() == reflect.Ptr {
					elemType = elemType.Elem()
				}
				// Only add element type if it's a struct (not primitives)
				if elemType.Kind() == reflect.Struct && !typeMap[elemType] {
					typeMap[elemType] = true
					types = append(types, elemType)
				}
				sg.collectNestedTypes(elemType, typeMap, &types)
			} else {
				// Existing logic for non-array types
				if !typeMap[respType] {
					typeMap[respType] = true
					types = append(types, respType)
				}
				sg.collectNestedTypes(respType, typeMap, &types)
			}
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

// registerNestedType registers a nested type for component generation
func (sg *SchemaGenerator) registerNestedType(version string, typ reflect.Type, componentName string) {
	if sg.nestedTypeRegistry[version] == nil {
		sg.nestedTypeRegistry[version] = make(map[reflect.Type]string)
	}
	sg.nestedTypeRegistry[version][typ] = componentName

	// Track for generation if not already tracked
	found := false
	for _, t := range sg.typesToGenerate[version] {
		if t == typ {
			found = true
			break
		}
	}
	if !found {
		sg.typesToGenerate[version] = append(sg.typesToGenerate[version], typ)
	}
}

// getComponentNameForType returns the component name for a type, or empty if not registered
func (sg *SchemaGenerator) getComponentNameForType(version string, typ reflect.Type) string {
	if sg.nestedTypeRegistry[version] == nil {
		return ""
	}
	return sg.nestedTypeRegistry[version][typ]
}

// generateComponentNameForType generates a component name for a nested type
func (sg *SchemaGenerator) generateComponentNameForType(typ reflect.Type) string {
	// Dereference pointer
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Get base type name
	typeName := typ.Name()

	// Handle anonymous types
	if typeName == "" {
		switch typ.Kind() {
		case reflect.Slice, reflect.Array:
			elemType := typ.Elem()
			elemName := sg.generateComponentNameForType(elemType)
			return elemName + "Array"
		case reflect.Struct:
			// Generate synthetic name based on package path
			if typ.PkgPath() != "" {
				parts := strings.Split(typ.PkgPath(), "/")
				pkgName := parts[len(parts)-1]
				return pkgName + "AnonymousStruct"
			}
			return "AnonymousStruct"
		default:
			return "AnonymousType"
		}
	}

	return typeName
}

// collectNestedTypesForGeneration collects all nested types that need component generation
// Uses visited map to prevent infinite recursion on circular dependencies
func (sg *SchemaGenerator) collectNestedTypesForGeneration(typ reflect.Type, version *epoch.Version) {
	// Use a visited map to prevent infinite recursion
	visited := make(map[reflect.Type]bool)
	sg.collectNestedTypesRecursive(typ, version, visited)
}

// collectNestedTypesRecursive is the internal recursive implementation with cycle detection
func (sg *SchemaGenerator) collectNestedTypesRecursive(typ reflect.Type, version *epoch.Version, visited map[reflect.Type]bool) {
	if typ == nil {
		return
	}

	// Dereference pointer
	if typ.Kind() == reflect.Ptr {
		typ = typ.Elem()
	}

	// Check if we've already visited this type (cycle detection)
	if visited[typ] {
		return
	}
	visited[typ] = true

	// Only process struct types for nested discovery
	if typ.Kind() != reflect.Struct {
		return
	}

	versionKey := version.String()

	// Get nested arrays and objects from the type
	nestedArrays, nestedObjects := epoch.BuildNestedTypeMaps(typ)

	// Register nested arrays
	for _, arrayItemType := range nestedArrays {
		// Dereference if pointer
		itemType := arrayItemType
		if itemType.Kind() == reflect.Ptr {
			itemType = itemType.Elem()
		}

		// Generate component name
		componentName := sg.generateComponentNameForType(itemType)
		sg.registerNestedType(versionKey, itemType, componentName)

		// Recursively collect nested types (with cycle detection)
		sg.collectNestedTypesRecursive(itemType, version, visited)
	}

	// Register nested objects
	for _, objectType := range nestedObjects {
		// Dereference if pointer
		objType := objectType
		if objType.Kind() == reflect.Ptr {
			objType = objType.Elem()
		}

		// Generate component name
		componentName := sg.generateComponentNameForType(objType)
		sg.registerNestedType(versionKey, objType, componentName)

		// Recursively collect nested types (with cycle detection)
		sg.collectNestedTypesRecursive(objType, version, visited)
	}
}

// generateSchemaWithoutRefs generates a schema WITHOUT replacing nested schemas with refs
// This is used for initial generation before refs are available
func (sg *SchemaGenerator) generateSchemaWithoutRefs(
	typ reflect.Type,
	version *epoch.Version,
	direction SchemaDirection,
) (*openapi3.Schema, error) {
	// Parse the HEAD version schema first
	sg.typeParser.Reset()
	schemaRef, err := sg.typeParser.ParseType(typ)
	if err != nil {
		return nil, fmt.Errorf("failed to parse type: %w", err)
	}

	// Get the base schema
	var baseSchema *openapi3.Schema
	if schemaRef.Ref != "" {
		// Extract component name from $ref
		components := sg.typeParser.GetComponents()
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

	// Clone the schema
	schema := CloneSchema(baseSchema)

	// Apply version transformations (without ref replacement)
	transformedSchema, err := sg.transformer.TransformSchemaForVersion(schema, typ, version, direction)
	if err != nil {
		return nil, fmt.Errorf("failed to transform schema: %w", err)
	}

	return transformedSchema, nil
}

// replaceNestedSchemasWithRefsGeneric replaces inline nested schemas with $ref pointers
// This version doesn't require the parent Go type - it infers refs from inline object/array schemas
func (sg *SchemaGenerator) replaceNestedSchemasWithRefsGeneric(
	schema *openapi3.Schema,
	spec *openapi3.T,
) error {
	if schema == nil {
		return nil
	}

	if schema.Properties == nil {
		return nil
	}

	// Scan properties for inline object schemas that should be refs
	for propName, propRef := range schema.Properties {
		if propRef == nil {
			continue
		}

		// Skip if it's already a $ref
		if propRef.Ref != "" {
			continue
		}

		propSchema := propRef.Value
		if propSchema == nil {
			continue
		}

		// Check if it's an inline object schema
		if propSchema.Type != nil && len(*propSchema.Type) > 0 {
			typeStr := (*propSchema.Type)[0]

			if typeStr == "object" && propSchema.Properties != nil {
				// This is an inline object - try to find a matching component
				// Look for components that match this schema structure
				matchingComponentName := sg.findMatchingComponent(propSchema, spec)
				if matchingComponentName != "" {
					// Replace with $ref
					schema.Properties[propName] = &openapi3.SchemaRef{
						Ref: fmt.Sprintf("#/components/schemas/%s", matchingComponentName),
					}
				}
			} else if typeStr == "array" && propSchema.Items != nil && propSchema.Items.Value != nil {
				// Check if array items are inline objects
				itemSchema := propSchema.Items.Value
				if itemSchema.Type != nil && len(*itemSchema.Type) > 0 && (*itemSchema.Type)[0] == "object" {
					// Inline object in array - try to find matching component
					matchingComponentName := sg.findMatchingComponent(itemSchema, spec)
					if matchingComponentName != "" {
						// Replace items with $ref
						propSchema.Items = &openapi3.SchemaRef{
							Ref: fmt.Sprintf("#/components/schemas/%s", matchingComponentName),
						}
					}
				}
			}
		}
	}

	return nil
}

// findMatchingComponent finds a component schema that matches the given inline schema structure
// This is used to deduplicate inline schemas by finding their component equivalents
//
// LIMITATION: This uses a property-name-only heuristic which could produce false positives.
// If two different types have identical property names but different types (e.g., User.id: int
// vs Product.id: string), they could incorrectly match. In practice, this is rare because:
// - The TypeParser generates component schemas for named Go types, preserving type identity
// - Inline schemas only occur for anonymous structs or when parsing fails
// - Property name collisions across different business domains are uncommon
//
// If false positives occur, the workaround is to ensure all nested types are named Go structs
// rather than anonymous inline definitions.
func (sg *SchemaGenerator) findMatchingComponent(
	inlineSchema *openapi3.Schema,
	spec *openapi3.T,
) string {
	if inlineSchema == nil || inlineSchema.Properties == nil {
		return ""
	}

	// Get the property names from the inline schema
	inlineProps := make(map[string]bool)
	for propName := range inlineSchema.Properties {
		inlineProps[propName] = true
	}

	// Search components for a matching schema
	for componentName, componentRef := range spec.Components.Schemas {
		if componentRef == nil || componentRef.Value == nil || componentRef.Value.Properties == nil {
			continue
		}

		// Check if properties match
		componentProps := make(map[string]bool)
		for propName := range componentRef.Value.Properties {
			componentProps[propName] = true
		}

		// Simple heuristic: if property names match, it's likely the same type
		if len(inlineProps) == len(componentProps) {
			allMatch := true
			for propName := range inlineProps {
				if !componentProps[propName] {
					allMatch = false
					break
				}
			}
			if allMatch {
				return componentName
			}
		}
	}

	return ""
}
