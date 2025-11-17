package openapi

import (
	"reflect"
	"testing"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
)

// Test types for Swag integration tests
type swagTestCreateUserRequest struct {
	FullName string `json:"full_name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone,omitempty"`
	Status   string `json:"status" binding:"required,oneof=active inactive pending suspended"`
}

type swagTestUserResponse struct {
	ID        int        `json:"id,omitempty" validate:"required"`
	FullName  string     `json:"full_name" validate:"required"`
	Email     string     `json:"email,omitempty" validate:"required,email"`
	Phone     string     `json:"phone,omitempty"`
	Status    string     `json:"status,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty" format:"date-time"`
}

type swagTestOrganizationResponse struct {
	ID        string            `json:"id" validate:"required" example:"clmaxoarx000008l2c5ayb9pt"`
	Name      string            `json:"name" validate:"required" example:"My organization"`
	Product   string            `json:"product,omitempty" enums:"HOSTED,HYBRID" example:"HOSTED"`
	Status    string            `json:"status,omitempty" enums:"ACTIVE,INACTIVE,SUSPENDED" example:"ACTIVE"`
	CreatedAt *time.Time        `json:"createdAt" validate:"required" format:"date-time"`
	UpdatedAt *time.Time        `json:"updatedAt" validate:"required" format:"date-time"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// TestSwagIntegration_Debug prints out what each version actually contains
func TestSwagIntegration_Debug(t *testing.T) {
	// Setup versions
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")
	v3, _ := epoch.NewDateVersion("2025-01-01")

	// Define version migrations
	v1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email and status fields to User").
		ForType(swagTestUserResponse{}, swagTestCreateUserRequest{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		RemoveField("status").
		Build()

	v2ToV3 := epoch.NewVersionChangeBuilder(v2, v3).
		Description("Rename name to full_name, add phone").
		ForType(swagTestUserResponse{}, swagTestCreateUserRequest{}).
		ResponseToPreviousVersion().
		RenameField("full_name", "name").
		RemoveField("phone").
		Build()

	epochInstance, err := epoch.NewEpoch().
		WithHeadVersion().
		WithVersions(v1, v2, v3). // Versions in ascending order (oldest first)
		WithChanges(v1ToV2, v2ToV3).
		WithTypes(swagTestUserResponse{}).
		Build()

	if err != nil {
		t.Fatalf("Failed to create Epoch instance: %v", err)
	}

	epochInstance.EndpointRegistry().Register("GET", "/users/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users/:id",
		ResponseType: reflect.TypeOf(swagTestUserResponse{}),
	})

	swagSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info:    &openapi3.Info{Title: "Test API", Version: "1.0.0"},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"versionedapi.swagTestUserResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":         openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"full_name":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"email":      openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"phone":      openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"status":     openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"created_at": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}, Format: "date-time"}),
					},
				}),
			},
		},
	}

	generator := NewSchemaGenerator(SchemaGeneratorConfig{
		VersionBundle:    epochInstance.VersionBundle(),
		TypeRegistry:     epochInstance.EndpointRegistry(),
		OutputFormat:     "yaml",
		SchemaNameMapper: func(typeName string) string { return "versionedapi." + typeName },
	})

	versionedSpecs, err := generator.GenerateVersionedSpecs(swagSpec)
	if err != nil {
		t.Fatalf("Failed to generate versioned specs: %v", err)
	}

	// Print out what each version has in sorted order
	versionOrder := []string{"head", "2025-01-01", "2024-06-01", "2024-01-01"}
	for _, versionStr := range versionOrder {
		spec, ok := versionedSpecs[versionStr]
		if !ok {
			t.Logf("%s: not found", versionStr)
			continue
		}
		schema := spec.Components.Schemas["versionedapi.swagTestUserResponse"]
		if schema != nil && schema.Value != nil {
			var fields []string
			for fieldName := range schema.Value.Properties {
				fields = append(fields, fieldName)
			}
			t.Logf("%s: %v", versionStr, fields)
		} else {
			t.Logf("%s: schema not found or nil", versionStr)
		}
	}

	// Also check what versions are in the bundle
	t.Logf("Versions in bundle:")
	for _, v := range epochInstance.VersionBundle().GetVersions() {
		t.Logf("  - %s (changes: %d)", v.String(), len(v.Changes))
		for _, c := range v.Changes {
			if vc, ok := c.(*epoch.VersionChange); ok {
				t.Logf("    Change: %s -> %s", vc.FromVersion().String(), vc.ToVersion().String())
			}
		}
	}
}

// TestSwagIntegration_TransformInPlace tests that existing Swag-generated schemas
// are transformed in place, preserving their original names
func TestSwagIntegration_TransformInPlace(t *testing.T) {
	// Setup versions
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")
	v3, _ := epoch.NewDateVersion("2025-01-01")

	// Define version migrations (same as main.go)
	v1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email and status fields to User").
		ForType(swagTestUserResponse{}, swagTestCreateUserRequest{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		RemoveField("status").
		Build()

	v2ToV3 := epoch.NewVersionChangeBuilder(v2, v3).
		Description("Rename name to full_name, add phone").
		ForType(swagTestUserResponse{}, swagTestCreateUserRequest{}).
		ResponseToPreviousVersion().
		RenameField("full_name", "name").
		RemoveField("phone").
		Build()

	// Create Epoch instance
	epochInstance, err := epoch.NewEpoch().
		WithHeadVersion().
		WithVersions(v1, v2, v3). // Versions in ascending order (oldest first)
		WithChanges(v1ToV2, v2ToV3).
		WithTypes(
			swagTestCreateUserRequest{},
			swagTestUserResponse{},
			swagTestOrganizationResponse{},
		).
		Build()

	if err != nil {
		t.Fatalf("Failed to create Epoch instance: %v", err)
	}

	// Register types
	epochInstance.EndpointRegistry().Register("GET", "/users/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users/:id",
		ResponseType: reflect.TypeOf(swagTestUserResponse{}),
	})
	epochInstance.EndpointRegistry().Register("POST", "/users", &epoch.EndpointDefinition{
		Method:       "POST",
		PathPattern:  "/users",
		RequestType:  reflect.TypeOf(swagTestCreateUserRequest{}),
		ResponseType: reflect.TypeOf(swagTestUserResponse{}),
	})
	epochInstance.EndpointRegistry().Register("GET", "/organizations/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/organizations/:id",
		ResponseType: reflect.TypeOf(swagTestOrganizationResponse{}),
	})

	// Create Swag-style base spec with package-prefixed schemas
	swagSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Example API with Swag",
			Version:     "1.0.0",
			Description: "Generated by Swag",
		},
		Paths: &openapi3.Paths{},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				// Swag generates schemas with package prefix
				"versionedapi.swagTestUserResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:        &openapi3.Types{"object"},
					Description: "User response (from Swag)",
					Properties: map[string]*openapi3.SchemaRef{
						"id": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"integer"},
							Description: "User ID from Swag",
						}),
						"full_name": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User full name from Swag",
						}),
						"email": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User email from Swag",
						}),
						"phone": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User phone from Swag",
						}),
						"status": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User status from Swag",
						}),
						"created_at": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Format:      "date-time",
							Description: "Creation timestamp from Swag",
						}),
					},
				}),
				"versionedapi.swagTestCreateUserRequest": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:        &openapi3.Types{"object"},
					Description: "Create user request (from Swag)",
					Properties: map[string]*openapi3.SchemaRef{
						"full_name": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
						"email": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
						"phone": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
						"status": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
					},
				}),
				"versionedapi.swagTestOrganizationResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:        &openapi3.Types{"object"},
					Description: "Organization response (from Swag)",
					Properties: map[string]*openapi3.SchemaRef{
						"id": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
						"product": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"string"},
						}),
					},
				}),
			},
		},
	}

	// Configure generator with SchemaNameMapper for Swag
	generator := NewSchemaGenerator(SchemaGeneratorConfig{
		VersionBundle: epochInstance.VersionBundle(),
		TypeRegistry:  epochInstance.EndpointRegistry(),
		OutputFormat:  "yaml",
		SchemaNameMapper: func(typeName string) string {
			// Map Go type names to Swag's package.TypeName format
			return "versionedapi." + typeName
		},
	})

	// Generate versioned specs
	versionedSpecs, err := generator.GenerateVersionedSpecs(swagSpec)
	if err != nil {
		t.Fatalf("Failed to generate versioned specs: %v", err)
	}

	// Verify we got all 4 versions (v1, v2, v3, head)
	if len(versionedSpecs) != 4 {
		t.Errorf("Expected 4 versioned specs, got %d", len(versionedSpecs))
	}

	// Test 1: HEAD version should keep original schema names and all fields
	t.Run("HEAD_preserves_original_name_and_fields", func(t *testing.T) {
		headSpec, ok := versionedSpecs["head"]
		if !ok {
			t.Fatal("HEAD spec not found")
		}

		// Schema should exist with Swag name (transformed in place)
		schema := headSpec.Components.Schemas["versionedapi.swagTestUserResponse"]
		if schema == nil {
			t.Fatal("versionedapi.swagTestUserResponse schema not found in HEAD")
		}

		if schema.Value == nil {
			t.Fatal("Schema value is nil")
		}

		// Verify all fields are present
		expectedFields := []string{"id", "full_name", "email", "phone", "status", "created_at"}
		for _, field := range expectedFields {
			if schema.Value.Properties[field] == nil {
				t.Errorf("HEAD: expected field %s to be present", field)
			}
		}
	})

	// Test 2: v3 (2025-01-01) is latest versioned version, equivalent to HEAD
	// Since no migration is defined from v3 to HEAD, v3 should have all fields
	t.Run("v3_equivalent_to_HEAD", func(t *testing.T) {
		v3Spec, ok := versionedSpecs["2025-01-01"]
		if !ok {
			t.Fatal("v3 spec not found")
		}

		// Schema should be transformed in place (same name)
		schema := v3Spec.Components.Schemas["versionedapi.swagTestUserResponse"]
		if schema == nil {
			t.Fatal("versionedapi.swagTestUserResponse schema not found in v3")
		}

		if schema.Value == nil {
			t.Fatal("Schema value is nil")
		}

		// v3 is the latest versioned version with no changes to HEAD
		// So v3 should have all fields just like HEAD
		expectedFields := []string{"id", "full_name", "email", "phone", "status", "created_at"}
		for _, field := range expectedFields {
			if schema.Value.Properties[field] == nil {
				t.Errorf("v3: expected field %s (should be same as HEAD)", field)
			}
		}
	})

	// Test 3: v2 (2024-06-01) should have v2ToV3 transformations applied
	t.Run("v2_has_v2ToV3_transformations", func(t *testing.T) {
		v2Spec, ok := versionedSpecs["2024-06-01"]
		if !ok {
			t.Fatal("v2 spec not found")
		}

		schema := v2Spec.Components.Schemas["versionedapi.swagTestUserResponse"]
		if schema == nil {
			t.Fatal("versionedapi.swagTestUserResponse schema not found in v2")
		}

		if schema.Value == nil {
			t.Fatal("Schema value is nil")
		}

		// v2 should have: id, name (renamed from full_name by v2ToV3), email, status, created_at
		// v2 should NOT have: phone (removed by v2ToV3), full_name (renamed to name)
		if schema.Value.Properties["name"] == nil {
			t.Error("v2: expected 'name' field (renamed from full_name)")
		}
		if schema.Value.Properties["full_name"] != nil {
			t.Error("v2: should not have 'full_name' field (renamed to name)")
		}
		if schema.Value.Properties["email"] == nil {
			t.Error("v2: expected 'email' field")
		}
		if schema.Value.Properties["status"] == nil {
			t.Error("v2: expected 'status' field")
		}
		if schema.Value.Properties["phone"] != nil {
			t.Error("v2: should not have 'phone' field (removed by v2ToV3)")
		}
	})

	// Test 4: v1 (2024-01-01) should have both v1ToV2 and v2ToV3 transformations
	t.Run("v1_has_cumulative_transformations", func(t *testing.T) {
		v1Spec, ok := versionedSpecs["2024-01-01"]
		if !ok {
			t.Fatal("v1 spec not found")
		}

		schema := v1Spec.Components.Schemas["versionedapi.swagTestUserResponse"]
		if schema == nil {
			t.Fatal("versionedapi.swagTestUserResponse schema not found in v1")
		}

		if schema.Value == nil {
			t.Fatal("Schema value is nil")
		}

		// v1 should have: id, name (from v2ToV3), created_at
		// v1 should NOT have: email (removed by v1ToV2), status (removed by v1ToV2),
		//                     phone (removed by v2ToV3), full_name (renamed to name by v2ToV3)
		expectedFields := []string{"id", "name", "created_at"}
		for _, field := range expectedFields {
			if schema.Value.Properties[field] == nil {
				t.Errorf("v1: expected field %s to be present", field)
			}
		}

		unexpectedFields := []string{"email", "status", "phone", "full_name"}
		for _, field := range unexpectedFields {
			if schema.Value.Properties[field] != nil {
				t.Errorf("v1: should not have field %s", field)
			}
		}
	})

	// Test 5: Verify schema names are preserved (not versioned)
	t.Run("schema_names_preserved_from_swag", func(t *testing.T) {
		for versionStr, spec := range versionedSpecs {
			// All versions should have the Swag-prefixed name
			if spec.Components.Schemas["versionedapi.swagTestUserResponse"] == nil {
				t.Errorf("%s: expected schema 'versionedapi.swagTestUserResponse' to exist", versionStr)
			}

			// Should NOT have versioned names like UserResponseV20240101
			for name := range spec.Components.Schemas {
				if name != "versionedapi.swagTestUserResponse" &&
					name != "versionedapi.swagTestCreateUserRequest" &&
					name != "versionedapi.swagTestOrganizationResponse" {
					t.Errorf("%s: unexpected schema name %s (should preserve Swag names)", versionStr, name)
				}
			}
		}
	})
}

// TestSwagIntegration_PreservesMetadata verifies that descriptions and metadata
// from Swag-generated specs are preserved through transformations
func TestSwagIntegration_PreservesMetadata(t *testing.T) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	headVersion := epoch.NewHeadVersion()

	// Simple migration that removes one field
	change := epoch.NewVersionChangeBuilder(v1, headVersion).
		Description("Add email field").
		ForType(swagTestUserResponse{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		Build()

	epochInstance, err := epoch.NewEpoch().
		WithHeadVersion().
		WithVersions(v1). // Only one version, so order doesn't matter
		WithChanges(change).
		WithTypes(swagTestUserResponse{}).
		Build()

	if err != nil {
		t.Fatalf("Failed to create Epoch instance: %v", err)
	}

	epochInstance.EndpointRegistry().Register("GET", "/users/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users/:id",
		ResponseType: reflect.TypeOf(swagTestUserResponse{}),
	})

	// Create Swag spec with rich metadata
	swagSpec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Test API",
			Version:     "1.0.0",
			Description: "API with metadata",
		},
		Components: &openapi3.Components{
			Schemas: openapi3.Schemas{
				"versionedapi.swagTestUserResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:        &openapi3.Types{"object"},
					Description: "User response with rich metadata from Swag",
					Properties: map[string]*openapi3.SchemaRef{
						"id": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"integer"},
							Description: "Unique user identifier from Swag",
							Example:     float64(42),
						}),
						"full_name": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User's full name from Swag",
							Example:     "John Doe",
						}),
						"email": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type:        &openapi3.Types{"string"},
							Description: "User's email address from Swag",
							Format:      "email",
							Example:     "john@example.com",
						}),
					},
				}),
			},
		},
	}

	generator := NewSchemaGenerator(SchemaGeneratorConfig{
		VersionBundle: epochInstance.VersionBundle(),
		TypeRegistry:  epochInstance.EndpointRegistry(),
		OutputFormat:  "yaml",
		SchemaNameMapper: func(typeName string) string {
			return "versionedapi." + typeName
		},
	})

	versionedSpecs, err := generator.GenerateVersionedSpecs(swagSpec)
	if err != nil {
		t.Fatalf("Failed to generate versioned specs: %v", err)
	}

	// Test HEAD: all metadata preserved
	t.Run("HEAD_preserves_all_metadata", func(t *testing.T) {
		headSpec := versionedSpecs["head"]
		schema := headSpec.Components.Schemas["versionedapi.swagTestUserResponse"]

		if schema.Value.Description != "User response with rich metadata from Swag" {
			t.Errorf("HEAD: schema description not preserved, got: %s", schema.Value.Description)
		}

		idSchema := schema.Value.Properties["id"]
		if idSchema.Value.Description != "Unique user identifier from Swag" {
			t.Error("HEAD: field descriptions not preserved")
		}

		if idSchema.Value.Example != float64(42) {
			t.Error("HEAD: field examples not preserved")
		}
	})

	// Test v1: metadata preserved even after field removal
	t.Run("v1_preserves_metadata_after_transformation", func(t *testing.T) {
		v1Spec := versionedSpecs["2024-01-01"]
		schema := v1Spec.Components.Schemas["versionedapi.swagTestUserResponse"]

		// Schema description should be preserved
		if schema.Value.Description != "User response with rich metadata from Swag" {
			t.Errorf("v1: schema description not preserved, got: %s", schema.Value.Description)
		}

		// Remaining fields should keep their metadata
		idSchema := schema.Value.Properties["id"]
		if idSchema.Value.Description != "Unique user identifier from Swag" {
			t.Error("v1: field descriptions not preserved after transformation")
		}

		if idSchema.Value.Example != float64(42) {
			t.Error("v1: field examples not preserved after transformation")
		}

		// email field should be removed
		if schema.Value.Properties["email"] != nil {
			t.Error("v1: email field should be removed")
		}
	})
}
