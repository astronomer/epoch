package epoch

import (
	"reflect"
	"strings"

	"github.com/bytedance/sonic/ast"
)

// ============================================================================
// VERSION CHANGE BUILDER - Fluent API for building migrations
// ============================================================================

// versionChangeBuilder implements the flow-based fluent builder API
// Following the actual migration flow:
// - Requests: Client Version → HEAD Version (always forward)
// - Responses: HEAD Version → Client Version (always backward)
type versionChangeBuilder struct {
	description    string
	fromVersion    *Version
	toVersion      *Version
	typeOps        map[reflect.Type]*typeBuilder
	customRequest  func(*RequestInfo) error
	customResponse func(*ResponseInfo) error
}

// NewVersionChangeBuilder creates a new type-based version change builder
func NewVersionChangeBuilder(fromVersion, toVersion *Version) *versionChangeBuilder {
	return &versionChangeBuilder{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		typeOps:     make(map[reflect.Type]*typeBuilder),
	}
}

// Description sets the human-readable description of the change
func (b *versionChangeBuilder) Description(desc string) *versionChangeBuilder {
	b.description = desc
	return b
}

// ForType starts building operations for specific types
// This allows targeting migrations to specific Go struct types (e.g., UserResponse)
// Types are explicitly declared at endpoint registration via WrapHandler().Returns()/.Accepts()
func (b *versionChangeBuilder) ForType(types ...interface{}) *typeBuilder {
	tb := &typeBuilder{
		parent:                       b,
		targetTypes:                  make([]reflect.Type, 0, len(types)),
		requestToNextVersionOps:      make(RequestToNextVersionOperationList, 0),
		responseToPreviousVersionOps: make(ResponseToPreviousVersionOperationList, 0),
	}

	// Convert types to reflect.Type
	for _, t := range types {
		reflectType := reflect.TypeOf(t)
		if reflectType.Kind() == reflect.Ptr {
			reflectType = reflectType.Elem()
		}
		tb.targetTypes = append(tb.targetTypes, reflectType)

		// Store by first type for retrieval
		if len(tb.targetTypes) == 1 {
			b.typeOps[reflectType] = tb
		}
	}

	return tb
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

	// Validate: require at least one type or custom transformer
	if len(b.typeOps) == 0 && b.customRequest == nil && b.customResponse == nil {
		panic("epoch: VersionChange must specify at least one type using ForType() or custom transformers")
	}

	var instructions []interface{}

	// Compile type-based operations into instructions
	for _, tb := range b.typeOps {
		if len(tb.requestToNextVersionOps) == 0 &&
			len(tb.responseToPreviousVersionOps) == 0 {
			continue
		}

		// CRITICAL: Create local copies to avoid closure variable capture bug
		// Without this, all closures would reference the same (last) tb
		tbCopy := tb

		// Get field mappings for error transformation
		fieldMappings := make(map[string]string)

		// Combine field mappings from both operation types
		for k, v := range tbCopy.requestToNextVersionOps.GetFieldMappings() {
			fieldMappings[k] = v
		}
		for k, v := range tbCopy.responseToPreviousVersionOps.GetFieldMappings() {
			fieldMappings[k] = v
		}

		// Create request instruction for each type
		for _, targetType := range tbCopy.targetTypes {
			// CRITICAL: Create local copy for closure
			targetTypeCopy := targetType
			requestOpsCopy := tbCopy.requestToNextVersionOps

			requestInst := &AlterRequestInstruction{
				Schemas: []interface{}{reflect.New(targetTypeCopy).Interface()},
				Transformer: func(req *RequestInfo) error {
					if req.Body == nil {
						return nil
					}

					// Request migration is always FROM client version TO HEAD version
					// Apply "to next version" operations (Client→HEAD)
					return requestOpsCopy.Apply(req.Body)
				},
			}
			instructions = append(instructions, requestInst)

			// Create response instruction
			responseOpsCopy := tbCopy.responseToPreviousVersionOps
			fieldMappingsCopy := make(map[string]string)
			for k, v := range fieldMappings {
				fieldMappingsCopy[k] = v
			}

			responseInst := &AlterResponseInstruction{
				Schemas:           []interface{}{reflect.New(targetTypeCopy).Interface()},
				MigrateHTTPErrors: true,
				Transformer: func(resp *ResponseInfo) error {
					if resp.Body != nil {
						// Handle arrays and objects separately
						if resp.Body.TypeSafe() == ast.V_ARRAY {
							// For arrays, apply operations to each item
							if err := resp.TransformArrayField("", func(node *ast.Node) error {
								// Response migration is always FROM HEAD version TO client version
								// Apply "to previous version" operations (HEAD→Client)
								return responseOpsCopy.Apply(node)
							}); err != nil {
								return err
							}
						} else {
							// For objects, apply operations to the object
							// Response migration is always FROM HEAD version TO client version
							if err := responseOpsCopy.Apply(resp.Body); err != nil {
								return err
							}
							// Note: Nested arrays are now handled by VersionChange.MigrateResponse
							// using type-aware transformNestedArrayItemsForSingleStep at each migration step
						}
					}

					// Additionally transform field names in error messages for validation errors
					if len(fieldMappingsCopy) > 0 {
						return transformErrorFieldNamesInResponse(resp, fieldMappingsCopy)
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

	// Create the VersionChange
	vc := NewVersionChange(b.description, b.fromVersion, b.toVersion, instructions...)

	// Populate operation metadata for OpenAPI schema generation
	// This allows the schema generator to extract field operations (Add/Remove/Rename)
	for _, tb := range b.typeOps {
		for _, targetType := range tb.targetTypes {
			// Store the operation lists for this type
			if len(tb.requestToNextVersionOps) > 0 {
				vc.requestOperationsByType[targetType] = tb.requestToNextVersionOps
			}
			if len(tb.responseToPreviousVersionOps) > 0 {
				vc.responseOperationsByType[targetType] = tb.responseToPreviousVersionOps
			}
		}
	}

	return vc
}

// typeBuilder builds operations for specific types
type typeBuilder struct {
	parent                       *versionChangeBuilder
	targetTypes                  []reflect.Type
	requestToNextVersionOps      RequestToNextVersionOperationList
	responseToPreviousVersionOps ResponseToPreviousVersionOperationList
}

// RequestToNextVersion returns a builder for request operations (Client→HEAD)
// This is the ONLY direction requests flow
func (tb *typeBuilder) RequestToNextVersion() *requestToNextVersionBuilder {
	return &requestToNextVersionBuilder{parent: tb}
}

// ResponseToPreviousVersion returns a builder for response operations (HEAD→Client)
// This is the ONLY direction responses flow
func (tb *typeBuilder) ResponseToPreviousVersion() *responseToPreviousVersionBuilder {
	return &responseToPreviousVersionBuilder{parent: tb}
}

// ForType returns to the parent and starts a new type builder
func (tb *typeBuilder) ForType(types ...interface{}) *typeBuilder {
	return tb.parent.ForType(types...)
}

// Build is a convenience method that calls the parent's Build()
func (tb *typeBuilder) Build() *VersionChange {
	return tb.parent.Build()
}

type requestToNextVersionBuilder struct {
	parent *typeBuilder
}

// AddField adds a field when request migrates from client to HEAD
func (b *requestToNextVersionBuilder) AddField(name string, defaultValue interface{}) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestAddField{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// AddFieldWithDefault adds a field ONLY if missing (Cadwyn-style default handling)
func (b *requestToNextVersionBuilder) AddFieldWithDefault(name string, defaultValue interface{}) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestAddFieldWithDefault{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// RemoveField removes a field when request migrates from client to HEAD
func (b *requestToNextVersionBuilder) RemoveField(name string) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestRemoveField{
			Name: name,
		})
	return b
}

// RenameField renames a field when request migrates from client to HEAD
func (b *requestToNextVersionBuilder) RenameField(olderVersionName, newerVersionName string) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestRenameField{
			OlderVersionName: olderVersionName,
			NewerVersionName: newerVersionName,
		})
	return b
}

// Custom applies a custom transformation function to the request
func (b *requestToNextVersionBuilder) Custom(fn func(*RequestInfo) error) *requestToNextVersionBuilder {
	// Wrap RequestInfo function to work with ast.Node
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestCustom{
			Fn: func(node *ast.Node) error {
				// Create a temporary RequestInfo wrapper
				req := &RequestInfo{Body: node}
				return fn(req)
			},
		})
	return b
}

// Back to response builder
func (b *requestToNextVersionBuilder) ResponseToPreviousVersion() *responseToPreviousVersionBuilder {
	return b.parent.ResponseToPreviousVersion()
}

// Back to type builder
func (b *requestToNextVersionBuilder) ForType(types ...interface{}) *typeBuilder {
	return b.parent.ForType(types...)
}

// Build completes the builder chain
func (b *requestToNextVersionBuilder) Build() *VersionChange {
	return b.parent.Build()
}

type responseToPreviousVersionBuilder struct {
	parent *typeBuilder
}

// AddField adds a field when response migrates from HEAD to client
func (b *responseToPreviousVersionBuilder) AddField(name string, defaultValue interface{}) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseAddField{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// RemoveField removes a field when response migrates from HEAD to client
func (b *responseToPreviousVersionBuilder) RemoveField(name string) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseRemoveField{
			Name: name,
		})
	return b
}

// RemoveFieldIfDefault removes a field ONLY if it equals the default value
func (b *responseToPreviousVersionBuilder) RemoveFieldIfDefault(name string, defaultValue interface{}) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseRemoveFieldIfDefault{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// RenameField renames a field when response migrates from HEAD to client
func (b *responseToPreviousVersionBuilder) RenameField(newerVersionName, olderVersionName string) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseRenameField{
			NewerVersionName: newerVersionName,
			OlderVersionName: olderVersionName,
		})
	return b
}

