package openapi

import (
	"reflect"
	"testing"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
)

// Test types
type TestUserRequest struct {
	Name  string `json:"name" binding:"required,max=50"`
	Email string `json:"email" binding:"required,email"`
}

type TestUserResponse struct {
	ID    int    `json:"id" validate:"required"`
	Name  string `json:"name" validate:"required"`
	Email string `json:"email,omitempty" validate:"required,email"`
}

func TestSchemaGenerator_NewSchemaGenerator(t *testing.T) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	registry := epoch.NewEndpointRegistry()

	config := SchemaGeneratorConfig{
		VersionBundle: versionBundle,
		TypeRegistry:  registry,
		OutputFormat:  "yaml",
	}

	generator := NewSchemaGenerator(config)

	if generator == nil {
		t.Fatal("NewSchemaGenerator returned nil")
	}

	if generator.config.OutputFormat != "yaml" {
		t.Errorf("Expected output format 'yaml', got %s", generator.config.OutputFormat)
	}
}

func TestSchemaGenerator_GetRegisteredTypes(t *testing.T) {
	// Create a version bundle
	v1, _ := epoch.NewDateVersion("2024-01-01")
	versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	// Create registry and register types
	registry := epoch.NewEndpointRegistry()
	registry.Register("GET", "/users/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users/:id",
		ResponseType: reflect.TypeOf(TestUserResponse{}),
	})
	registry.Register("POST", "/users", &epoch.EndpointDefinition{
		Method:       "POST",
		PathPattern:  "/users",
		RequestType:  reflect.TypeOf(TestUserRequest{}),
		ResponseType: reflect.TypeOf(TestUserResponse{}),
	})

	config := SchemaGeneratorConfig{
		VersionBundle: versionBundle,
		TypeRegistry:  registry,
	}

	generator := NewSchemaGenerator(config)
	types := generator.getRegisteredTypes()

	if len(types) != 2 {
		t.Errorf("Expected 2 registered types, got %d", len(types))
	}

	// Check that both types are present
	typeMap := make(map[reflect.Type]bool)
	for _, typ := range types {
		typeMap[typ] = true
	}

	if !typeMap[reflect.TypeOf(TestUserRequest{})] {
		t.Error("TestUserRequest not found in registered types")
	}
	if !typeMap[reflect.TypeOf(TestUserResponse{})] {
		t.Error("TestUserResponse not found in registered types")
	}
}

func TestSchemaGenerator_GetVersionSuffix(t *testing.T) {
	versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{})

	config := SchemaGeneratorConfig{
		VersionBundle: versionBundle,
	}

	generator := NewSchemaGenerator(config)

	tests := []struct {
		name    string
		version *epoch.Version
		want    string
	}{
		{
			name:    "HEAD version",
			version: epoch.NewHeadVersion(),
			want:    "",
		},
		{
			name: "date version",
			version: func() *epoch.Version {
				v, _ := epoch.NewDateVersion("2024-01-01")
				return v
			}(),
			want: "V20240101",
		},
		{
			name: "semver version",
			version: func() *epoch.Version {
				v, _ := epoch.NewSemverVersion("1.2.3")
				return v
			}(),
			want: "V123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generator.getVersionSuffix(tt.version)
			if got != tt.want {
				t.Errorf("getVersionSuffix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSchemaGenerator_CloneSpec(t *testing.T) {
	original := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"TestType": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
				}),
			},
		},
	}

	versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{})
	registry := epoch.NewEndpointRegistry()

	config := SchemaGeneratorConfig{
		VersionBundle: versionBundle,
		TypeRegistry:  registry,
	}

	generator := NewSchemaGenerator(config)
	clone := generator.cloneSpec(original)

	// Check that it's a different instance
	if clone == original {
		t.Error("cloneSpec returned same instance")
	}

	// Check that values match
	if clone.OpenAPI != original.OpenAPI {
		t.Errorf("OpenAPI version doesn't match: got %s, want %s", clone.OpenAPI, original.OpenAPI)
	}

	if clone.Info.Title != original.Info.Title {
		t.Errorf("Info.Title doesn't match: got %s, want %s", clone.Info.Title, original.Info.Title)
	}

	// Check that components.schemas are preserved from base spec
	if len(clone.Components.Schemas) != 1 {
		t.Errorf("Expected 1 schema in clone, got %d", len(clone.Components.Schemas))
	}

	if clone.Components.Schemas["TestType"] == nil {
		t.Error("Expected 'TestType' schema to be preserved")
	}
}

