package migration

import (
	"testing"
)

func TestApplyFieldChangesToMap(t *testing.T) {
	data := map[string]interface{}{
		"id":   1,
		"name": "Alice",
	}

	tests := []struct {
		name     string
		changes  []FieldChange
		expected map[string]interface{}
	}{
		{
			name: "add field",
			changes: []FieldChange{
				AddField("email", "alice@example.com"),
			},
			expected: map[string]interface{}{
				"id":    1,
				"name":  "Alice",
				"email": "alice@example.com",
			},
		},
		{
			name: "remove field",
			changes: []FieldChange{
				RemoveField("name"),
			},
			expected: map[string]interface{}{
				"id": 1,
			},
		},
		{
			name: "rename field",
			changes: []FieldChange{
				RenameField("name", "full_name"),
			},
			expected: map[string]interface{}{
				"id":        1,
				"full_name": "Alice",
			},
		},
		{
			name: "transform field",
			changes: []FieldChange{
				TransformField("name", func(value interface{}) interface{} {
					if str, ok := value.(string); ok {
						return str + " Smith"
					}
					return value
				}),
			},
			expected: map[string]interface{}{
				"id":   1,
				"name": "Alice Smith",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ApplyFieldChanges(data, tt.changes)
			if err != nil {
				t.Errorf("ApplyFieldChanges failed: %v", err)
			}

			resultMap, ok := result.(map[string]interface{})
			if !ok {
				t.Errorf("result should be a map")
			}

			for key, expectedValue := range tt.expected {
				if actualValue, exists := resultMap[key]; !exists {
					t.Errorf("expected key '%s' not found", key)
				} else if actualValue != expectedValue {
					t.Errorf("key '%s': expected '%v', got '%v'", key, expectedValue, actualValue)
				}
			}

			// Check that no unexpected keys exist
			for key := range resultMap {
				if _, expected := tt.expected[key]; !expected {
					t.Errorf("unexpected key '%s' found", key)
				}
			}
		})
	}
}

func TestApplyFieldChangesToSlice(t *testing.T) {
	data := []interface{}{
		map[string]interface{}{
			"id":   1,
			"name": "Alice",
		},
		map[string]interface{}{
			"id":   2,
			"name": "Bob",
		},
	}

	changes := []FieldChange{
		AddField("email", "default@example.com"),
	}

	result, err := ApplyFieldChanges(data, changes)
	if err != nil {
		t.Errorf("ApplyFieldChanges failed: %v", err)
	}

	resultSlice, ok := result.([]interface{})
	if !ok {
		t.Errorf("result should be a slice")
	}

	if len(resultSlice) != 2 {
		t.Errorf("expected 2 items, got %d", len(resultSlice))
	}

	// Check that email field was added to each item
	for i, item := range resultSlice {
		itemMap, ok := item.(map[string]interface{})
		if !ok {
			t.Errorf("item %d should be a map", i)
		}

		if itemMap["email"] != "default@example.com" {
			t.Errorf("item %d should have email field", i)
		}
	}
}

func TestStructToMap(t *testing.T) {
	type TestStruct struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		Email      string `json:"email,omitempty"`
		unexported string
	}

	data := TestStruct{
		ID:         1,
		Name:       "Alice",
		Email:      "alice@example.com",
		unexported: "should not appear",
	}

	result, err := structToMap(data)
	if err != nil {
		t.Errorf("structToMap failed: %v", err)
	}

	expected := map[string]interface{}{
		"id":    1,
		"name":  "Alice",
		"email": "alice@example.com",
	}

	for key, expectedValue := range expected {
		if actualValue, exists := result[key]; !exists {
			t.Errorf("expected key '%s' not found", key)
		} else if actualValue != expectedValue {
			t.Errorf("key '%s': expected '%v', got '%v'", key, expectedValue, actualValue)
		}
	}

	// Check that unexported field is not included
	if _, exists := result["unexported"]; exists {
		t.Errorf("unexported field should not be included")
	}
}

func TestCreateFieldChange(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		fieldName string
		options   []interface{}
		validate  func(FieldChange) bool
	}{
		{
			name:      "add field",
			operation: "add",
			fieldName: "email",
			options:   []interface{}{"default@example.com"},
			validate: func(fc FieldChange) bool {
				return fc.Operation == "add" &&
					fc.FieldName == "email" &&
					fc.DefaultValue == "default@example.com"
			},
		},
		{
			name:      "remove field",
			operation: "remove",
			fieldName: "name",
			options:   nil,
			validate: func(fc FieldChange) bool {
				return fc.Operation == "remove" && fc.FieldName == "name"
			},
		},
		{
			name:      "rename field",
			operation: "rename",
			fieldName: "name",
			options:   []interface{}{"full_name"},
			validate: func(fc FieldChange) bool {
				return fc.Operation == "rename" &&
					fc.FieldName == "name" &&
					fc.NewFieldName == "full_name"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			change := CreateFieldChange(tt.operation, tt.fieldName, tt.options...)

			if !tt.validate(change) {
				t.Errorf("field change validation failed")
			}
		})
	}
}

func TestHelperFunctions(t *testing.T) {
	// Test AddField
	addChange := AddField("email", "test@example.com")
	if addChange.Operation != "add" || addChange.FieldName != "email" || addChange.DefaultValue != "test@example.com" {
		t.Errorf("AddField helper failed")
	}

	// Test RemoveField
	removeChange := RemoveField("name")
	if removeChange.Operation != "remove" || removeChange.FieldName != "name" {
		t.Errorf("RemoveField helper failed")
	}

	// Test RenameField
	renameChange := RenameField("name", "full_name")
	if renameChange.Operation != "rename" || renameChange.FieldName != "name" || renameChange.NewFieldName != "full_name" {
		t.Errorf("RenameField helper failed")
	}

	// Test TransformField
	transformChange := TransformField("name", func(v interface{}) interface{} { return v })
	if transformChange.Operation != "transform" || transformChange.FieldName != "name" || transformChange.TransformFunc == nil {
		t.Errorf("TransformField helper failed")
	}
}
