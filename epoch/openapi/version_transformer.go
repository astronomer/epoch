package openapi

import (
	"fmt"
	"reflect"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
)

// VersionTransformer applies version migrations to OpenAPI schemas
type VersionTransformer struct {
	versionBundle *epoch.VersionBundle
	typeParser    *TypeParser
}

// NewVersionTransformer creates a new version transformer
func NewVersionTransformer(versionBundle *epoch.VersionBundle) *VersionTransformer {
	return &VersionTransformer{
		versionBundle: versionBundle,
		typeParser:    NewTypeParser(),
	}
}

// TransformSchemaForVersion applies version transformations to generate a schema for a specific version
// direction specifies whether this is for a request (Client→HEAD) or response (HEAD→Client)
func (vt *VersionTransformer) TransformSchemaForVersion(
	baseSchema *openapi3.Schema,
	targetType reflect.Type,
	targetVersion *epoch.Version,
	direction SchemaDirection,
) (*openapi3.Schema, error) {
	// Clone the schema to avoid modifying the original
	schema := CloneSchema(baseSchema)

	if targetVersion.IsHead {
		// No transformation needed for HEAD version
		return schema, nil
	}

	// Get all version changes that apply to this type
	changes := vt.getVersionChanges(targetType, targetVersion, direction)

	// Apply all changes in order
	for _, change := range changes {
		if err := vt.applyChange(schema, change, direction); err != nil {
			return nil, fmt.Errorf("failed to apply change from %s to %s: %w",
				change.fromVersion.String(), change.toVersion.String(), err)
		}
	}

	return schema, nil
}

// versionChange holds information about a single version change operation
type versionChange struct {
	fromVersion *epoch.Version
	toVersion   *epoch.Version
	operation   interface{}  // RequestOperation or ResponseOperation
	targetType  reflect.Type // The Go type this change applies to
	inverted    bool         // For request schema generation: signals operations should be inverted
}

// getVersionChanges returns all version changes that need to be applied
func (vt *VersionTransformer) getVersionChanges(
	targetType reflect.Type,
	targetVersion *epoch.Version,
	direction SchemaDirection,
) []versionChange {
	var changes []versionChange

	// Get all versions
	versions := vt.versionBundle.GetVersions()

	if direction == SchemaDirectionRequest {
		// Request: walk BACKWARD from HEAD to target version and INVERT operations
		// For schema generation, we need to transform HEAD schemas backward to older versions
		// But RequestToNextVersion operations are defined forward (Client→HEAD)
		// So we walk backward and mark operations for inversion

		// Find ending position (target version)
		endIdx := -1
		for i, v := range versions {
			if v.Equal(targetVersion) {
				endIdx = i
				break
			}
		}

		if endIdx == -1 {
			return changes // Version not found
		}

		// Walk backward from HEAD to target version
		for i := len(versions) - 1; i >= endIdx; i-- {
			currentVer := versions[i]

			// Determine previous version for the transformation
			var prevVer *epoch.Version
			if i > 0 {
				prevVer = versions[i-1]
			} else {
				// At oldest version
				prevVer = currentVer
			}

			// Process changes on currentVer (which describe transitions FROM currentVer)
			// These are applied in reverse for request schema transformation
			for _, vc := range currentVer.Changes {
				if epochVC, ok := vc.(*epoch.VersionChange); ok {
					// Check if this change has request operations for our type
					if ops, exists := epochVC.GetRequestOperationsByType(targetType); exists && len(ops) > 0 {
						changes = append(changes, versionChange{
							fromVersion: currentVer,
							toVersion:   prevVer,
							operation:   epochVC,
							targetType:  targetType,
							inverted:    true, // INVERT operations for schema generation
						})
					}
				}
			}
		}

	} else {
		// Response: walk BACKWARD from HEAD to target version
		// Find ending position
		endIdx := -1
		for i, v := range versions {
			if v.Equal(targetVersion) {
				endIdx = i
				break
			}
		}

		if endIdx == -1 {
			return changes // Version not found
		}

		// Walk backward collecting changes from all versions >= target
		// Changes are attached to the "FROM" version (e.g., v1→v2 change is on v1)
		// We process them in reverse to transform schemas backward from HEAD to target
		for i := len(versions) - 1; i >= endIdx; i-- {
			currentVer := versions[i]

			// Determine previous version for the transformation
			// This handles the case where we're at the oldest version (endIdx=0)
			var prevVer *epoch.Version
			if i > 0 {
				prevVer = versions[i-1]
			} else {
				// At oldest version: if it has changes (against validation), we still process them
				// The prevVer doesn't matter since changes describe transition away from this version
				prevVer = currentVer
			}

			// Process changes on currentVer (which describe transitions FROM currentVer)
			// These are applied in reverse for response transformation
			for _, vc := range currentVer.Changes {
				if epochVC, ok := vc.(*epoch.VersionChange); ok {
					// Check if this change applies to our type
					if vt.changeAppliesToType(epochVC, targetType, SchemaDirectionResponse) {
						changes = append(changes, versionChange{
							fromVersion: currentVer,
							toVersion:   prevVer,
							operation:   epochVC,
							targetType:  targetType,
						})
					}
				}
			}
		}
	}

	return changes
}

