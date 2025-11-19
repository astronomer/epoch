package openapi

import (
	"reflect"
	"testing"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
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

// createTestVersions creates the standard v1, v2, v3 versions used across tests
func createTestVersions() (*epoch.Version, *epoch.Version, *epoch.Version) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")
	v3, _ := epoch.NewDateVersion("2025-01-01")
	return v1, v2, v3
}

// createTestMigrations creates the standard v1→v2 and v2→v3 migrations
func createTestMigrations(v1, v2, v3 *epoch.Version) (*epoch.VersionChange, *epoch.VersionChange) {
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

	return v1ToV2, v2ToV3
}

// swagSchemaNameMapper returns the standard SchemaNameMapper for Swag integration
func swagSchemaNameMapper() func(string) string {
	return func(typeName string) string {
		return "versionedapi." + typeName
	}
}

// createUserResponseSchema creates a full UserResponse schema with all fields
func createUserResponseSchema(withDescriptions bool) *openapi3.SchemaRef {
	properties := map[string]*openapi3.SchemaRef{
		"id": openapi3.NewSchemaRef("", &openapi3.Schema{
			Type: &openapi3.Types{"integer"},
		}),
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
		"created_at": openapi3.NewSchemaRef("", &openapi3.Schema{
			Type:   &openapi3.Types{"string"},
			Format: "date-time",
		}),
	}

	if withDescriptions {
		properties["id"].Value.Description = "User ID from Swag"
		properties["full_name"].Value.Description = "User full name from Swag"
		properties["email"].Value.Description = "User email from Swag"
		properties["phone"].Value.Description = "User phone from Swag"
		properties["status"].Value.Description = "User status from Swag"
		properties["created_at"].Value.Description = "Creation timestamp from Swag"
	}

	schema := &openapi3.Schema{
		Type:       &openapi3.Types{"object"},
		Properties: properties,
	}

	if withDescriptions {
		schema.Description = "User response (from Swag)"
	}

	return openapi3.NewSchemaRef("", schema)
}

// createCreateUserRequestSchema creates a CreateUserRequest schema
func createCreateUserRequestSchema() *openapi3.SchemaRef {
	return openapi3.NewSchemaRef("", &openapi3.Schema{
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
	})
}

// createOrganizationResponseSchema creates an OrganizationResponse schema
func createOrganizationResponseSchema() *openapi3.SchemaRef {
	return openapi3.NewSchemaRef("", &openapi3.Schema{
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
	})
}

// verifySchemaFields checks that a schema has expected fields present and unexpected fields absent
func verifySchemaFields(t *testing.T, schema *openapi3.SchemaRef, versionName string, expectedFields, unexpectedFields []string) {
	t.Helper()

	if schema == nil {
		t.Fatalf("%s: schema is nil", versionName)
	}

	if schema.Value == nil {
		t.Fatalf("%s: schema value is nil", versionName)
	}

	for _, field := range expectedFields {
		if schema.Value.Properties[field] == nil {
			t.Errorf("%s: expected field %s to be present", versionName, field)
		}
	}

	for _, field := range unexpectedFields {
		if schema.Value.Properties[field] != nil {
			t.Errorf("%s: should not have field %s", versionName, field)
		}
	}
}

// getSchemaOrFail retrieves a schema from a spec or fails the test
func getSchemaOrFail(t *testing.T, spec *openapi3.T, schemaName, versionName string) *openapi3.SchemaRef {
	t.Helper()

	schema := spec.Components.Schemas[schemaName]
	if schema == nil {
		t.Fatalf("%s: schema '%s' not found", versionName, schemaName)
	}

	return schema
}

