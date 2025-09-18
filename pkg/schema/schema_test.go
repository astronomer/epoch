package schema

import (
	"reflect"
	"testing"
)

// Test structs for schema analysis
type UserV1 struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type UserV2 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserV3 struct {
	ID        int    `json:"id"`
	FullName  string `json:"full_name"` // renamed from Name
	Email     string `json:"email"`
	CreatedAt string `json:"created_at"`
}

func TestSchemaAnalyzer(t *testing.T) {
	analyzer := NewSchemaAnalyzer()

	user := UserV1{ID: 1, Name: "Alice"}
	schema, err := analyzer.AnalyzeStruct(user)
	if err != nil {
		t.Fatalf("Failed to analyze struct: %v", err)
	}

	if schema.Name != "UserV1" {
		t.Errorf("Expected schema name 'UserV1', got '%s'", schema.Name)
	}

	if len(schema.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(schema.Fields))
	}

	// Check fields
	expectedFields := map[string]string{
		"ID":   "id",
		"Name": "name",
	}

	for _, field := range schema.Fields {
		expectedJSONName, exists := expectedFields[field.Name]
		if !exists {
			t.Errorf("Unexpected field: %s", field.Name)
			continue
		}

		if field.JSONName != expectedJSONName {
			t.Errorf("Field %s: expected JSON name '%s', got '%s'",
				field.Name, expectedJSONName, field.JSONName)
		}

		if !field.IsExported {
			t.Errorf("Field %s should be exported", field.Name)
		}
	}
}

func TestSchemaComparison(t *testing.T) {
	analyzer := NewSchemaAnalyzer()

	userV1 := UserV1{}
	userV2 := UserV2{}

	schemaV1, err := analyzer.AnalyzeStruct(userV1)
	if err != nil {
		t.Fatalf("Failed to analyze UserV1: %v", err)
	}

	schemaV2, err := analyzer.AnalyzeStruct(userV2)
	if err != nil {
		t.Fatalf("Failed to analyze UserV2: %v", err)
	}

	diff, err := analyzer.CompareSchemas(schemaV1, schemaV2)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	// Should have one change: email field added
	if len(diff.Changes) != 1 {
		t.Errorf("Expected 1 change, got %d", len(diff.Changes))
	}

	if len(diff.Changes) > 0 {
		change := diff.Changes[0]
		if change.Type != FieldDiffTypeAdded {
			t.Errorf("Expected field addition, got %s", change.Type.String())
		}
		if change.FieldName != "email" {
			t.Errorf("Expected field name 'email', got '%s'", change.FieldName)
		}
	}
}

func TestComplexSchemaComparison(t *testing.T) {
	analyzer := NewSchemaAnalyzer()

	userV2 := UserV2{}
	userV3 := UserV3{}

	schemaV2, err := analyzer.AnalyzeStruct(userV2)
	if err != nil {
		t.Fatalf("Failed to analyze UserV2: %v", err)
	}

	schemaV3, err := analyzer.AnalyzeStruct(userV3)
	if err != nil {
		t.Fatalf("Failed to analyze UserV3: %v", err)
	}

	diff, err := analyzer.CompareSchemas(schemaV2, schemaV3)
	if err != nil {
		t.Fatalf("Failed to compare schemas: %v", err)
	}

	// Should have changes: name removed, full_name added, created_at added
	if len(diff.Changes) != 3 {
		t.Errorf("Expected 3 changes, got %d", len(diff.Changes))
	}

	changeTypes := make(map[FieldDiffType]int)
	for _, change := range diff.Changes {
		changeTypes[change.Type]++
	}

	if changeTypes[FieldDiffTypeAdded] != 2 {
		t.Errorf("Expected 2 added fields, got %d", changeTypes[FieldDiffTypeAdded])
	}

	if changeTypes[FieldDiffTypeRemoved] != 1 {
		t.Errorf("Expected 1 removed field, got %d", changeTypes[FieldDiffTypeRemoved])
	}
}

func TestToMigrationChanges(t *testing.T) {
	analyzer := NewSchemaAnalyzer()

	userV1 := UserV1{}
	userV2 := UserV2{}

	schemaV1, _ := analyzer.AnalyzeStruct(userV1)
	schemaV2, _ := analyzer.AnalyzeStruct(userV2)

	diff, _ := analyzer.CompareSchemas(schemaV1, schemaV2)

	migrationChanges := diff.ToMigrationChanges(analyzer)

	if len(migrationChanges) != 1 {
		t.Errorf("Expected 1 migration change, got %d", len(migrationChanges))
	}

	if len(migrationChanges) > 0 {
		change := migrationChanges[0]
		if change.Operation != "add" {
			t.Errorf("Expected 'add' operation, got '%s'", change.Operation)
		}
		if change.FieldName != "email" {
			t.Errorf("Expected field name 'email', got '%s'", change.FieldName)
		}
		if change.DefaultValue != "" {
			t.Errorf("Expected empty string default, got %v", change.DefaultValue)
		}
	}
}