// changeAppliesToType checks if a version change applies to the given type
func (vt *VersionTransformer) changeAppliesToType(
	vc *epoch.VersionChange,
	targetType reflect.Type,
	direction SchemaDirection,
) bool {
	if targetType == nil {
		return false
	}

	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}

	if direction == SchemaDirectionRequest {
		_, exists := vc.GetRequestOperationsByType(targetType)
		return exists
	} else {
		_, exists := vc.GetResponseOperationsByType(targetType)
		return exists
	}
}

// applyChange applies a single version change to a schema
func (vt *VersionTransformer) applyChange(
	schema *openapi3.Schema,
	change versionChange,
	direction SchemaDirection,
) error {
	vc, ok := change.operation.(*epoch.VersionChange)
	if !ok {
		return fmt.Errorf("invalid operation type")
	}

	// Unwrap pointer type if needed
	targetType := change.targetType
	if targetType.Kind() == reflect.Ptr {
		targetType = targetType.Elem()
	}

	// Get operations for this type based on direction
	if direction == SchemaDirectionRequest {
		ops, exists := vc.GetRequestOperationsByType(targetType)
		if !exists || len(ops) == 0 {
			return nil // No operations for this type
		}

		// Apply each operation to the schema (inverted if flagged)
		for _, op := range ops {
			actualOp := op

			if change.inverted {
				// Invert the operation for schema generation
				actualOp = op.Inverse()
				if actualOp == nil {
					// Operation not invertible (e.g., Custom), skip it
					// This is safe - custom operations handle runtime logic, not schema structure
					continue
				}
			}

			if err := vt.applyOperationToSchema(schema, actualOp, direction); err != nil {
				return fmt.Errorf("failed to apply request operation: %w", err)
			}
		}
	} else {
		ops, exists := vc.GetResponseOperationsByType(targetType)
		if !exists || len(ops) == 0 {
			return nil // No operations for this type
		}

		// Apply each operation to the schema
		for _, op := range ops {
			if err := vt.applyOperationToSchema(schema, op, direction); err != nil {
				return fmt.Errorf("failed to apply response operation: %w", err)
			}
		}
	}

	return nil
}

// applyOperationToSchema applies a single field operation to a schema
func (vt *VersionTransformer) applyOperationToSchema(
	schema *openapi3.Schema,
	op interface{},
	direction SchemaDirection,
) error {
	switch operation := op.(type) {
	case *epoch.ResponseAddField:
		// Add a field to the response schema
		fieldSchema := vt.createSchemaForValue(operation.Default)
		vt.AddFieldToSchema(schema, operation.Name, fieldSchema, false)

	case *epoch.ResponseRemoveField:
		// Remove a field from the response schema
		vt.RemoveFieldFromSchema(schema, operation.Name)

	case *epoch.ResponseRenameField:
		// Rename a field in the response schema
		vt.RenameFieldInSchema(schema, operation.NewerVersionName, operation.OlderVersionName)

	case *epoch.RequestAddField:
		// Add a field to the request schema
		fieldSchema := vt.createSchemaForValue(operation.Default)
		vt.AddFieldToSchema(schema, operation.Name, fieldSchema, false)

	case *epoch.RequestAddFieldWithDefault:
		// Add a field with default to the request schema
		fieldSchema := vt.createSchemaForValue(operation.Default)
		vt.AddFieldToSchema(schema, operation.Name, fieldSchema, false)

	case *epoch.RequestRemoveField:
		// Remove a field from the request schema
		vt.RemoveFieldFromSchema(schema, operation.Name)

	case *epoch.RequestRenameField:
		// Rename a field in the request schema
		vt.RenameFieldInSchema(schema, operation.OlderVersionName, operation.NewerVersionName)

	case *epoch.ResponseRemoveFieldIfDefault:
		// For schema generation, treat this as a regular remove
		// (The conditional logic only applies at runtime)
		vt.RemoveFieldFromSchema(schema, operation.Name)

	case *epoch.RequestCustom, *epoch.ResponseCustom:
		// Custom operations are skipped during schema generation
		// They contain arbitrary logic that can't be represented in OpenAPI
		return nil

	default:
		// Unknown operation type - skip it
		return nil
	}

	return nil
}