// TestSwagIntegration_Debug prints out what each version actually contains
func TestSwagIntegration_Debug(t *testing.T) {
	// Setup versions and migrations using helpers
	v1, v2, v3 := createTestVersions()
	v1ToV2, v2ToV3 := createTestMigrations(v1, v2, v3)

	epochInstance, err := epoch.NewEpoch().
		WithHeadVersion().
		WithVersions(v1, v2, v3).
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
				"versionedapi.swagTestUserResponse": createUserResponseSchema(false),
			},
		},
	}

	generator := NewSchemaGenerator(SchemaGeneratorConfig{
		VersionBundle:    epochInstance.VersionBundle(),
		TypeRegistry:     epochInstance.EndpointRegistry(),
		OutputFormat:     "yaml",
		SchemaNameMapper: swagSchemaNameMapper(),
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
	// Setup versions and migrations using helpers
	v1, v2, v3 := createTestVersions()
	v1ToV2, v2ToV3 := createTestMigrations(v1, v2, v3)

	epochInstance, err := epoch.NewEpoch().
		WithHeadVersion().
		WithVersions(v1, v2, v3).
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
				"versionedapi.swagTestUserResponse":         createUserResponseSchema(true),
				"versionedapi.swagTestCreateUserRequest":    createCreateUserRequestSchema(),
				"versionedapi.swagTestOrganizationResponse": createOrganizationResponseSchema(),
			},
		},
	}

	// Configure generator with SchemaNameMapper for Swag
	generator := NewSchemaGenerator(SchemaGeneratorConfig{
		VersionBundle:    epochInstance.VersionBundle(),
		TypeRegistry:     epochInstance.EndpointRegistry(),
		OutputFormat:     "yaml",
		SchemaNameMapper: swagSchemaNameMapper(),
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

		schema := getSchemaOrFail(t, headSpec, "versionedapi.swagTestUserResponse", "HEAD")
		verifySchemaFields(t, schema, "HEAD",
			[]string{"id", "full_name", "email", "phone", "status", "created_at"},
			nil)
	})

	// Test 2: v3 (2025-01-01) is latest versioned version, equivalent to HEAD
	t.Run("v3_equivalent_to_HEAD", func(t *testing.T) {
		v3Spec, ok := versionedSpecs["2025-01-01"]
		if !ok {
			t.Fatal("v3 spec not found")
		}

		schema := getSchemaOrFail(t, v3Spec, "versionedapi.swagTestUserResponse", "v3")
		verifySchemaFields(t, schema, "v3",
			[]string{"id", "full_name", "email", "phone", "status", "created_at"},
			nil)
	})

	// Test 3: v2 (2024-06-01) should have v2ToV3 transformations applied
	t.Run("v2_has_v2ToV3_transformations", func(t *testing.T) {
		v2Spec, ok := versionedSpecs["2024-06-01"]
		if !ok {
			t.Fatal("v2 spec not found")
		}

		schema := getSchemaOrFail(t, v2Spec, "versionedapi.swagTestUserResponse", "v2")
		// v2 should have: id, name (renamed from full_name), email, status, created_at
		// v2 should NOT have: phone (removed by v2ToV3), full_name (renamed to name)
		verifySchemaFields(t, schema, "v2",
			[]string{"id", "name", "email", "status", "created_at"},
			[]string{"phone", "full_name"})
	})

	// Test 4: v1 (2024-01-01) should have both v1ToV2 and v2ToV3 transformations
	t.Run("v1_has_cumulative_transformations", func(t *testing.T) {
		v1Spec, ok := versionedSpecs["2024-01-01"]
		if !ok {
			t.Fatal("v1 spec not found")
		}

		schema := getSchemaOrFail(t, v1Spec, "versionedapi.swagTestUserResponse", "v1")
		// v1 should have: id, name (from v2ToV3), created_at
		// v1 should NOT have: email, status (removed by v1ToV2), phone, full_name
		verifySchemaFields(t, schema, "v1",
			[]string{"id", "name", "created_at"},
			[]string{"email", "status", "phone", "full_name"})
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
		VersionBundle:    epochInstance.VersionBundle(),
		TypeRegistry:     epochInstance.EndpointRegistry(),
		OutputFormat:     "yaml",
		SchemaNameMapper: swagSchemaNameMapper(),
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

// dummyHandler is a no-op handler for registration testing
func dummyHandler(c *gin.Context) {
	// No-op handler - only used for type registration via WrapHandler
}

// TestSwagIntegration_ProductionPattern replicates the production setup pattern
// where handlers are registered via WrapHandler().ToHandlerFunc() on a gin router,
// rather than manually calling EndpointRegistry.Register()
func TestSwagIntegration_ProductionPattern(t *testing.T) {
	// Setup versions and migrations using helpers
	v1, v2, v3 := createTestVersions()
	v1ToV2, v2ToV3 := createTestMigrations(v1, v2, v3)

	epochInstance, err := epoch.NewEpoch().
		WithHeadVersion().
		WithVersions(v1, v2, v3).
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

	// Create dummy Gin router (like spec_converter.go)
	dummyRouter := gin.New()
	routerGroup := dummyRouter.Group("")

	// Register handlers via WrapHandler pattern (NOT manual Register)
	// This is the key difference - we let ToHandlerFunc do the registration
	routerGroup.GET("/users/:id",
		epochInstance.WrapHandler(dummyHandler).
			Returns(swagTestUserResponse{}).
			ToHandlerFunc("GET", "/users/:id"))

	routerGroup.POST("/users",
		epochInstance.WrapHandler(dummyHandler).
			Accepts(swagTestCreateUserRequest{}).
			Returns(swagTestUserResponse{}).
			ToHandlerFunc("POST", "/users"))

	routerGroup.GET("/organizations/:id",
		epochInstance.WrapHandler(dummyHandler).
			Returns(swagTestOrganizationResponse{}).
			ToHandlerFunc("GET", "/organizations/:id"))

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
				"versionedapi.swagTestUserResponse":         createUserResponseSchema(true),
				"versionedapi.swagTestCreateUserRequest":    createCreateUserRequestSchema(),
				"versionedapi.swagTestOrganizationResponse": createOrganizationResponseSchema(),
			},
		},
	}

	// Configure generator with SchemaNameMapper for Swag
	generator := NewSchemaGenerator(SchemaGeneratorConfig{
		VersionBundle:    epochInstance.VersionBundle(),
		TypeRegistry:     epochInstance.EndpointRegistry(),
		OutputFormat:     "yaml",
		SchemaNameMapper: swagSchemaNameMapper(),
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

		schema := getSchemaOrFail(t, headSpec, "versionedapi.swagTestUserResponse", "HEAD")
		verifySchemaFields(t, schema, "HEAD",
			[]string{"id", "full_name", "email", "phone", "status", "created_at"},
			nil)
	})

	// Test 2: v3 (2025-01-01) is latest versioned version, equivalent to HEAD
	t.Run("v3_equivalent_to_HEAD", func(t *testing.T) {
		v3Spec, ok := versionedSpecs["2025-01-01"]
		if !ok {
			t.Fatal("v3 spec not found")
		}

		schema := getSchemaOrFail(t, v3Spec, "versionedapi.swagTestUserResponse", "v3")
		verifySchemaFields(t, schema, "v3",
			[]string{"id", "full_name", "email", "phone", "status", "created_at"},
			nil)
	})

	// Test 3: v2 (2024-06-01) should have v2ToV3 transformations applied
	t.Run("v2_has_v2ToV3_transformations", func(t *testing.T) {
		v2Spec, ok := versionedSpecs["2024-06-01"]
		if !ok {
			t.Fatal("v2 spec not found")
		}

		schema := getSchemaOrFail(t, v2Spec, "versionedapi.swagTestUserResponse", "v2")
		// v2 should have: id, name (renamed from full_name), email, status, created_at
		// v2 should NOT have: phone (removed by v2ToV3), full_name (renamed to name)
		verifySchemaFields(t, schema, "v2",
			[]string{"id", "name", "email", "status", "created_at"},
			[]string{"phone", "full_name"})
	})

	// Test 4: v1 (2024-01-01) should have both v1ToV2 and v2ToV3 transformations
	t.Run("v1_has_cumulative_transformations", func(t *testing.T) {
		v1Spec, ok := versionedSpecs["2024-01-01"]
		if !ok {
			t.Fatal("v1 spec not found")
		}

		schema := getSchemaOrFail(t, v1Spec, "versionedapi.swagTestUserResponse", "v1")
		// v1 should have: id, name (from v2ToV3), created_at
		// v1 should NOT have: email, status (removed by v1ToV2), phone, full_name
		verifySchemaFields(t, schema, "v1",
			[]string{"id", "name", "created_at"},
			[]string{"email", "status", "phone", "full_name"})
	})

	// Test 5: Verify endpoint registry was populated via ToHandlerFunc
	t.Run("endpoint_registry_populated_by_ToHandlerFunc", func(t *testing.T) {
		registry := epochInstance.EndpointRegistry()

		// Check that endpoints were registered
		getUserDef, err := registry.Lookup("GET", "/users/:id")
		if err != nil {
			t.Errorf("GET /users/:id endpoint not registered: %v", err)
		} else {
			if getUserDef.ResponseType != reflect.TypeOf(swagTestUserResponse{}) {
				t.Error("GET /users/:id response type mismatch")
			}
		}

		createUserDef, err := registry.Lookup("POST", "/users")
		if err != nil {
			t.Errorf("POST /users endpoint not registered: %v", err)
		} else {
			if createUserDef.RequestType != reflect.TypeOf(swagTestCreateUserRequest{}) {
				t.Error("POST /users request type mismatch")
			}
			if createUserDef.ResponseType != reflect.TypeOf(swagTestUserResponse{}) {
				t.Error("POST /users response type mismatch")
			}
		}

		getOrgDef, err := registry.Lookup("GET", "/organizations/:id")
		if err != nil {
			t.Errorf("GET /organizations/:id endpoint not registered: %v", err)
		} else {
			if getOrgDef.ResponseType != reflect.TypeOf(swagTestOrganizationResponse{}) {
				t.Error("GET /organizations/:id response type mismatch")
			}
		}
	})
}