func TestGetZeroValue(t *testing.T) {
	analyzer := NewSchemaAnalyzer()

	tests := []struct {
		typ      reflect.Type
		expected interface{}
	}{
		{reflect.TypeOf(""), ""},
		{reflect.TypeOf(0), 0},
		{reflect.TypeOf(0.0), 0.0},
		{reflect.TypeOf(true), false},
		{reflect.TypeOf([]int{}), []interface{}{}},
		{reflect.TypeOf(map[string]int{}), map[string]interface{}{}},
	}

	for _, tt := range tests {
		result := analyzer.getZeroValue(tt.typ)
		if !reflect.DeepEqual(result, tt.expected) {
			t.Errorf("Type %v: expected %v, got %v", tt.typ, tt.expected, result)
		}
	}
}

func TestStructTransformer(t *testing.T) {
	transformer := NewStructTransformer()

	// Test transforming UserV1 to UserV2 schema
	userV1 := UserV1{ID: 1, Name: "Alice"}

	userV2Schema, _ := transformer.analyzer.AnalyzeStruct(UserV2{})

	transformed, err := transformer.TransformStruct(userV1, userV2Schema)
	if err != nil {
		t.Fatalf("Transform failed: %v", err)
	}

	transformedMap, ok := transformed.(map[string]interface{})
	if !ok {
		t.Fatalf("Transformed result should be a map")
	}

	// Should have email field added
	if _, exists := transformedMap["email"]; !exists {
		t.Errorf("Transformed struct should have email field")
	}

	// Original fields should still be present
	if transformedMap["id"] != 1 {
		t.Errorf("ID field should be preserved")
	}

	if transformedMap["name"] != "Alice" {
		t.Errorf("Name field should be preserved")
	}
}

func TestGenerateMigrationChanges(t *testing.T) {
	transformer := NewStructTransformer()

	userV1 := UserV1{}
	userV2 := UserV2{}

	changes, err := transformer.GenerateMigrationChanges(userV1, userV2)
	if err != nil {
		t.Fatalf("Failed to generate migration changes: %v", err)
	}

	if len(changes) != 1 {
		t.Errorf("Expected 1 migration change, got %d", len(changes))
	}

	if len(changes) > 0 {
		change := changes[0]
		if change.Operation != "add" {
			t.Errorf("Expected 'add' operation, got '%s'", change.Operation)
		}
		if change.FieldName != "email" {
			t.Errorf("Expected field name 'email', got '%s'", change.FieldName)
		}
	}
}

func TestParseJSONTag(t *testing.T) {
	analyzer := NewSchemaAnalyzer()

	tests := []struct {
		tag          string
		expectedName string
		expectedOmit bool
	}{
		{"name", "name", false},
		{"name,omitempty", "name", true},
		{"full_name,omitempty", "full_name", true},
		{"-", "", false},
		{"", "", false},
	}

	for _, tt := range tests {
		name, omit := analyzer.parseJSONTag(tt.tag)
		if name != tt.expectedName {
			t.Errorf("Tag '%s': expected name '%s', got '%s'", tt.tag, tt.expectedName, name)
		}
		if omit != tt.expectedOmit {
			t.Errorf("Tag '%s': expected omitempty %t, got %t", tt.tag, tt.expectedOmit, omit)
		}
	}
}

func TestFieldsEqual(t *testing.T) {
	analyzer := NewSchemaAnalyzer()

	field1 := Field{
		Type:      reflect.TypeOf(""),
		JSONName:  "name",
		OmitEmpty: false,
	}

	field2 := Field{
		Type:      reflect.TypeOf(""),
		JSONName:  "name",
		OmitEmpty: false,
	}

	field3 := Field{
		Type:      reflect.TypeOf(0),
		JSONName:  "name",
		OmitEmpty: false,
	}

	if !analyzer.fieldsEqual(field1, field2) {
		t.Errorf("Identical fields should be equal")
	}

	if analyzer.fieldsEqual(field1, field3) {
		t.Errorf("Fields with different types should not be equal")
	}
}