// Custom applies a custom transformation function to the response
func (b *responseToPreviousVersionBuilder) Custom(fn func(*ResponseInfo) error) *responseToPreviousVersionBuilder {
	// Wrap ResponseInfo function to work with ast.Node
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseCustom{
			Fn: func(node *ast.Node) error {
				// Create a temporary ResponseInfo wrapper
				resp := &ResponseInfo{Body: node}
				return fn(resp)
			},
		})
	return b
}

// Back to request builder
func (b *responseToPreviousVersionBuilder) RequestToNextVersion() *requestToNextVersionBuilder {
	return b.parent.RequestToNextVersion()
}

// Back to type builder
func (b *responseToPreviousVersionBuilder) ForType(types ...interface{}) *typeBuilder {
	return b.parent.ForType(types...)
}

// Build completes the builder chain
func (b *responseToPreviousVersionBuilder) Build() *VersionChange {
	return b.parent.Build()
}

// ============================================================================
// ERROR FIELD NAME TRANSFORMATION HELPERS
// ============================================================================

// transformErrorFieldNamesInResponse transforms field names in error messages
// Works with any error response format by recursively processing all string fields
func transformErrorFieldNamesInResponse(resp *ResponseInfo, fieldMapping map[string]string) error {
	// Only transform validation errors (400 Bad Request)
	if resp.StatusCode != 400 || resp.Body == nil {
		return nil
	}

	// Recursively transform all string fields in the error response
	return transformStringsInNode(resp.Body, fieldMapping)
}

