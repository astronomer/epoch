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
	versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})

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

	// NEW BEHAVIOR: Schema exists in base spec, so it's transformed in place
	// Verify v1 spec has the schema with the SAME name (transformed in place)
	if v1Spec.Components.Schemas["TestUserResponse"] == nil {
		t.Error("v1 spec should have TestUserResponse (transformed in place)")
	}

	// Verify transformation was applied (email removed)
	v1Schema := v1Spec.Components.Schemas["TestUserResponse"].Value
	if v1Schema == nil {
		t.Fatal("v1 schema value is nil")
	}
	if v1Schema.Properties["email"] != nil {
		t.Error("email field should be removed in v1")
	}

	// Verify description is still preserved after transformation
	if v1Schema.Properties["id"] != nil && v1Schema.Properties["id"].Value.Description != "User ID from base spec" {
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

// Test types for Swag integration tests
type UpdateExampleRequest struct {
	BetterNewName string `json:"betterNewName"`
	Timezone      string `json:"timezone"`
}

type ExistingRequest struct {
	Field1 string `json:"field1"`
}

type MissingRequest struct {
	Field2 string `json:"field2"`
}

// Test 1: Transform existing Swag schemas with package prefix
func TestSchemaGenerator_TransformSwagSchemas(t *testing.T) {
	// Create base spec with Swag-style naming
	baseSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"versionedapi.UpdateExampleRequest": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"betterNewName": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
						"timezone": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
					},
				}),
			},
		},
	}

	// Setup versions and migrations
	v1, _ := epoch.NewSemverVersion("1.0")
	headVersion := epoch.NewHeadVersion()

	change := epoch.NewVersionChangeBuilder(v1, headVersion).
		Description("Add betterNewName and timezone fields").
		ForType(UpdateExampleRequest{}).
		ResponseToPreviousVersion().
		RenameField("betterNewName", "name").
		RemoveField("timezone").
		Build()

	vb, err := epoch.NewVersionBundle([]*epoch.Version{headVersion, v1})
	if err != nil {
		t.Fatalf("Failed to create version bundle: %v", err)
	}
	v1.Changes = []epoch.VersionChangeInterface{change}

	// Configure with schema name mapper
	registry := epoch.NewEndpointRegistry()
	registry.Register("POST", "/examples", &epoch.EndpointDefinition{
		Method:      "POST",
		PathPattern: "/examples",
		RequestType: reflect.TypeOf(UpdateExampleRequest{}),
	})

	config := SchemaGeneratorConfig{
		VersionBundle: vb,
		TypeRegistry:  registry,
		SchemaNameMapper: func(name string) string {
			return "versionedapi." + name
		},
	}

	generator := NewSchemaGenerator(config)

	// Generate v1.0 spec
	v1Spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
	if err != nil {
		t.Fatalf("Failed to generate v1 spec: %v", err)
	}

	// Verify: v1 schema transformed in place with correct fields
	v1Schema := v1Spec.Components.Schemas["versionedapi.UpdateExampleRequest"]
	if v1Schema == nil {
		t.Fatal("v1 schema not found")
	}
	if v1Schema.Value == nil {
		t.Fatal("v1 schema value is nil")
	}

	// Should have "name" (renamed from betterNewName)
	if v1Schema.Value.Properties["name"] == nil {
		t.Error("v1 schema should have 'name' field")
	}
	// Should NOT have "betterNewName"
	if v1Schema.Value.Properties["betterNewName"] != nil {
		t.Error("v1 schema should NOT have 'betterNewName' field")
	}
	// Should NOT have "timezone" (removed)
	if v1Schema.Value.Properties["timezone"] != nil {
		t.Error("v1 schema should NOT have 'timezone' field")
	}

	// Generate HEAD spec
	headSpec, err := generator.GenerateSpecForVersion(baseSpec, headVersion)
	if err != nil {
		t.Fatalf("Failed to generate head spec: %v", err)
	}

	// Verify: HEAD schema is unchanged (no transformations)
	headSchema := headSpec.Components.Schemas["versionedapi.UpdateExampleRequest"]
	if headSchema == nil {
		t.Fatal("HEAD schema not found")
	}
	if headSchema.Value.Properties["betterNewName"] == nil {
		t.Error("HEAD schema should have 'betterNewName' field")
	}
	if headSchema.Value.Properties["timezone"] == nil {
		t.Error("HEAD schema should have 'timezone' field")
	}
}

