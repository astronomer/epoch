package epoch

import (
	"reflect"

	"github.com/bytedance/sonic/ast"
)

// versionChangeBuilder implements the fluent builder API for creating version changes
type versionChangeBuilder struct {
	description    string
	fromVersion    *Version
	toVersion      *Version
	schemaOps      map[reflect.Type]*schemaBuilder
	customRequest  func(*RequestInfo) error
	customResponse func(*ResponseInfo) error
}

// NewVersionChangeBuilder creates a new version change builder (new fluent API)
func NewVersionChangeBuilder(fromVersion, toVersion *Version) *versionChangeBuilder {
	return &versionChangeBuilder{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		schemaOps:   make(map[reflect.Type]*schemaBuilder),
	}
}

// Description sets the human-readable description of the change
func (b *versionChangeBuilder) Description(desc string) *versionChangeBuilder {
	b.description = desc
	return b
}

// Schema starts building operations for a specific schema
func (b *versionChangeBuilder) Schema(schema interface{}) *schemaBuilder {
	schemaType := reflect.TypeOf(schema)

	// Check if we already have a builder for this schema
	if sb, exists := b.schemaOps[schemaType]; exists {
		return sb
	}

	// Create new schema builder
	sb := &schemaBuilder{
		parent:     b,
		schemaType: schemaType,
		operations: make([]Operation, 0),
	}

	b.schemaOps[schemaType] = sb
	return sb
}

// CustomRequest adds a global custom request transformer
func (b *versionChangeBuilder) CustomRequest(fn func(*RequestInfo) error) *versionChangeBuilder {
	b.customRequest = fn
	return b
}

// CustomResponse adds a global custom response transformer
func (b *versionChangeBuilder) CustomResponse(fn func(*ResponseInfo) error) *versionChangeBuilder {
	b.customResponse = fn
	return b
}

// Build compiles all operations into a VersionChange
func (b *versionChangeBuilder) Build() *VersionChange {
	if b.description == "" {
		b.description = "Migration from " + b.fromVersion.String() + " to " + b.toVersion.String()
	}

	var instructions []interface{}

	// Compile schema operations into instructions
	for schemaType, sb := range b.schemaOps {
		if len(sb.operations) == 0 {
			continue
		}

		// Get field mappings for error transformation
		fieldMappings := sb.operations.GetFieldMappings()

		// Create request instruction
		requestInst := &AlterRequestInstruction{
			Schemas: []interface{}{reflect.New(schemaType).Elem().Interface()},
			Transformer: func(req *RequestInfo) error {
				if req.Body == nil {
					return nil
				}

				// Apply operations to request body
				if err := sb.operations.ApplyToRequest(req.Body); err != nil {
					return err
				}

				return nil
			},
		}
		instructions = append(instructions, requestInst)

		// Create response instruction
		responseInst := &AlterResponseInstruction{
			Schemas:           []interface{}{reflect.New(schemaType).Elem().Interface()},
			MigrateHTTPErrors: true,
			Transformer: func(resp *ResponseInfo) error {
				// For non-error responses, apply operations
				if resp.StatusCode < 400 {
					if resp.Body != nil {
						if err := sb.operations.ApplyToResponse(resp.Body); err != nil {
							return err
						}
					}

					// Handle arrays
					return resp.TransformArrayField("", func(node *ast.Node) error {
						return sb.operations.ApplyToResponse(node)
					})
				}

				// For error responses, transform field names
				if len(fieldMappings) > 0 {
					return transformErrorFieldNames(resp, fieldMappings)
				}

				return nil
			},
		}
		instructions = append(instructions, responseInst)
	}

	// Add custom transformers if provided
	if b.customRequest != nil {
		instructions = append(instructions, &AlterRequestInstruction{
			Schemas:     []interface{}{}, // Global
			Transformer: b.customRequest,
		})
	}

	if b.customResponse != nil {
		instructions = append(instructions, &AlterResponseInstruction{
			Schemas:           []interface{}{}, // Global
			MigrateHTTPErrors: true,
			Transformer:       b.customResponse,
		})
	}

	// Use the old NewVersionChange function to create the actual VersionChange
	return newVersionChangeFromBuilder(b.description, b.fromVersion, b.toVersion, instructions...)
}

// schemaBuilder builds operations for a specific schema
type schemaBuilder struct {
	parent         *versionChangeBuilder
	schemaType     reflect.Type
	operations     OperationList
	customRequest  func(*RequestInfo) error
	customResponse func(*ResponseInfo) error
}

// AddField adds a field with a default value
func (sb *schemaBuilder) AddField(name string, defaultValue interface{}) *schemaBuilder {
	sb.operations = append(sb.operations, &FieldAddOp{
		Name:    name,
		Default: defaultValue,
	})
	return sb
}

