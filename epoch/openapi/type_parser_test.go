package openapi

import (
	"reflect"
	"testing"
	"time"
)

func TestTypeParser_ParsePrimitiveTypes(t *testing.T) {
	tp := NewTypeParser()

	tests := []struct {
		name       string
		typ        reflect.Type
		wantType   string
		wantFormat string
	}{
		{
			name:     "string",
			typ:      reflect.TypeOf(""),
			wantType: "string",
		},
		{
			name:       "int",
			typ:        reflect.TypeOf(int(0)),
			wantType:   "integer",
			wantFormat: "int64",
		},
		{
			name:       "int64",
			typ:        reflect.TypeOf(int64(0)),
			wantType:   "integer",
			wantFormat: "int64",
		},
		{
			name:       "int32",
			typ:        reflect.TypeOf(int32(0)),
			wantType:   "integer",
			wantFormat: "int32",
		},
		{
			name:       "float32",
			typ:        reflect.TypeOf(float32(0)),
			wantType:   "number",
			wantFormat: "float",
		},
		{
			name:       "float64",
			typ:        reflect.TypeOf(float64(0)),
			wantType:   "number",
			wantFormat: "double",
		},
		{
			name:     "bool",
			typ:      reflect.TypeOf(false),
			wantType: "boolean",
		},
		{
			name:       "time.Time",
			typ:        reflect.TypeOf(time.Time{}),
			wantType:   "string",
			wantFormat: "date-time",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaRef, err := tp.ParseType(tt.typ)
			if err != nil {
				t.Fatalf("ParseType() error = %v", err)
			}

			if schemaRef.Value == nil {
				t.Fatal("ParseType() returned nil schema")
			}

			if !schemaRef.Value.Type.Is(tt.wantType) {
				t.Errorf("ParseType() type = %v, want %v", schemaRef.Value.Type, tt.wantType)
			}

			if tt.wantFormat != "" && schemaRef.Value.Format != tt.wantFormat {
				t.Errorf("ParseType() format = %v, want %v", schemaRef.Value.Format, tt.wantFormat)
			}
		})
	}
}

func TestTypeParser_ParseStruct(t *testing.T) {
	type SimpleStruct struct {
		Name  string `json:"name" binding:"required,max=50"`
		Email string `json:"email" binding:"email"`
		Age   int    `json:"age,omitempty"`
	}

	tp := NewTypeParser()
	schemaRef, err := tp.ParseType(reflect.TypeOf(SimpleStruct{}))
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	// Should create a component reference
	if schemaRef.Ref == "" {
		t.Error("Expected $ref for named struct")
	}

	// Check component was created
	components := tp.GetComponents()
	if len(components) == 0 {
		t.Fatal("Expected component to be created")
	}

	schema := components["SimpleStruct"].Value
	if schema == nil {
		t.Fatal("Component schema is nil")
	}

	// Check properties
	if len(schema.Properties) != 3 {
		t.Errorf("Expected 3 properties, got %d", len(schema.Properties))
	}

	// Check name field
	nameSchema := schema.Properties["name"]
	if nameSchema == nil {
		t.Fatal("name property not found")
	}
	if nameSchema.Value.MaxLength == nil || *nameSchema.Value.MaxLength != 50 {
		t.Errorf("name maxLength = %v, want 50", nameSchema.Value.MaxLength)
	}

	// Check required fields (only 'name' has binding:"required")
	if len(schema.Required) != 1 {
		t.Errorf("Expected 1 required field (name), got %d: %v", len(schema.Required), schema.Required)
	}
	if len(schema.Required) > 0 && schema.Required[0] != "name" {
		t.Errorf("Expected 'name' to be required, got %v", schema.Required)
	}
}

func TestTypeParser_ParseSlice(t *testing.T) {
	tp := NewTypeParser()

	type Item struct {
		ID string `json:"id"`
	}

	schemaRef, err := tp.ParseType(reflect.TypeOf([]Item{}))
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	if !schemaRef.Value.Type.Is("array") {
		t.Errorf("Expected type=array, got %s", schemaRef.Value.Type)
	}

	if schemaRef.Value.Items == nil {
		t.Fatal("Expected items schema")
	}

	// Should have a $ref to Item component
	if schemaRef.Value.Items.Ref == "" {
		t.Error("Expected $ref for array items")
	}
}