// Test 2: Fallback to generation when schema doesn't exist
func TestSchemaGenerator_FallbackToGeneration(t *testing.T) {
	// Empty base spec (no schemas)
	baseSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{},
		},
	}

	v1, _ := epoch.NewSemverVersion("1.0")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	registry := epoch.NewEndpointRegistry()
	registry.Register("POST", "/examples", &epoch.EndpointDefinition{
		Method:      "POST",
		PathPattern: "/examples",
		RequestType: reflect.TypeOf(UpdateExampleRequest{}),
	})

	config := SchemaGeneratorConfig{
		VersionBundle: vb,
		TypeRegistry:  registry,
		SchemaNameMapper: func(name string) string {
			return "versionedapi." + name
		},
	}

	generator := NewSchemaGenerator(config)
	v1Spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
	if err != nil {
		t.Fatalf("Failed to generate v1 spec: %v", err)
	}

	// Should generate schema from scratch with versioned name
	generatedSchema := v1Spec.Components.Schemas["UpdateExampleRequestV10"]
	if generatedSchema == nil {
		t.Error("Generated schema not found")
	}
	if generatedSchema != nil && generatedSchema.Value == nil {
		t.Error("Generated schema value is nil")
	}
}

// Test 3: Mixed scenario - some schemas exist, some don't
func TestSchemaGenerator_MixedScenario(t *testing.T) {
	baseSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				// Only one schema exists
				"versionedapi.ExistingRequest": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"field1": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
					},
				}),
				// MissingRequest is NOT in base spec
			},
		},
	}

	v1, _ := epoch.NewSemverVersion("1.0")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	registry := epoch.NewEndpointRegistry()
	registry.Register("POST", "/existing", &epoch.EndpointDefinition{
		Method:      "POST",
		PathPattern: "/existing",
		RequestType: reflect.TypeOf(ExistingRequest{}),
	})
	registry.Register("POST", "/missing", &epoch.EndpointDefinition{
		Method:      "POST",
		PathPattern: "/missing",
		RequestType: reflect.TypeOf(MissingRequest{}),
	})

	config := SchemaGeneratorConfig{
		VersionBundle: vb,
		TypeRegistry:  registry,
		SchemaNameMapper: func(name string) string {
			return "versionedapi." + name
		},
	}

	generator := NewSchemaGenerator(config)
	v1Spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
	if err != nil {
		t.Fatalf("Failed to generate v1 spec: %v", err)
	}

	// ExistingRequest: should be transformed in place
	if v1Spec.Components.Schemas["versionedapi.ExistingRequest"] == nil {
		t.Error("ExistingRequest should be transformed in place")
	}

	// MissingRequest: should be generated with versioned name
	if v1Spec.Components.Schemas["MissingRequestV10"] == nil {
		t.Error("MissingRequest should be generated with versioned name")
	}
}

// Test 4: No package prefix (identity mapper)
func TestSchemaGenerator_NoPackagePrefix(t *testing.T) {
	baseSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"UpdateExampleRequest": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
					},
				}), // No prefix
			},
		},
	}

	v1, _ := epoch.NewSemverVersion("1.0")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	registry := epoch.NewEndpointRegistry()
	registry.Register("POST", "/examples", &epoch.EndpointDefinition{
		Method:      "POST",
		PathPattern: "/examples",
		RequestType: reflect.TypeOf(UpdateExampleRequest{}),
	})

	config := SchemaGeneratorConfig{
		VersionBundle: vb,
		TypeRegistry:  registry,
		// SchemaNameMapper is nil (defaults to identity)
	}

	generator := NewSchemaGenerator(config)
	v1Spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
	if err != nil {
		t.Fatalf("Failed to generate v1 spec: %v", err)
	}

	// Should transform with same name
	if v1Spec.Components.Schemas["UpdateExampleRequest"] == nil {
		t.Error("UpdateExampleRequest should be transformed with same name")
	}
}

// Test 5: Preserves descriptions and metadata from base spec
func TestSchemaGenerator_PreservesMetadata(t *testing.T) {
	baseSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"versionedapi.TestUserResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:        &openapi3.Types{"object"},
					Description: "User response from Swag",
					Properties: map[string]*openapi3.SchemaRef{
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User name from Swag",
						}),
						"id": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"integer"},
							Description: "User ID from Swag",
						}),
					},
				}),
			},
		},
	}

	v1, _ := epoch.NewSemverVersion("1.0")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	registry := epoch.NewEndpointRegistry()
	registry.Register("GET", "/users", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users",
		ResponseType: reflect.TypeOf(TestUserResponse{}),
	})

	config := SchemaGeneratorConfig{
		VersionBundle: vb,
		TypeRegistry:  registry,
		SchemaNameMapper: func(name string) string {
			return "versionedapi." + name
		},
	}

	generator := NewSchemaGenerator(config)
	v1Spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
	if err != nil {
		t.Fatalf("Failed to generate v1 spec: %v", err)
	}

	schema := v1Spec.Components.Schemas["versionedapi.TestUserResponse"]
	if schema == nil {
		t.Fatal("Schema not found")
	}
	if schema.Value.Description != "User response from Swag" {
		t.Errorf("Expected description 'User response from Swag', got '%s'", schema.Value.Description)
	}
	if schema.Value.Properties["name"] != nil && schema.Value.Properties["name"].Value.Description != "User name from Swag" {
		t.Error("Field description not preserved")
	}
}