// transformStringsInNode recursively transforms all string fields in an AST node
// This works with any error format: {"error": "..."}, {"message": "..."}, RFC 7807, etc.
func transformStringsInNode(node *ast.Node, fieldMapping map[string]string) error {
	if node == nil || !node.Exists() {
		return nil
	}

	nodeType := node.TypeSafe()

	switch nodeType {
	case ast.V_OBJECT:
		// Get all keys and recursively transform each field
		objMap, err := node.Map()
		if err != nil {
			return err
		}

		for key := range objMap {
			fieldNode := node.Get(key)
			if fieldNode == nil || !fieldNode.Exists() {
				continue
			}

			fieldType := fieldNode.TypeSafe()
			switch fieldType {
			case ast.V_STRING:
				// Transform string value
				strVal, _ := fieldNode.String()
				transformed := replaceFieldNamesInErrorString(strVal, fieldMapping)
				node.SetAny(key, transformed)

			case ast.V_ARRAY:
				// Check if array contains strings that need transformation
				if err := transformStringsInArrayField(node, key, fieldNode, fieldMapping); err != nil {
					return err
				}

			case ast.V_OBJECT:
				// Recursively process nested objects
				transformStringsInNode(fieldNode, fieldMapping)
			}
		}

	case ast.V_ARRAY:
		// For arrays not in an object (root arrays), transform each element
		length, err := node.Len()
		if err != nil {
			return err
		}

		for i := 0; i < length; i++ {
			item := node.Index(i)
			if item != nil && item.Exists() && item.TypeSafe() == ast.V_OBJECT {
				transformStringsInNode(item, fieldMapping)
			}
		}
	}

	return nil
}

// transformStringsInArrayField handles transformation of array fields that may contain strings
func transformStringsInArrayField(parentNode *ast.Node, key string, arrayNode *ast.Node, fieldMapping map[string]string) error {
	length, err := arrayNode.Len()
	if err != nil {
		return err
	}

	// Check if any elements need transformation
	needsTransform := false
	newArray := make([]interface{}, length)

	for i := 0; i < length; i++ {
		item := arrayNode.Index(i)
		if item == nil || !item.Exists() {
			continue
		}

		itemType := item.TypeSafe()
		if itemType == ast.V_STRING {
			strVal, _ := item.String()
			transformed := replaceFieldNamesInErrorString(strVal, fieldMapping)
			newArray[i] = transformed
			if transformed != strVal {
				needsTransform = true
			}
		} else if itemType == ast.V_OBJECT {
			// Recursively transform objects in arrays
			transformStringsInNode(item, fieldMapping)
			// Keep the original object node
			val, _ := item.Interface()
			newArray[i] = val
		} else {
			// Keep other types as-is
			val, _ := item.Interface()
			newArray[i] = val
		}
	}

	// Only update the array if we transformed any strings
	if needsTransform {
		parentNode.SetAny(key, newArray)
	}

	return nil
}

// replaceFieldNamesInErrorString replaces field names in error messages
func replaceFieldNamesInErrorString(errorMsg string, fieldMapping map[string]string) string {
	result := errorMsg

	for newField, oldField := range fieldMapping {
		// Replace various formats
		patterns := []struct {
			old string
			new string
		}{
			{newField, oldField},
			{toPascalCaseString(newField), toPascalCaseString(oldField)},
			{"'" + newField + "'", "'" + oldField + "'"},
			{"\"" + newField + "\"", "\"" + oldField + "\""},
			{"Key: 'User." + toPascalCaseString(newField) + "'", "Key: 'User." + toPascalCaseString(oldField) + "'"},
		}

		for _, p := range patterns {
			result = strings.ReplaceAll(result, p.old, p.new)
		}
	}

	return result
}

// toPascalCaseString converts snake_case to PascalCase
func toPascalCaseString(s string) string {
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

		// Capitalize first character
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
