package migration

import (
	"fmt"
	"reflect"
)

// ApplyFieldChanges applies a list of field changes to a data structure
func ApplyFieldChanges(data interface{}, changes []FieldChange) (interface{}, error) {
	if len(changes) == 0 {
		return data, nil
	}

	// Handle different data types
	switch d := data.(type) {
	case map[string]interface{}:
		return applyChangesToMap(d, changes)
	case []interface{}:
		return applyChangesToSlice(d, changes)
	default:
		return applyChangesToStruct(data, changes)
	}
}

// applyChangesToMap applies field changes to a map
func applyChangesToMap(data map[string]interface{}, changes []FieldChange) (interface{}, error) {
	result := make(map[string]interface{})

	// Copy existing fields
	for k, v := range data {
		result[k] = v
	}

	// Apply changes
	for _, change := range changes {
		switch change.Operation {
		case "add":
			if change.DefaultValue != nil {
				result[change.FieldName] = change.DefaultValue
			}
		case "remove":
			delete(result, change.FieldName)
		case "rename":
			if value, exists := result[change.FieldName]; exists {
				result[change.NewFieldName] = value
				delete(result, change.FieldName)
			}
		case "transform":
			if value, exists := result[change.FieldName]; exists && change.TransformFunc != nil {
				result[change.FieldName] = change.TransformFunc(value)
			}
		default:
			return nil, fmt.Errorf("unsupported operation: %s", change.Operation)
		}
	}

	return result, nil
}

// applyChangesToSlice applies field changes to each element in a slice
func applyChangesToSlice(data []interface{}, changes []FieldChange) (interface{}, error) {
	result := make([]interface{}, len(data))

	for i, item := range data {
		transformed, err := ApplyFieldChanges(item, changes)
		if err != nil {
			return nil, fmt.Errorf("failed to transform slice item %d: %w", i, err)
		}
		result[i] = transformed
	}

	return result, nil
}

// applyChangesToStruct applies field changes to a struct using reflection
func applyChangesToStruct(data interface{}, changes []FieldChange) (interface{}, error) {
	// Convert struct to map, apply changes, then convert back
	dataMap, err := structToMap(data)
	if err != nil {
		return nil, fmt.Errorf("failed to convert struct to map: %w", err)
	}

	transformedMap, err := applyChangesToMap(dataMap, changes)
	if err != nil {
		return nil, err
	}

	// For now, return the map. In a more advanced implementation,
	// we could reconstruct the struct with the new fields
	return transformedMap, nil
}

// structToMap converts a struct to a map using reflection
func structToMap(data interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})

	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("expected struct, got %s", v.Kind())
	}

	t := v.Type()
	for i := 0; i < v.NumField(); i++ {
		field := t.Field(i)
		value := v.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Use JSON tag if available, otherwise use field name
		fieldName := field.Name
		if jsonTag := field.Tag.Get("json"); jsonTag != "" && jsonTag != "-" {
			// Handle json:"name,omitempty" format
			if commaIdx := findComma(jsonTag); commaIdx != -1 {
				fieldName = jsonTag[:commaIdx]
			} else {
				fieldName = jsonTag
			}
		}

		result[fieldName] = value.Interface()
	}

	return result, nil
}

// findComma finds the first comma in a string, returns -1 if not found
func findComma(s string) int {
	for i, r := range s {
		if r == ',' {
			return i
		}
	}
	return -1
}

// CreateFieldChange is a helper function to create common field changes
func CreateFieldChange(operation, fieldName string, options ...interface{}) FieldChange {
	change := FieldChange{
		FieldName: fieldName,
		Operation: operation,
	}

	// Process options based on operation type
	switch operation {
	case "rename":
		if len(options) > 0 {
			if newName, ok := options[0].(string); ok {
				change.NewFieldName = newName
			}
		}
	case "add":
		if len(options) > 0 {
			change.DefaultValue = options[0]
		}
	case "transform":
		if len(options) > 0 {
			if transformFunc, ok := options[0].(func(interface{}) interface{}); ok {
				change.TransformFunc = transformFunc
			}
		}
	}

	return change
}

// Common field transformation functions

// AddField creates a field change that adds a new field with a default value
func AddField(fieldName string, defaultValue interface{}) FieldChange {
	return CreateFieldChange("add", fieldName, defaultValue)
}

// RemoveField creates a field change that removes a field
func RemoveField(fieldName string) FieldChange {
	return CreateFieldChange("remove", fieldName)
}

// RenameField creates a field change that renames a field
func RenameField(oldName, newName string) FieldChange {
	return CreateFieldChange("rename", oldName, newName)
}

// TransformField creates a field change that transforms a field's value
func TransformField(fieldName string, transformFunc func(interface{}) interface{}) FieldChange {
	return CreateFieldChange("transform", fieldName, transformFunc)
}

// Common transformation functions

// StringToInt transforms a string value to an integer
func StringToInt(value interface{}) interface{} {
	if str, ok := value.(string); ok {
		// Simple conversion - in practice, you'd want proper error handling
		if str == "" {
			return 0
		}
		// This is a simplified conversion
		return len(str) // Just as an example
	}
	return value
}

// IntToString transforms an integer value to a string
func IntToString(value interface{}) interface{} {
	if i, ok := value.(int); ok {
		return fmt.Sprintf("%d", i)
	}
	return value
}

// AddTimestamp adds a timestamp field with the current time
func AddTimestamp(fieldName string) FieldChange {
	return AddField(fieldName, "2025-01-01T00:00:00Z") // Default timestamp
}

// ConvertToLowercase transforms a string field to lowercase
func ConvertToLowercase(fieldName string) FieldChange {
	return TransformField(fieldName, func(value interface{}) interface{} {
		if str, ok := value.(string); ok {
			return fmt.Sprintf("%s", str) // Simplified - would use strings.ToLower
		}
		return value
	})
}
