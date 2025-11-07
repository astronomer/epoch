package openapi

import (
	"reflect"
	"strconv"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
)

// TagParser extracts OpenAPI metadata from Go struct tags
type TagParser struct {
	// No configuration needed yet
}

// NewTagParser creates a new tag parser
func NewTagParser() *TagParser {
	return &TagParser{}
}

// ParseJSONTag parses a json struct tag and returns the field name and omitempty flag
// Examples:
//
//	json:"field_name" → "field_name", false
//	json:"field_name,omitempty" → "field_name", true
//	json:"-" → "-", false (skip field)
func (tp *TagParser) ParseJSONTag(tag string) (fieldName string, omitempty bool) {
	if tag == "" {
		return "", false
	}

	parts := strings.Split(tag, ",")
	fieldName = parts[0]

	// Check for omitempty
	for i := 1; i < len(parts); i++ {
		if parts[i] == "omitempty" {
			omitempty = true
			break
		}
	}

	return fieldName, omitempty
}

// ApplyValidationTags applies validation constraints from binding and validate tags to a schema
// Supports tags from both gin's binding validator and go-playground/validator
func (tp *TagParser) ApplyValidationTags(schema *openapi3.Schema, bindingTag, validateTag string) {
	// Parse binding tag (used for request structs)
	if bindingTag != "" {
		tp.parseValidationTag(schema, bindingTag)
	}

	// Parse validate tag (used for response structs)
	if validateTag != "" {
		tp.parseValidationTag(schema, validateTag)
	}
}

// parseValidationTag parses a validation tag and applies constraints to the schema
// Handles tags like: "required,max=50,email" or "required" or "len=0|email"
func (tp *TagParser) parseValidationTag(schema *openapi3.Schema, tag string) {
	// Split by comma for multiple validators
	parts := strings.Split(tag, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Check for key=value format
		if strings.Contains(part, "=") {
			tp.parseKeyValueValidator(schema, part)
		} else if strings.Contains(part, "|") {
			// Handle oneOf format like "len=0|email"
			tp.parseOneOfValidator(schema, part)
		} else {
			// Simple validator without value
			tp.parseSimpleValidator(schema, part)
		}
	}
}

// parseSimpleValidator handles validators without values (e.g., "required", "email")
func (tp *TagParser) parseSimpleValidator(schema *openapi3.Schema, validator string) {
	switch validator {
	case "required":
		// Required is handled at the parent level (in required array)
		// We don't set anything on the schema itself

	case "email":
		schema.Format = "email"

	case "url":
		schema.Format = "uri"

	case "uuid":
		schema.Format = "uuid"

	case "numeric":
		if schema.Type.Is("string") {
			schema.Pattern = "^[0-9]+$"
		}

	case "alpha":
		schema.Pattern = "^[a-zA-Z]+$"

	case "alphanum":
		schema.Pattern = "^[a-zA-Z0-9]+$"

	case "base64":
		schema.Format = "byte" // OpenAPI format for base64
	}
}

// parseKeyValueValidator handles validators with values (e.g., "max=50", "min=1")
func (tp *TagParser) parseKeyValueValidator(schema *openapi3.Schema, validator string) {
	parts := strings.SplitN(validator, "=", 2)
	if len(parts) != 2 {
		return
	}

	key := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])

	switch key {
	case "max":
		if schema.Type.Is("string") {
			if maxLen, err := strconv.ParseUint(value, 10, 64); err == nil {
				schema.MaxLength = &maxLen
			}
		} else if schema.Type.Is("integer") || schema.Type.Is("number") {
			if max, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Max = &max
			}
		}

	case "min":
		if schema.Type.Is("string") {
			if minLen, err := strconv.ParseUint(value, 10, 64); err == nil {
				schema.MinLength = minLen
			}
		} else if schema.Type.Is("integer") || schema.Type.Is("number") {
			if min, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Min = &min
			}
		}

	case "len":
		// Exact length for strings
		if schema.Type.Is("string") {
			if length, err := strconv.ParseUint(value, 10, 64); err == nil {
				schema.MinLength = length
				schema.MaxLength = &length
			}
		}

	case "gt":
		// Greater than
		if schema.Type.Is("integer") || schema.Type.Is("number") {
			if gt, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Min = &gt
				schema.ExclusiveMin = true
			}
		}

	case "gte":
		// Greater than or equal
		if schema.Type.Is("integer") || schema.Type.Is("number") {
			if gte, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Min = &gte
			}
		}

	case "lt":
		// Less than
		if schema.Type.Is("integer") || schema.Type.Is("number") {
			if lt, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Max = &lt
				schema.ExclusiveMax = true
			}
		}

	case "lte":
		// Less than or equal
		if schema.Type.Is("integer") || schema.Type.Is("number") {
			if lte, err := strconv.ParseFloat(value, 64); err == nil {
				schema.Max = &lte
			}
		}

	case "oneof":
		// Enum values separated by spaces
		enumValues := strings.Split(value, " ")
		schema.Enum = make([]interface{}, len(enumValues))
		for i, v := range enumValues {
			schema.Enum[i] = v
		}
	}
}

// parseOneOfValidator handles validators with | separator (e.g., "len=0|email")
// This creates a oneOf schema with multiple options
func (tp *TagParser) parseOneOfValidator(schema *openapi3.Schema, validator string) {
	// For now, we'll just apply the first valid constraint
	// Full oneOf support would require more complex schema generation
	parts := strings.Split(validator, "|")
	if len(parts) > 0 {
		tp.parseValidationTag(schema, parts[0])
	}
}

// ApplyCommonTags applies common struct tags like example, enums, format, description
func (tp *TagParser) ApplyCommonTags(schema *openapi3.Schema, field reflect.StructField) {
	// Example tag
	if example := field.Tag.Get("example"); example != "" {
		schema.Example = example
	}

	// Enums tag (comma-separated values)
	if enums := field.Tag.Get("enums"); enums != "" {
		enumValues := strings.Split(enums, ",")
		schema.Enum = make([]interface{}, len(enumValues))
		for i, v := range enumValues {
			schema.Enum[i] = strings.TrimSpace(v)
		}
	}

	// Format tag (overrides auto-detected format)
	if format := field.Tag.Get("format"); format != "" {
		schema.Format = format
	}

	// Description tag
	if description := field.Tag.Get("description"); description != "" {
		schema.Description = description
	}
}

// IsRequired determines if a field is required based on validation tags and omitempty
func (tp *TagParser) IsRequired(bindingTag, validateTag string, omitempty bool) bool {
	// If omitempty is set, field is not required
	if omitempty {
		return false
	}

	// Check if "required" is in either tag
	if strings.Contains(bindingTag, "required") || strings.Contains(validateTag, "required") {
		return true
	}

	return false
}
