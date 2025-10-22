package epoch

import (
	"strings"

	"github.com/bytedance/sonic/ast"
)

// versionChangeBuilder implements the fluent builder API for creating version changes
type versionChangeBuilder struct {
	description    string
	fromVersion    *Version
	toVersion      *Version
	pathOps        map[string]*pathBuilder
	customRequest  func(*RequestInfo) error
	customResponse func(*ResponseInfo) error
}

// NewVersionChangeBuilder creates a new version change builder (new fluent API)
func NewVersionChangeBuilder(fromVersion, toVersion *Version) *versionChangeBuilder {
	return &versionChangeBuilder{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		pathOps:     make(map[string]*pathBuilder),
	}
}

// Description sets the human-readable description of the change
func (b *versionChangeBuilder) Description(desc string) *versionChangeBuilder {
	b.description = desc
	return b
}

// ForPath starts building operations for specific paths (runtime routing)
// Multiple paths can be specified to apply the same operations to all of them.
func (b *versionChangeBuilder) ForPath(paths ...string) *pathBuilder {
	pb := &pathBuilder{
		parent:     b,
		paths:      paths,
		operations: make([]Operation, 0),
	}

	// Store by first path for retrieval
	if len(paths) > 0 {
		b.pathOps[paths[0]] = pb
	}

	return pb
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

	// Validate: require at least one path or custom transformer
	if len(b.pathOps) == 0 && b.customRequest == nil && b.customResponse == nil {
		panic("epoch: VersionChange must specify at least one path using ForPath() or custom transformers")
	}

	var instructions []interface{}

	// Compile path-based operations into instructions (RUNTIME ROUTING)
	for _, pb := range b.pathOps {
		if len(pb.operations) == 0 {
			continue
		}

		// Get field mappings for error transformation
		fieldMappings := pb.operations.GetFieldMappings()

		// Create request instruction for each path
		for _, path := range pb.paths {
			requestInst := &AlterRequestInstruction{
				Path:    path,
				Methods: []string{}, // Empty means all methods
				Transformer: func(req *RequestInfo) error {
					if req.Body == nil {
						return nil
					}

					// Apply operations to request body
					if err := pb.operations.ApplyToRequest(req.Body); err != nil {
						return err
					}

					return nil
				},
			}
			instructions = append(instructions, requestInst)

			// Create response instruction
			responseInst := &AlterResponseInstruction{
				Path:              path,
				Methods:           []string{}, // Empty means all methods
				MigrateHTTPErrors: true,
				Transformer: func(resp *ResponseInfo) error {
					// For non-error responses, apply operations
					if resp.StatusCode < 400 {
						if resp.Body != nil {
							if err := pb.operations.ApplyToResponse(resp.Body); err != nil {
								return err
							}
						}

						// Handle arrays
						return resp.TransformArrayField("", func(node *ast.Node) error {
							return pb.operations.ApplyToResponse(node)
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
	return NewVersionChange(b.description, b.fromVersion, b.toVersion, instructions...)
}

// pathBuilder builds operations for specific paths (runtime routing)
type pathBuilder struct {
	parent     *versionChangeBuilder
	paths      []string
	operations OperationList
}

// pathBuilder methods

// AddField adds a field with a default value
func (pb *pathBuilder) AddField(name string, defaultValue interface{}) *pathBuilder {
	pb.operations = append(pb.operations, &FieldAddOp{
		Name:    name,
		Default: defaultValue,
	})
	return pb
}

// RemoveField removes a field
func (pb *pathBuilder) RemoveField(name string) *pathBuilder {
	pb.operations = append(pb.operations, &FieldRemoveOp{
		Name: name,
	})
	return pb
}

// RenameField renames a field
func (pb *pathBuilder) RenameField(from, to string) *pathBuilder {
	pb.operations = append(pb.operations, &FieldRenameOp{
		From: from,
		To:   to,
	})
	return pb
}

// MapEnumValues maps enum values
func (pb *pathBuilder) MapEnumValues(field string, mapping map[string]string) *pathBuilder {
	pb.operations = append(pb.operations, &EnumValueMapOp{
		Field:   field,
		Mapping: mapping,
	})
	return pb
}

// ForPath returns to the parent and starts a new path builder
func (pb *pathBuilder) ForPath(paths ...string) *pathBuilder {
	return pb.parent.ForPath(paths...)
}

// Build is a convenience method that calls the parent's Build()
func (pb *pathBuilder) Build() *VersionChange {
	return pb.parent.Build()
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
			result = strings.ReplaceAll(result, p.old, p.new)
		}
	}

	return result
}

// toPascalCase converts snake_case to PascalCase
func toPascalCase(s string) string {
	if s == "" {
		return ""
	}

	// Handle common API naming conventions
	s = strings.Replace(s, "ID", "Id", -1)
	s = strings.Replace(s, "URL", "Url", -1)
	s = strings.Replace(s, "HTTP", "Http", -1)
	s = strings.Replace(s, "API", "Api", -1)

	// Split by underscores for snake_case
	parts := strings.Split(s, "_")

	var result strings.Builder
	result.Grow(len(s))

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Capitalize first character properly using strings.ToUpper
		runes := []rune(part)
		if len(runes) > 0 {
			result.WriteString(strings.ToUpper(string(runes[0])))
			if len(runes) > 1 {
				result.WriteString(string(runes[1:]))
			}
		}
	}

	return result.String()
}