func TestSchemaGenerator_Integration(t *testing.T) {
	// This is a basic integration test to ensure all pieces work together

	// Define versions
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")

	// Create version change
	change := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email field").
		ForType(TestUserResponse{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		Build()

	// Create Epoch instance
	versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v2, v1})

	// Create registry
	registry := epoch.NewEndpointRegistry()
	registry.Register("GET", "/users/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users/:id",
		ResponseType: reflect.TypeOf(TestUserResponse{}),
	})

	// Add changes to v2
	v2.Changes = []epoch.VersionChangeInterface{change}

	// Create generator
	config := SchemaGeneratorConfig{
		VersionBundle: versionBundle,
		TypeRegistry:  registry,
		OutputFormat:  "yaml",
	}

	generator := NewSchemaGenerator(config)

	// Generate schema for HEAD
	headVersion := epoch.NewHeadVersion()
	schema, err := generator.GetSchemaForType(
		reflect.TypeOf(TestUserResponse{}),
		headVersion,
		SchemaDirectionResponse,
	)

	if err != nil {
		t.Fatalf("GetSchemaForType failed: %v", err)
	}

	if schema == nil {
		t.Fatal("GetSchemaForType returned nil schema")
	}

	// Check that required fields are present
	if len(schema.Properties) == 0 {
		t.Error("Expected properties in schema")
	}

	if schema.Properties["id"] == nil {
		t.Error("Expected 'id' property")
	}

	if schema.Properties["name"] == nil {
		t.Error("Expected 'name' property")
	}
}

// TestSchemaGenerator_SmartMerging tests that base schemas are preserved and transformations applied
func TestSchemaGenerator_SmartMerging(t *testing.T) {
	// Setup versions
	v1, _ := epoch.NewDateVersion("2024-01-01")
	headVersion := epoch.NewHeadVersion()
	versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	// Create version change: remove 'email' field in v1
	change := epoch.NewVersionChangeBuilder(v1, headVersion).
		Description("Add email field").
		ForType(TestUserResponse{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		Build()

	v1.Changes = []epoch.VersionChangeInterface{change}

	// Create registry
	registry := epoch.NewEndpointRegistry()
	registry.Register("GET", "/users/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users/:id",
		ResponseType: reflect.TypeOf(TestUserResponse{}),
	})

	// Create base spec with schema that has descriptions
	baseSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"TestUserResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"integer"},
							Description: "User ID from base spec",
						}),
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User name from base spec",
						}),
						"email": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User email from base spec",
						}),
					},
				}),
				"ErrorResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"message": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "Error message",
						}),
					},
				}),
			},
		},
	}

	// Create generator
	config := SchemaGeneratorConfig{
		VersionBundle: versionBundle,
		TypeRegistry:  registry,
	}
	generator := NewSchemaGenerator(config)

	// Generate HEAD version spec
	headSpec, err := generator.GenerateSpecForVersion(baseSpec, headVersion)
	if err != nil {
		t.Fatalf("Failed to generate HEAD spec: %v", err)
	}

	// Verify HEAD spec has bare schema name
	if headSpec.Components.Schemas["TestUserResponse"] == nil {
		t.Error("HEAD spec should have TestUserResponse")
	}

	// Verify description is preserved
	headSchema := headSpec.Components.Schemas["TestUserResponse"].Value
	if headSchema.Properties["id"].Value.Description != "User ID from base spec" {
		t.Error("Description should be preserved from base spec")
	}

	// Verify unmanaged schema is preserved
	if headSpec.Components.Schemas["ErrorResponse"] == nil {
		t.Error("HEAD spec should have ErrorResponse (unmanaged schema)")
	}

	// Generate v1 spec
	v1Spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
	if err != nil {
		t.Fatalf("Failed to generate v1 spec: %v", err)
	}

	// Verify v1 spec has versioned schema name only
	if v1Spec.Components.Schemas["TestUserResponseV20240101"] == nil {
		t.Error("v1 spec should have TestUserResponseV20240101")
	}

	// Verify bare name is NOT in v1 spec
	if v1Spec.Components.Schemas["TestUserResponse"] != nil {
		t.Error("v1 spec should NOT have bare TestUserResponse (only versioned)")
	}

	// Verify transformation was applied (email removed)
	v1Schema := v1Spec.Components.Schemas["TestUserResponseV20240101"].Value
	if v1Schema.Properties["email"] != nil {
		t.Error("email field should be removed in v1")
	}

	// Verify description is still preserved after transformation
	if v1Schema.Properties["id"].Value.Description != "User ID from base spec" {
		t.Error("Description should be preserved after transformation")
	}

	// Verify unmanaged schema is preserved in v1
	if v1Spec.Components.Schemas["ErrorResponse"] == nil {
		t.Error("v1 spec should have ErrorResponse (unmanaged schema)")
	}
}

// Benchmark type parsing
func BenchmarkTypeParser_ParseStruct(b *testing.B) {
	type BenchStruct struct {
		ID        int       `json:"id" validate:"required"`
		Name      string    `json:"name" validate:"required,max=100"`
		Email     string    `json:"email" validate:"required,email"`
		CreatedAt time.Time `json:"created_at" format:"date-time"`
		Tags      []string  `json:"tags"`
	}

	tp := NewTypeParser()
	typ := reflect.TypeOf(BenchStruct{})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := tp.ParseType(typ)
		if err != nil {
			b.Fatal(err)
		}
		tp.Reset()
	}
}