// RemoveField removes a field
func (sb *schemaBuilder) RemoveField(name string) *schemaBuilder {
	sb.operations = append(sb.operations, &FieldRemoveOp{
		Name: name,
	})
	return sb
}

// RenameField renames a field
func (sb *schemaBuilder) RenameField(from, to string) *schemaBuilder {
	sb.operations = append(sb.operations, &FieldRenameOp{
		From: from,
		To:   to,
	})
	return sb
}

// MapEnumValues maps enum values
func (sb *schemaBuilder) MapEnumValues(field string, mapping map[string]string) *schemaBuilder {
	sb.operations = append(sb.operations, &EnumValueMapOp{
		Field:   field,
		Mapping: mapping,
	})
	return sb
}

// OnRequest adds a custom request transformer for this schema only
func (sb *schemaBuilder) OnRequest(fn func(*RequestInfo) error) *schemaBuilder {
	sb.customRequest = fn
	return sb
}

// OnResponse adds a custom response transformer for this schema only
func (sb *schemaBuilder) OnResponse(fn func(*ResponseInfo) error) *schemaBuilder {
	sb.customResponse = fn
	return sb
}

// Done returns to the parent builder (allows chaining multiple schemas)
func (sb *schemaBuilder) Done() *versionChangeBuilder {
	return sb.parent
}

// Build is a convenience method that calls the parent's Build()
func (sb *schemaBuilder) Build() *VersionChange {
	return sb.parent.Build()
}

// Schema returns to the parent and starts a new schema builder
func (sb *schemaBuilder) Schema(schema interface{}) *schemaBuilder {
	return sb.parent.Schema(schema)
}

// transformErrorFieldNames transforms field names in error messages
func transformErrorFieldNames(resp *ResponseInfo, fieldMapping map[string]string) error {
	if resp.StatusCode < 400 || resp.Body == nil {
		return nil // Not an error response
	}

	errorNode := resp.Body.Get("error")
	if errorNode == nil || !errorNode.Exists() {
		return nil // No error field found
	}

	// Handle simple string errors
	if errorNode.TypeSafe() == ast.V_STRING {
		errorStr, _ := errorNode.String()
		transformedError := replaceFieldNamesInString(errorStr, fieldMapping)
		resp.SetField("error", transformedError)
		return nil
	}

	// Handle structured errors with message
	if errorNode.TypeSafe() == ast.V_OBJECT {
		messageNode := errorNode.Get("message")
		if messageNode != nil && messageNode.Exists() {
			messageStr, _ := messageNode.String()
			transformedMessage := replaceFieldNamesInString(messageStr, fieldMapping)

			// Reconstruct error object
			errorObj := map[string]interface{}{
				"message": transformedMessage,
			}

			// Preserve code if it exists
			codeNode := errorNode.Get("code")
			if codeNode != nil && codeNode.Exists() {
				code, _ := codeNode.String()
				errorObj["code"] = code
			}

			resp.SetField("error", errorObj)
		}
	}

	return nil
}

// replaceFieldNamesInString replaces field names in error messages
func replaceFieldNamesInString(errorMsg string, fieldMapping map[string]string) string {
	result := errorMsg

	for newField, oldField := range fieldMapping {
		// Replace various formats
		patterns := []struct {
			old string
			new string
		}{
			{newField, oldField},                                                                         // better_new_name -> new_name
			{toPascalCase(newField), toPascalCase(oldField)},                                             // BetterNewName -> NewName
			{"'" + newField + "'", "'" + oldField + "'"},                                                 // 'better_new_name' -> 'new_name'
			{"\"" + newField + "\"", "\"" + oldField + "\""},                                             // "better_new_name" -> "new_name"
			{"Key: 'User." + toPascalCase(newField) + "'", "Key: 'User." + toPascalCase(oldField) + "'"}, // Gin validation
		}

		for _, p := range patterns {
			result = stringReplaceAll(result, p.old, p.new)
		}
	}

	return result
}

// toPascalCase converts snake_case to PascalCase
func toPascalCase(s string) string {
	result := ""
	capitalize := true

	for _, ch := range s {
		if ch == '_' {
			capitalize = true
			continue
		}

		if capitalize {
			result += string(ch - 32) // Convert to uppercase
			capitalize = false
		} else {
			result += string(ch)
		}
	}

	return result
}

// stringReplaceAll replaces all occurrences of old with new in s
func stringReplaceAll(s, old, new string) string {
	result := ""
	for {
		idx := stringIndex(s, old)
		if idx == -1 {
			result += s
			break
		}
		result += s[:idx] + new
		s = s[idx+len(old):]
	}
	return result
}

// stringIndex returns the index of the first occurrence of substr in s, or -1
func stringIndex(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(substr) > len(s) {
		return -1
	}

	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}
