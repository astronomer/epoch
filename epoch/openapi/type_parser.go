package openapi

import (
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
)

// TypeParser converts Go types to OpenAPI schemas using reflection
type TypeParser struct {
	tagParser *TagParser

	// Cache to prevent infinite recursion and reuse schemas
	cache map[reflect.Type]*openapi3.SchemaRef

	// Component definitions for reusable schemas (will be added to components/schemas)
	components map[string]*openapi3.SchemaRef

	// Track types currently being parsed to detect cycles
	parsing map[reflect.Type]bool
}

// NewTypeParser creates a new type parser
func NewTypeParser() *TypeParser {
	return &TypeParser{
		tagParser:  NewTagParser(),
		cache:      make(map[reflect.Type]*openapi3.SchemaRef),
		components: make(map[string]*openapi3.SchemaRef),
		parsing:    make(map[reflect.Type]bool),
	}
}

// ParseType converts a Go type to an OpenAPI schema
// Returns a SchemaRef which may contain either an inline schema or a $ref
func (tp *TypeParser) ParseType(t reflect.Type) (*openapi3.SchemaRef, error) {
	// Unwrap pointers
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Check cache first
	if cached, ok := tp.cache[t]; ok {
		return cached, nil
	}

	// Check for circular reference
	if tp.parsing[t] {
		// Create a forward reference
		return tp.createRef(t), nil
	}

	// Mark as being parsed
	tp.parsing[t] = true
	defer func() {
		delete(tp.parsing, t)
	}()

	// Parse based on kind
	var schemaRef *openapi3.SchemaRef
	var err error

	switch t.Kind() {
	case reflect.Bool:
		schemaRef = tp.parseBool()

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		schemaRef = tp.parseInt(t)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schemaRef = tp.parseUint(t)

	case reflect.Float32, reflect.Float64:
		schemaRef = tp.parseFloat(t)

	case reflect.String:
		schemaRef = tp.parseString()

	case reflect.Struct:
		// Check for special types first
		if t == reflect.TypeOf(time.Time{}) {
			schemaRef = tp.parseTime()
		} else {
			schemaRef, err = tp.parseStruct(t)
		}

	case reflect.Slice, reflect.Array:
		schemaRef, err = tp.parseSlice(t)

	case reflect.Map:
		schemaRef, err = tp.parseMap(t)

	case reflect.Interface:
		schemaRef = tp.parseInterface()

	default:
		return nil, fmt.Errorf("unsupported type kind: %s for type %s", t.Kind(), t.Name())
	}

	if err != nil {
		return nil, err
	}

	// Cache the result
	tp.cache[t] = schemaRef

	return schemaRef, nil
}

// parseBool creates a schema for bool types
func (tp *TypeParser) parseBool() *openapi3.SchemaRef {
	return openapi3.NewSchemaRef("", &openapi3.Schema{
		Type: &openapi3.Types{"boolean"},
	})
}

// parseInt creates a schema for integer types
func (tp *TypeParser) parseInt(t reflect.Type) *openapi3.SchemaRef {
	schema := &openapi3.Schema{
		Type: &openapi3.Types{"integer"},
	}

	// Set format based on size
	switch t.Kind() {
	case reflect.Int32:
		schema.Format = "int32"
	case reflect.Int64:
		schema.Format = "int64"
	default:
		schema.Format = "int64" // Default to int64
	}

	return openapi3.NewSchemaRef("", schema)
}

// parseUint creates a schema for unsigned integer types
func (tp *TypeParser) parseUint(t reflect.Type) *openapi3.SchemaRef {
	schema := &openapi3.Schema{
		Type: &openapi3.Types{"integer"},
	}

	// Unsigned integers - use int64 format with minimum: 0
	schema.Format = "int64"
	min := 0.0
	schema.Min = &min

	return openapi3.NewSchemaRef("", schema)
}