// createSchemaForValue generates an OpenAPI schema for a given default value
// This is used when adding fields to schemas during version transformations
func (vt *VersionTransformer) createSchemaForValue(value interface{}) *openapi3.SchemaRef {
	if value == nil {
		// Return a generic object schema for nil values
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type: &openapi3.Types{"object"},
		})
	}

	typ := reflect.TypeOf(value)

	// Handle primitive types directly
	switch typ.Kind() {
	case reflect.String:
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type: &openapi3.Types{"string"},
		})

	case reflect.Bool:
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type: &openapi3.Types{"boolean"},
		})

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32:
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type:   &openapi3.Types{"integer"},
			Format: "int32",
		})

	case reflect.Int64:
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type:   &openapi3.Types{"integer"},
			Format: "int64",
		})

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		schema := &openapi3.Schema{
			Type:   &openapi3.Types{"integer"},
			Format: "int64",
		}
		min := 0.0
		schema.Min = &min
		return openapi3.NewSchemaRef("", schema)

	case reflect.Float32:
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type:   &openapi3.Types{"number"},
			Format: "float",
		})

	case reflect.Float64:
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type:   &openapi3.Types{"number"},
			Format: "double",
		})

	case reflect.Struct, reflect.Ptr, reflect.Slice, reflect.Array, reflect.Map:
		// For complex types, use the TypeParser to generate full schemas
		schemaRef, err := vt.typeParser.ParseType(typ)
		if err != nil {
			// Fallback to a generic object schema on error
			return openapi3.NewSchemaRef("", &openapi3.Schema{
				Type: &openapi3.Types{"object"},
			})
		}
		return schemaRef

	default:
		// Fallback for unknown types
		return openapi3.NewSchemaRef("", &openapi3.Schema{
			Type: &openapi3.Types{"object"},
		})
	}
}

// Field operations that can be applied to schemas

// AddFieldToSchema adds a field to a schema
func (vt *VersionTransformer) AddFieldToSchema(schema *openapi3.Schema, fieldName string, fieldSchema *openapi3.SchemaRef, required bool) {
	if schema.Properties == nil {
		schema.Properties = make(map[string]*openapi3.SchemaRef)
	}

	schema.Properties[fieldName] = fieldSchema

	if required {
		schema.Required = append(schema.Required, fieldName)
	}
}

// RemoveFieldFromSchema removes a field from a schema
func (vt *VersionTransformer) RemoveFieldFromSchema(schema *openapi3.Schema, fieldName string) {
	if schema.Properties != nil {
		delete(schema.Properties, fieldName)
	}

	// Remove from required array
	schema.Required = removeFromSlice(schema.Required, fieldName)
}

// RenameFieldInSchema renames a field in a schema
func (vt *VersionTransformer) RenameFieldInSchema(schema *openapi3.Schema, oldName, newName string) {
	if schema.Properties == nil {
		return
	}

	// Get the field schema
	fieldSchema, exists := schema.Properties[oldName]
	if !exists {
		return
	}

	// Add with new name
	schema.Properties[newName] = fieldSchema

	// Remove old name
	delete(schema.Properties, oldName)

	// Update required array
	for i, req := range schema.Required {
		if req == oldName {
			schema.Required[i] = newName
			break
		}
	}
}

// CloneSchema creates a deep copy of an OpenAPI schema
func CloneSchema(original *openapi3.Schema) *openapi3.Schema {
	if original == nil {
		return nil
	}

	clone := &openapi3.Schema{
		Type:        &openapi3.Types{},
		Format:      original.Format,
		Description: original.Description,
		Example:     original.Example,
		Enum:        make([]interface{}, len(original.Enum)),
		Default:     original.Default,
	}

	// Deep copy Type slice
	if original.Type != nil {
		*clone.Type = make(openapi3.Types, len(*original.Type))
		copy(*clone.Type, *original.Type)
	}

	// Copy enums
	copy(clone.Enum, original.Enum)

	// Copy numeric constraints
	if original.Min != nil {
		min := *original.Min
		clone.Min = &min
	}
	if original.Max != nil {
		max := *original.Max
		clone.Max = &max
	}
	clone.ExclusiveMin = original.ExclusiveMin
	clone.ExclusiveMax = original.ExclusiveMax

	// Copy string constraints
	if original.MaxLength != nil {
		maxLen := *original.MaxLength
		clone.MaxLength = &maxLen
	}
	clone.MinLength = original.MinLength
	clone.Pattern = original.Pattern

	// Copy array constraints
	if original.MaxItems != nil {
		maxItems := *original.MaxItems
		clone.MaxItems = &maxItems
	}
	clone.MinItems = original.MinItems

	// Copy properties
	if original.Properties != nil {
		clone.Properties = make(map[string]*openapi3.SchemaRef, len(original.Properties))
		for k, v := range original.Properties {
			// Deep clone each property's SchemaRef
			if v.Value != nil {
				clone.Properties[k] = openapi3.NewSchemaRef("", CloneSchema(v.Value))
			} else if v.Ref != "" {
				clone.Properties[k] = &openapi3.SchemaRef{Ref: v.Ref}
			} else {
				clone.Properties[k] = v
			}
		}
	}

	// Copy required array
	if original.Required != nil {
		clone.Required = make([]string, len(original.Required))
		copy(clone.Required, original.Required)
	}

	// Copy items
	clone.Items = original.Items

	// Copy additionalProperties
	clone.AdditionalProperties = original.AdditionalProperties

	return clone
}

// Helper function to remove an item from a string slice
func removeFromSlice(slice []string, item string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != item {
			result = append(result, s)
		}
	}
	return result
}