func TestTypeParser_ParseMap(t *testing.T) {
	tp := NewTypeParser()

	tests := []struct {
		name     string
		typ      reflect.Type
		wantType string
	}{
		{
			name:     "map[string]string",
			typ:      reflect.TypeOf(map[string]string{}),
			wantType: "object",
		},
		{
			name:     "map[string]interface{}",
			typ:      reflect.TypeOf(map[string]interface{}{}),
			wantType: "object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schemaRef, err := tp.ParseType(tt.typ)
			if err != nil {
				t.Fatalf("ParseType() error = %v", err)
			}

			if !schemaRef.Value.Type.Is(tt.wantType) {
				t.Errorf("Expected type=%s, got %s", tt.wantType, schemaRef.Value.Type)
			}

			if schemaRef.Value.AdditionalProperties.Schema == nil {
				t.Error("Expected additionalProperties to be set")
			}
		})
	}
}

func TestTypeParser_ParsePointer(t *testing.T) {
	tp := NewTypeParser()

	type TestStruct struct {
		Value string
	}

	// Pointer to struct should unwrap to struct
	ptrType := reflect.TypeOf(&TestStruct{})
	schemaRef, err := tp.ParseType(ptrType)
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	// Should create reference to TestStruct component
	if schemaRef.Ref == "" {
		t.Error("Expected $ref for pointer to struct")
	}
}

func TestTypeParser_ParseEmbeddedStruct(t *testing.T) {
	type BaseStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	type ExtendedStruct struct {
		BaseStruct
		Email string `json:"email"`
	}

	tp := NewTypeParser()
	_, err := tp.ParseType(reflect.TypeOf(ExtendedStruct{}))
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	components := tp.GetComponents()
	schema := components["ExtendedStruct"].Value

	// Should have all fields from embedded struct promoted
	if len(schema.Properties) != 3 {
		t.Errorf("Expected 3 properties (id, name, email), got %d", len(schema.Properties))
	}

	if schema.Properties["id"] == nil {
		t.Error("Expected 'id' field from embedded struct")
	}
	if schema.Properties["name"] == nil {
		t.Error("Expected 'name' field from embedded struct")
	}
	if schema.Properties["email"] == nil {
		t.Error("Expected 'email' field")
	}
}

func TestTypeParser_ParseInterface(t *testing.T) {
	tp := NewTypeParser()

	type TestStruct struct {
		Data interface{} `json:"data"`
	}

	_, err := tp.ParseType(reflect.TypeOf(TestStruct{}))
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	components := tp.GetComponents()
	schema := components["TestStruct"].Value

	dataField := schema.Properties["data"]
	if dataField == nil {
		t.Fatal("data field not found")
	}

	// interface{} should be type: object
	if !dataField.Value.Type.Is("object") {
		t.Errorf("Expected type=object for interface{}, got %s", dataField.Value.Type)
	}
}

func TestTypeParser_Cache(t *testing.T) {
	tp := NewTypeParser()

	type TestStruct struct {
		Value string
	}

	typ := reflect.TypeOf(TestStruct{})

	// Parse once
	ref1, err := tp.ParseType(typ)
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	// Parse again - should use cache
	ref2, err := tp.ParseType(typ)
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	// Should return same reference
	if ref1.Ref != ref2.Ref {
		t.Error("Expected cached result to match")
	}

	// Reset and parse again
	tp.Reset()
	ref3, err := tp.ParseType(typ)
	if err != nil {
		t.Fatalf("ParseType() error = %v", err)
	}

	// After reset, should create new component
	if ref3.Ref != ref1.Ref {
		t.Error("Reference should be same after reset (same component name)")
	}
}

func TestTypeParser_CircularReference(t *testing.T) {
	type Node struct {
		Value string
		Next  *Node
	}

	tp := NewTypeParser()
	_, err := tp.ParseType(reflect.TypeOf(Node{}))
	if err != nil {
		t.Fatalf("ParseType() should handle circular references, got error = %v", err)
	}

	// Should not panic and should create component
	components := tp.GetComponents()
	if len(components) == 0 {
		t.Error("Expected component to be created for circular reference")
	}
}