// parseFloat creates a schema for float types
func (tp *TypeParser) parseFloat(t reflect.Type) *openapi3.SchemaRef {
	schema := &openapi3.Schema{
		Type: &openapi3.Types{"number"},
	}

	// Set format based on size
	switch t.Kind() {
	case reflect.Float32:
		schema.Format = "float"
	case reflect.Float64:
		schema.Format = "double"
	default:
		schema.Format = "double"
	}

	return openapi3.NewSchemaRef("", schema)
}

// parseString creates a schema for string types
func (tp *TypeParser) parseString() *openapi3.SchemaRef {
	return openapi3.NewSchemaRef("", &openapi3.Schema{
		Type: &openapi3.Types{"string"},
	})
}

// parseTime creates a schema for time.Time
func (tp *TypeParser) parseTime() *openapi3.SchemaRef {
	return openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:   &openapi3.Types{"string"},
		Format: "date-time",
	})
}

// parseStruct creates a schema for struct types
func (tp *TypeParser) parseStruct(t reflect.Type) (*openapi3.SchemaRef, error) {
	// For named structs, create a component reference
	if t.Name() != "" {
		// Check if we've already created this component
		componentName := t.Name()
		if _, exists := tp.components[componentName]; exists {
			return tp.createRef(t), nil
		}

		// Create the component
		schema := &openapi3.Schema{
			Type:       &openapi3.Types{"object"},
			Properties: make(map[string]*openapi3.SchemaRef),
		}

		// Store in components first to handle circular references
		schemaRef := openapi3.NewSchemaRef("", schema)
		tp.components[componentName] = schemaRef

		// Parse all fields
		required := []string{}
		for i := 0; i < t.NumField(); i++ {
			field := t.Field(i)

			// Skip unexported fields
			if !field.IsExported() {
				continue
			}

			// Parse JSON tag
			jsonTag := field.Tag.Get("json")
			if jsonTag == "-" {
				continue // Skip this field
			}

			fieldName, omitempty := tp.tagParser.ParseJSONTag(jsonTag)
			if fieldName == "" {
				// If no json tag, use field name in lowercase
				fieldName = strings.ToLower(field.Name)
			}

			// Handle embedded/anonymous fields
			if field.Anonymous {
				// For embedded structs, promote their fields to this level
				embeddedSchema, err := tp.ParseType(field.Type)
				if err != nil {
					return nil, fmt.Errorf("failed to parse embedded field %s: %w", field.Name, err)
				}

				// Resolve the actual schema (could be a $ref)
				var actualSchema *openapi3.Schema
				if embeddedSchema.Ref != "" {
					// It's a reference, resolve it from components
					embeddedTypeName := field.Type.Name()
					if embeddedTypeName != "" {
						if component, ok := tp.components[embeddedTypeName]; ok {
							actualSchema = component.Value
						}
					}
				} else {
					// It's an inline schema
					actualSchema = embeddedSchema.Value
				}

				// Merge properties from the embedded struct
				if actualSchema != nil && actualSchema.Properties != nil {
					for k, v := range actualSchema.Properties {
						schema.Properties[k] = v
					}
					// Merge required fields
					if actualSchema.Required != nil {
						required = append(required, actualSchema.Required...)
					}
				}
				continue
			}

			// Parse field type
			fieldSchema, err := tp.ParseType(field.Type)
			if err != nil {
				return nil, fmt.Errorf("failed to parse field %s.%s: %w", t.Name(), field.Name, err)
			}

			// Apply validation tags if this is a new inline schema
			if fieldSchema.Value != nil {
				bindingTag := field.Tag.Get("binding")
				validateTag := field.Tag.Get("validate")
				tp.tagParser.ApplyValidationTags(fieldSchema.Value, bindingTag, validateTag)
				tp.tagParser.ApplyCommonTags(fieldSchema.Value, field)

				// Check if required
				if tp.tagParser.IsRequired(bindingTag, validateTag, omitempty) {
					required = append(required, fieldName)
				}
			}

			schema.Properties[fieldName] = fieldSchema
		}

		// Set required fields
		if len(required) > 0 {
			schema.Required = required
		}

		return tp.createRef(t), nil
	}

	// For anonymous structs, create inline schema
	schema := &openapi3.Schema{
		Type:       &openapi3.Types{"object"},
		Properties: make(map[string]*openapi3.SchemaRef),
	}

	required := []string{}
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		if !field.IsExported() {
			continue
		}

		jsonTag := field.Tag.Get("json")
		if jsonTag == "-" {
			continue
		}

		fieldName, omitempty := tp.tagParser.ParseJSONTag(jsonTag)
		if fieldName == "" {
			fieldName = strings.ToLower(field.Name)
		}

		fieldSchema, err := tp.ParseType(field.Type)
		if err != nil {
			return nil, err
		}

		if fieldSchema.Value != nil {
			bindingTag := field.Tag.Get("binding")
			validateTag := field.Tag.Get("validate")
			tp.tagParser.ApplyValidationTags(fieldSchema.Value, bindingTag, validateTag)
			tp.tagParser.ApplyCommonTags(fieldSchema.Value, field)

			if tp.tagParser.IsRequired(bindingTag, validateTag, omitempty) {
				required = append(required, fieldName)
			}
		}

		schema.Properties[fieldName] = fieldSchema
	}

	if len(required) > 0 {
		schema.Required = required
	}

	return openapi3.NewSchemaRef("", schema), nil
}

