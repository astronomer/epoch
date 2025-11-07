package openapi

import (
	"reflect"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

func TestTagParser_ParseJSONTag(t *testing.T) {
	tp := NewTagParser()

	tests := []struct {
		name          string
		tag           string
		wantFieldName string
		wantOmitempty bool
	}{
		{
			name:          "simple field name",
			tag:           "user_id",
			wantFieldName: "user_id",
			wantOmitempty: false,
		},
		{
			name:          "field name with omitempty",
			tag:           "email,omitempty",
			wantFieldName: "email",
			wantOmitempty: true,
		},
		{
			name:          "skip field",
			tag:           "-",
			wantFieldName: "-",
			wantOmitempty: false,
		},
		{
			name:          "empty tag",
			tag:           "",
			wantFieldName: "",
			wantOmitempty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotFieldName, gotOmitempty := tp.ParseJSONTag(tt.tag)
			if gotFieldName != tt.wantFieldName {
				t.Errorf("ParseJSONTag() gotFieldName = %v, want %v", gotFieldName, tt.wantFieldName)
			}
			if gotOmitempty != tt.wantOmitempty {
				t.Errorf("ParseJSONTag() gotOmitempty = %v, want %v", gotOmitempty, tt.wantOmitempty)
			}
		})
	}
}

func TestTagParser_ApplyValidationTags(t *testing.T) {
	tp := NewTagParser()

	tests := []struct {
		name        string
		bindingTag  string
		validateTag string
		schemaType  string
		check       func(*testing.T, *openapi3.Schema)
	}{
		{
			name:       "email format",
			bindingTag: "email",
			schemaType: "string",
			check: func(t *testing.T, s *openapi3.Schema) {
				if s.Format != "email" {
					t.Errorf("expected format=email, got %s", s.Format)
				}
			},
		},
		{
			name:       "max length",
			bindingTag: "max=50",
			schemaType: "string",
			check: func(t *testing.T, s *openapi3.Schema) {
				if s.MaxLength == nil || *s.MaxLength != 50 {
					t.Errorf("expected maxLength=50, got %v", s.MaxLength)
				}
			},
		},
		{
			name:       "min length",
			bindingTag: "min=1",
			schemaType: "string",
			check: func(t *testing.T, s *openapi3.Schema) {
				if s.MinLength != 1 {
					t.Errorf("expected minLength=1, got %d", s.MinLength)
				}
			},
		},
		{
			name:       "enum values",
			bindingTag: "oneof=active inactive pending",
			schemaType: "string",
			check: func(t *testing.T, s *openapi3.Schema) {
				if len(s.Enum) != 3 {
					t.Errorf("expected 3 enum values, got %d", len(s.Enum))
				}
			},
		},
		{
			name:        "validate tag",
			validateTag: "required,email",
			schemaType:  "string",
			check: func(t *testing.T, s *openapi3.Schema) {
				if s.Format != "email" {
					t.Errorf("expected format=email, got %s", s.Format)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := &openapi3.Schema{Type: &openapi3.Types{tt.schemaType}}
			tp.ApplyValidationTags(schema, tt.bindingTag, tt.validateTag)
			tt.check(t, schema)
		})
	}
}

func TestTagParser_ApplyCommonTags(t *testing.T) {
	tp := NewTagParser()

	type testStruct struct {
		Field1 string `json:"field1" example:"test value"`
		Field2 string `json:"field2" enums:"A,B,C"`
		Field3 string `json:"field3" format:"date-time"`
		Field4 string `json:"field4" description:"A test field"`
	}

	structType := reflect.TypeOf(testStruct{})

	tests := []struct {
		name      string
		fieldName string
		check     func(*testing.T, *openapi3.Schema)
	}{
		{
			name:      "example tag",
			fieldName: "Field1",
			check: func(t *testing.T, s *openapi3.Schema) {
				if s.Example != "test value" {
					t.Errorf("expected example='test value', got %v", s.Example)
				}
			},
		},
		{
			name:      "enums tag",
			fieldName: "Field2",
			check: func(t *testing.T, s *openapi3.Schema) {
				if len(s.Enum) != 3 {
					t.Errorf("expected 3 enum values, got %d", len(s.Enum))
				}
			},
		},
		{
			name:      "format tag",
			fieldName: "Field3",
			check: func(t *testing.T, s *openapi3.Schema) {
				if s.Format != "date-time" {
					t.Errorf("expected format='date-time', got %s", s.Format)
				}
			},
		},
		{
			name:      "description tag",
			fieldName: "Field4",
			check: func(t *testing.T, s *openapi3.Schema) {
				if s.Description != "A test field" {
					t.Errorf("expected description='A test field', got %s", s.Description)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			field, _ := structType.FieldByName(tt.fieldName)
			schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
			tp.ApplyCommonTags(schema, field)
			tt.check(t, schema)
		})
	}
}

func TestTagParser_IsRequired(t *testing.T) {
	tp := NewTagParser()

	tests := []struct {
		name        string
		bindingTag  string
		validateTag string
		omitempty   bool
		want        bool
	}{
		{
			name:       "required in binding tag",
			bindingTag: "required",
			want:       true,
		},
		{
			name:        "required in validate tag",
			validateTag: "required",
			want:        true,
		},
		{
			name:       "omitempty overrides",
			bindingTag: "required",
			omitempty:  true,
			want:       false,
		},
		{
			name: "not required",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tp.IsRequired(tt.bindingTag, tt.validateTag, tt.omitempty)
			if got != tt.want {
				t.Errorf("IsRequired() = %v, want %v", got, tt.want)
			}
		})
	}
}