// parseSlice creates a schema for slice/array types
func (tp *TypeParser) parseSlice(t reflect.Type) (*openapi3.SchemaRef, error) {
	// Get element type
	elemType := t.Elem()
	elemSchema, err := tp.ParseType(elemType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse array element type: %w", err)
	}

	schema := &openapi3.Schema{
		Type:  &openapi3.Types{"array"},
		Items: elemSchema,
	}

	// If it's an array (fixed size), set min/max items
	if t.Kind() == reflect.Array {
		length := uint64(t.Len())
		schema.MinItems = length
		schema.MaxItems = &length
	}

	return openapi3.NewSchemaRef("", schema), nil
}

// parseMap creates a schema for map types
func (tp *TypeParser) parseMap(t reflect.Type) (*openapi3.SchemaRef, error) {
	// Only support map[string]T
	if t.Key().Kind() != reflect.String {
		return nil, fmt.Errorf("only map[string]T is supported, got map[%s]T", t.Key().Kind())
	}

	// Get value type
	valueType := t.Elem()
	valueSchema, err := tp.ParseType(valueType)
	if err != nil {
		return nil, fmt.Errorf("failed to parse map value type: %w", err)
	}

	schema := &openapi3.Schema{
		Type:                 &openapi3.Types{"object"},
		AdditionalProperties: openapi3.AdditionalProperties{Schema: valueSchema},
	}

	return openapi3.NewSchemaRef("", schema), nil
}

// parseInterface creates a schema for interface{} types
func (tp *TypeParser) parseInterface() *openapi3.SchemaRef {
	// interface{} is represented as a free-form object
	return openapi3.NewSchemaRef("", &openapi3.Schema{
		Type: &openapi3.Types{"object"},
	})
}

// createRef creates a $ref reference to a component schema
func (tp *TypeParser) createRef(t reflect.Type) *openapi3.SchemaRef {
	componentName := t.Name()
	ref := fmt.Sprintf("#/components/schemas/%s", componentName)
	return &openapi3.SchemaRef{
		Ref: ref,
	}
}

// GetComponents returns all component schemas collected during parsing
func (tp *TypeParser) GetComponents() map[string]*openapi3.SchemaRef {
	return tp.components
}

// Reset clears the parser's cache and components (useful for generating multiple versions)
func (tp *TypeParser) Reset() {
	tp.cache = make(map[reflect.Type]*openapi3.SchemaRef)
	tp.components = make(map[string]*openapi3.SchemaRef)
	tp.parsing = make(map[reflect.Type]bool)
}
