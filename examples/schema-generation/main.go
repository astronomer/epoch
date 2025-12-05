package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/astronomer/epoch/epoch/openapi"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// NESTED TYPE DEFINITIONS - Response Types (HEAD version)
// ============================================================================

// Skill represents an item in the profile.skills[] array (nested array inside nested object)
// v1: skill_name, no level
// v2+: name, level
type Skill struct {
	Name  string `json:"name"`  // v1: "skill_name"
	Level int    `json:"level"` // Added in v2
}

// ProfileSettings represents deeply nested settings (3 levels: user.profile.settings)
// v1: color_theme
// v2+: theme
type ProfileSettings struct {
	Theme string `json:"theme"` // v1: "color_theme"
}

// UserProfile represents a nested object containing an array and another nested object
// v1: biography
// v2+: bio
type UserProfile struct {
	Bio      string          `json:"bio"`      // v1: "biography"
	Skills   []Skill         `json:"skills"`   // Array inside nested object
	Settings ProfileSettings `json:"settings"` // 3-level deep nesting
}

// ============================================================================
// NESTED TYPE DEFINITIONS - Request Types (HEAD version)
// ============================================================================

// SkillRequest represents a skill in the request (HEAD version)
// v1: skill_name, no level
// v2+: name, level
type SkillRequest struct {
	Name  string `json:"name"`            // v1: "skill_name"
	Level int    `json:"level,omitempty"` // Added in v2
}

// ProfileSettingsRequest represents deeply nested settings in request (HEAD version)
// v1: color_theme
// v2+: theme
type ProfileSettingsRequest struct {
	Theme string `json:"theme,omitempty"` // v1: "color_theme"
}

// ProfileRequest represents a nested object in requests (HEAD version)
// v1: biography, skills[].skill_name
// v2+: bio, skills[].name + level
type ProfileRequest struct {
	Bio      string                  `json:"bio,omitempty"`      // v1: "biography"
	Skills   []SkillRequest          `json:"skills,omitempty"`   // Nested array in request
	Settings *ProfileSettingsRequest `json:"settings,omitempty"` // Deeply nested object
}

// ============================================================================
// TOP-LEVEL TYPE DEFINITIONS
// ============================================================================

type CreateUserRequest struct {
	FullName string          `json:"full_name" binding:"required,max=100"`
	Email    string          `json:"email" binding:"required,email"`
	Phone    string          `json:"phone,omitempty"`
	Status   string          `json:"status" binding:"required,oneof=active inactive pending suspended"`
	Profile  *ProfileRequest `json:"profile,omitempty"` // Nested object with array and deeply nested settings
}

type UserResponse struct {
	ID        int          `json:"id,omitempty" validate:"required"`
	FullName  string       `json:"full_name" validate:"required"`
	Email     string       `json:"email,omitempty" validate:"required,email"`
	Phone     string       `json:"phone,omitempty"`
	Status    string       `json:"status,omitempty"`
	CreatedAt *time.Time   `json:"created_at,omitempty" format:"date-time"`
	Profile   *UserProfile `json:"profile,omitempty"` // Nested object with array and deeply nested settings
}

type OrganizationResponse struct {
	ID        string            `json:"id" validate:"required" example:"clmaxoarx000008l2c5ayb9pt"`
	Name      string            `json:"name" validate:"required" example:"My organization"`
	Product   string            `json:"product,omitempty" enums:"HOSTED,HYBRID" example:"HOSTED"`
	Status    string            `json:"status,omitempty" enums:"ACTIVE,INACTIVE,SUSPENDED" example:"ACTIVE"`
	CreatedAt *time.Time        `json:"createdAt" validate:"required" format:"date-time"`
	UpdatedAt *time.Time        `json:"updatedAt" validate:"required" format:"date-time"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

// Dummy handler functions for Gin router registration
func getUser(c *gin.Context) {
	c.JSON(200, UserResponse{})
}

func createUser(c *gin.Context) {
	c.JSON(201, UserResponse{})
}

func getOrganization(c *gin.Context) {
	c.JSON(200, OrganizationResponse{})
}

// Helper function to create a minimal base OpenAPI spec
func createBaseSpec() *openapi3.T {
	return &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Example API",
			Version:     "1.0.0",
			Description: "A minimal base specification",
		},
		Paths:      &openapi3.Paths{},
		Components: &openapi3.Components{},
	}
}

// Helper function to create a rich existing OpenAPI spec (simulating swag output)
func createExistingSpec() *openapi3.T {
	spec := &openapi3.T{
		OpenAPI: "3.0.3",
		Info: &openapi3.Info{
			Title:       "Example API",
			Version:     "1.0.0",
			Description: "A rich specification with paths, security, and schemas",
		},
		Paths: &openapi3.Paths{},
		Components: &openapi3.Components{
			SecuritySchemes: openapi3.SecuritySchemes{
				"BearerAuth": &openapi3.SecuritySchemeRef{
					Value: openapi3.NewJWTSecurityScheme(),
				},
			},
			Schemas: openapi3.Schemas{},
		},
		Tags: openapi3.Tags{
			{
				Name:        "users",
				Description: "User management endpoints",
			},
			{
				Name:        "organizations",
				Description: "Organization management endpoints",
			},
		},
	}

	// Add BearerAuth description
	spec.Components.SecuritySchemes["BearerAuth"].Value.Description = "JWT Bearer token authentication"

	// Add paths with descriptions
	spec.Paths.Set("/users", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Summary:     "List users",
			Description: "Retrieve a list of all users",
			Tags:        []string{"users"},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{
					Value: openapi3.NewResponse().WithDescription("Successful response"),
				}),
			),
		},
		Post: &openapi3.Operation{
			Summary:     "Create user",
			Description: "Create a new user",
			Tags:        []string{"users"},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(201, &openapi3.ResponseRef{
					Value: openapi3.NewResponse().WithDescription("User created"),
				}),
			),
		},
	})

	spec.Paths.Set("/users/{id}", &openapi3.PathItem{
		Get: &openapi3.Operation{
			Summary:     "Get user",
			Description: "Retrieve a specific user by ID",
			Tags:        []string{"users"},
			Parameters: openapi3.Parameters{
				&openapi3.ParameterRef{
					Value: &openapi3.Parameter{
						Name:        "id",
						In:          "path",
						Description: "User ID",
						Required:    true,
						Schema: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"integer"},
							},
						},
					},
				},
			},
			Responses: openapi3.NewResponses(
				openapi3.WithStatus(200, &openapi3.ResponseRef{
					Value: openapi3.NewResponse().WithDescription("Successful response"),
				}),
			),
		},
	})

	// Add schemas with descriptions (these will be managed by Epoch)
	// Include nested types to demonstrate nested transformation support

	// ProfileSettings schema (deeply nested - 3 levels)
	spec.Components.Schemas["ProfileSettings"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Profile settings object (HEAD: theme, v1: color_theme)",
		Properties: map[string]*openapi3.SchemaRef{
			"theme": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "Color theme setting",
			}),
		},
	})

	// Skill schema (array item inside nested object)
	spec.Components.Schemas["Skill"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Skill object (HEAD: name+level, v1: skill_name only)",
		Properties: map[string]*openapi3.SchemaRef{
			"name": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "Skill name",
			}),
			"level": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"integer"},
				Description: "Skill level (1-5)",
			}),
		},
	})

	// UserProfile schema (nested object containing array and nested object)
	spec.Components.Schemas["UserProfile"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "User profile object (HEAD: bio, v1: biography)",
		Properties: map[string]*openapi3.SchemaRef{
			"bio": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User biography",
			}),
			"skills": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"array"},
				Description: "List of skills",
				Items:       &openapi3.SchemaRef{Ref: "#/components/schemas/Skill"},
			}),
			"settings": &openapi3.SchemaRef{Ref: "#/components/schemas/ProfileSettings"},
		},
	})

	// UserResponse with profile
	spec.Components.Schemas["UserResponse"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "User response object",
		Properties: map[string]*openapi3.SchemaRef{
			"id": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"integer"},
				Description: "User ID from base spec",
			}),
			"full_name": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User full name from base spec",
			}),
			"email": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User email from base spec",
			}),
			"phone": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User phone from base spec",
			}),
			"status": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User status from base spec",
			}),
			"created_at": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Format:      "date-time",
				Description: "User creation timestamp from base spec",
			}),
			"profile": &openapi3.SchemaRef{Ref: "#/components/schemas/UserProfile"},
		},
	})

	// ProfileSettingsRequest schema
	spec.Components.Schemas["ProfileSettingsRequest"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Profile settings request object",
		Properties: map[string]*openapi3.SchemaRef{
			"theme": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "Color theme setting",
			}),
		},
	})

	// SkillRequest schema
	spec.Components.Schemas["SkillRequest"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Skill request object",
		Properties: map[string]*openapi3.SchemaRef{
			"name": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "Skill name",
			}),
			"level": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"integer"},
				Description: "Skill level",
			}),
		},
	})

	// ProfileRequest schema
	spec.Components.Schemas["ProfileRequest"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Profile request object",
		Properties: map[string]*openapi3.SchemaRef{
			"bio": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User biography",
			}),
			"skills": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"array"},
				Description: "List of skills",
				Items:       &openapi3.SchemaRef{Ref: "#/components/schemas/SkillRequest"},
			}),
			"settings": &openapi3.SchemaRef{Ref: "#/components/schemas/ProfileSettingsRequest"},
		},
	})

	// CreateUserRequest with profile
	spec.Components.Schemas["CreateUserRequest"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Create user request object",
		Properties: map[string]*openapi3.SchemaRef{
			"full_name": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User full name",
			}),
			"email": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User email",
			}),
			"phone": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User phone",
			}),
			"status": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "User status",
			}),
			"profile": &openapi3.SchemaRef{Ref: "#/components/schemas/ProfileRequest"},
		},
	})

	spec.Components.Schemas["OrganizationResponse"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Organization response object",
		Properties: map[string]*openapi3.SchemaRef{
			"id": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "Organization ID",
			}),
			"name": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "Organization name",
			}),
		},
	})

	// Add unmanaged schemas (not in Epoch registry - should be preserved)
	spec.Components.Schemas["ErrorResponse"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Error response object",
		Properties: map[string]*openapi3.SchemaRef{
			"message": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"string"},
				Description: "Error message",
			}),
			"code": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"integer"},
				Description: "Error code",
			}),
		},
	})

	spec.Components.Schemas["PaginationMeta"] = openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Description: "Pagination metadata",
		Properties: map[string]*openapi3.SchemaRef{
			"total": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"integer"},
				Description: "Total number of items",
			}),
			"page": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"integer"},
				Description: "Current page number",
			}),
			"page_size": openapi3.NewSchemaRef("", &openapi3.Schema{
				Type:        &openapi3.Types{"integer"},
				Description: "Number of items per page",
			}),
		},
	})

	return spec
}

// Verification helpers
type VerificationResults struct {
	TotalSpecs              int
	ParsedSuccessfully      int
	TransformationsCorrect  bool
	TransformV1Correct      bool
	TransformV2Correct      bool
	TransformV3Correct      bool
	TransformHEADCorrect    bool
	SmartMergingCorrect     bool
	SmartMergeV1Correct     bool
	SmartMergeV2Correct     bool
	SmartMergeV3Correct     bool
	SmartMergeHEADCorrect   bool
	UnmanagedSchemasCorrect bool
	ErrorResponsePreserved  bool
	PaginationMetaPreserved bool
	NamingConventionCorrect bool
	HEADUsesBareName        bool
	V1UsesVersionedOnly     bool
	V2UsesVersionedOnly     bool
	V3UsesVersionedOnly     bool
	SpecPreservationCorrect bool
	PathsPreserved          int
	SecurityPreserved       bool
	TagsPreserved           bool
	OperationDescPreserved  bool
}

func main() {
	fmt.Println("=== Epoch OpenAPI Schema Generation Example ===")

	// Define versions
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")
	v3, _ := epoch.NewDateVersion("2025-01-01")

	// Define version migrations
	// ============================================================================
	// TOP-LEVEL MIGRATIONS
	// ============================================================================

	// v1→v2: Add email and status fields
	// HEAD has these fields, v1 doesn't, so:
	// - Response: Remove them when sending to v1 (ResponseToPreviousVersion)
	// - Request: Add them when v1 client sends request (RequestToNextVersion)
	v1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email and status fields to User").
		ForType(CreateUserRequest{}).
		RequestToNextVersion().
		AddField("email", "").
		AddField("status", "active").
		ForType(UserResponse{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		RemoveField("status").
		Build()

	// v2→v3: Rename name to full_name, add phone
	// HEAD has full_name and phone, v2 has name (no phone), so:
	// - Response: Rename full_name→name, remove phone when sending to v2
	// - Request: Rename name→full_name, add phone when v2 client sends request
	v2ToV3 := epoch.NewVersionChangeBuilder(v2, v3).
		Description("Rename name to full_name, add phone").
		ForType(CreateUserRequest{}).
		RequestToNextVersion().
		RenameField("name", "full_name").
		AddField("phone", "").
		ForType(UserResponse{}).
		ResponseToPreviousVersion().
		RenameField("full_name", "name").
		RemoveField("phone").
		Build()

	// ============================================================================
	// NESTED TYPE MIGRATIONS (separate migrations for each nested type)
	// ============================================================================

	// Profile migration: bio ↔ biography
	profileV1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Transform profile.biography -> profile.bio").
		ForType(UserProfile{}, ProfileRequest{}).
		RequestToNextVersion().
		RenameField("biography", "bio").
		ResponseToPreviousVersion().
		RenameField("bio", "biography").
		Build()

	// Skill migration: name ↔ skill_name, add/remove level
	skillV1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Transform skills[].skill_name -> skills[].name, add level").
		ForType(Skill{}, SkillRequest{}).
		RequestToNextVersion().
		RenameField("skill_name", "name").
		AddField("level", 1).
		ResponseToPreviousVersion().
		RenameField("name", "skill_name").
		RemoveField("level").
		Build()

	// ProfileSettings migration: theme ↔ color_theme
	settingsV1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Transform settings.color_theme -> settings.theme").
		ForType(ProfileSettings{}, ProfileSettingsRequest{}).
		RequestToNextVersion().
		RenameField("color_theme", "theme").
		ResponseToPreviousVersion().
		RenameField("theme", "color_theme").
		Build()

	// Create Epoch instance with all types and migrations
	epochInstance, err := epoch.NewEpoch().
		WithHeadVersion().
		WithVersions(v1, v2, v3). // Versions in ascending order (oldest first)
		WithChanges(
			// Top-level migrations
			v1ToV2, v2ToV3,
			// Nested type migrations (each nested type needs its own migration)
			profileV1ToV2, skillV1ToV2, settingsV1ToV2,
		).
		WithTypes(
			// Top-level types
			CreateUserRequest{},
			UserResponse{},
			OrganizationResponse{},
			// Response nested types
			UserProfile{},
			Skill{},
			ProfileSettings{},
			// Request nested types
			ProfileRequest{},
			SkillRequest{},
			ProfileSettingsRequest{},
		).
		Build()

	if err != nil {
		log.Fatalf("Failed to create Epoch instance: %v", err)
	}

	fmt.Println("✓ Created Epoch instance with 3 versions + HEAD")
	fmt.Println("  Versions: 2024-01-01, 2024-06-01, 2025-01-01, head")

	// Register endpoints via Gin router using WrapHandler pattern
	// This is the production pattern that automatically registers types
	router := gin.New()
	routerGroup := router.Group("")

	routerGroup.GET("/users/:id",
		epochInstance.WrapHandler(getUser).
			Returns(UserResponse{}).
			ToHandlerFunc("GET", "/users/:id"))

	routerGroup.POST("/users",
		epochInstance.WrapHandler(createUser).
			Accepts(CreateUserRequest{}).
			Returns(UserResponse{}).
			ToHandlerFunc("POST", "/users"))

	routerGroup.GET("/organizations/:id",
		epochInstance.WrapHandler(getOrganization).
			Returns(OrganizationResponse{}).
			ToHandlerFunc("GET", "/organizations/:id"))

	fmt.Println("✓ Registered endpoints via Gin router")

	// Create schema generator
	generator := openapi.NewSchemaGenerator(openapi.SchemaGeneratorConfig{
		VersionBundle: epochInstance.VersionBundle(),
		TypeRegistry:  epochInstance.EndpointRegistry(),
		OutputFormat:  "yaml",
	})

	fmt.Println("✓ Created OpenAPI schema generator")
	fmt.Println()

	// Section 1: Generate from Scratch
	fmt.Println("📝 Test 1: Generate from Scratch")

	baseSpec := createBaseSpec()
	fmt.Println("  ✓ Created minimal base spec")

	versionedSpecs, err := generator.GenerateVersionedSpecs(baseSpec)
	if err != nil {
		log.Fatalf("Failed to generate versioned specs: %v", err)
	}
	fmt.Printf("  ✓ Generated %d versioned specs\n", len(versionedSpecs))

	// Create output directory relative to this example
	fromScratchDir := filepath.Join("output", "from_scratch")
	if err := os.MkdirAll(fromScratchDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Write specs to files
	filenamePattern := filepath.Join(fromScratchDir, "api_%s.yaml")
	if err := generator.WriteVersionedSpecs(versionedSpecs, filenamePattern); err != nil {
		log.Fatalf("Failed to write versioned specs: %v", err)
	}
	fmt.Printf("  ✓ Wrote to %s/\n", fromScratchDir)
	fmt.Println()

	// Section 2: Smart Merging with Existing Spec
	fmt.Println("📝 Test 2: Smart Merging with Existing Spec")

	existingSpec := createExistingSpec()
	fmt.Println("  ✓ Created base spec with paths, security, schemas")

	versionedSpecsWithBase, err := generator.GenerateVersionedSpecs(existingSpec)
	if err != nil {
		log.Fatalf("Failed to generate versioned specs with base: %v", err)
	}
	fmt.Printf("  ✓ Generated %d versioned specs with smart merging\n", len(versionedSpecsWithBase))

	// Create output directory relative to this example
	withExistingDir := filepath.Join("output", "with_existing_spec")
	if err := os.MkdirAll(withExistingDir, 0755); err != nil {
		log.Fatalf("Failed to create output directory: %v", err)
	}

	// Write specs to files
	filenamePattern = filepath.Join(withExistingDir, "api_%s.yaml")
	if err := generator.WriteVersionedSpecs(versionedSpecsWithBase, filenamePattern); err != nil {
		log.Fatalf("Failed to write versioned specs with base: %v", err)
	}
	fmt.Printf("  ✓ Wrote to %s/\n", withExistingDir)
	fmt.Println()

	// Section 3: Comprehensive Verification
	fmt.Println("✅ Verification Results:")

	results := &VerificationResults{
		TotalSpecs: 8,
	}

	// Load all generated specs
	loader := openapi3.NewLoader()
	loader.IsExternalRefsAllowed = true

	specs := make(map[string]*openapi3.T)
	versions := []string{"2024-01-01", "2024-06-01", "2025-01-01", "head"}

	// Load from_scratch specs
	for _, version := range versions {
		filename := filepath.Join(fromScratchDir, fmt.Sprintf("api_%s.yaml", version))
		spec, err := loader.LoadFromFile(filename)
		if err != nil {
			log.Printf("Failed to load %s: %v", filename, err)
			continue
		}
		results.ParsedSuccessfully++
		specs[fmt.Sprintf("from_scratch_%s", version)] = spec
	}

	// Load with_existing_spec specs
	for _, version := range versions {
		filename := filepath.Join(withExistingDir, fmt.Sprintf("api_%s.yaml", version))
		spec, err := loader.LoadFromFile(filename)
		if err != nil {
			log.Printf("Failed to load %s: %v", filename, err)
			continue
		}
		results.ParsedSuccessfully++
		specs[fmt.Sprintf("with_existing_%s", version)] = spec
	}

	fmt.Printf("  Parsing: %d/%d specs valid ✓\n", results.ParsedSuccessfully, results.TotalSpecs)
	fmt.Println()

	// Verify Epoch Transformations (using with_existing_spec)
	fmt.Println("  Epoch Transformations:")

	v1Spec := specs["with_existing_2024-01-01"]
	v2Spec := specs["with_existing_2024-06-01"]
	v3Spec := specs["with_existing_2025-01-01"]
	headSpec := specs["with_existing_head"]

	// Check v1 transformations (uses bare name when transforming existing schema)
	if v1Spec != nil {
		v1Schema := v1Spec.Components.Schemas["UserResponse"]
		if v1Schema != nil && v1Schema.Value != nil {
			hasEmail := v1Schema.Value.Properties["email"] != nil
			hasStatus := v1Schema.Value.Properties["status"] != nil
			results.TransformV1Correct = !hasEmail && !hasStatus
			if results.TransformV1Correct {
				fmt.Println("    ✓ v1: email and status removed")
			} else {
				fmt.Println("    ✗ v1: email and status should be removed")
			}
		}
	}

	// Check v2 transformations (uses bare name when transforming existing schema)
	if v2Spec != nil {
		v2Schema := v2Spec.Components.Schemas["UserResponse"]
		if v2Schema != nil && v2Schema.Value != nil {
			hasName := v2Schema.Value.Properties["name"] != nil
			hasFullName := v2Schema.Value.Properties["full_name"] != nil
			hasPhone := v2Schema.Value.Properties["phone"] != nil
			results.TransformV2Correct = hasName && !hasFullName && !hasPhone
			if results.TransformV2Correct {
				fmt.Println("    ✓ v2: full_name→name, phone removed")
			} else {
				fmt.Println("    ✗ v2: should have name (not full_name), phone removed")
			}
		}
	}

	// Check v3 and HEAD have all fields (should be identical - no v3→HEAD change defined)
	if v3Spec != nil {
		v3Schema := v3Spec.Components.Schemas["UserResponse"]
		if v3Schema != nil && v3Schema.Value != nil {
			// v3 should be identical to HEAD (all fields with full_name)
			hasAllFields := v3Schema.Value.Properties["id"] != nil &&
				v3Schema.Value.Properties["full_name"] != nil &&
				v3Schema.Value.Properties["email"] != nil &&
				v3Schema.Value.Properties["phone"] != nil &&
				v3Schema.Value.Properties["status"] != nil &&
				v3Schema.Value.Properties["created_at"] != nil
			results.TransformV3Correct = hasAllFields
		}
	}

	if headSpec != nil {
		headSchema := headSpec.Components.Schemas["UserResponse"]
		if headSchema != nil && headSchema.Value != nil {
			hasAllFields := headSchema.Value.Properties["id"] != nil &&
				headSchema.Value.Properties["full_name"] != nil &&
				headSchema.Value.Properties["email"] != nil &&
				headSchema.Value.Properties["phone"] != nil &&
				headSchema.Value.Properties["status"] != nil &&
				headSchema.Value.Properties["created_at"] != nil
			results.TransformHEADCorrect = hasAllFields
		}
	}

	if results.TransformV3Correct && results.TransformHEADCorrect {
		fmt.Println("    ✓ v3: all fields (identical to HEAD)")
		fmt.Println("    ✓ HEAD: all fields present with full_name")
	} else {
		if !results.TransformV3Correct {
			fmt.Println("    ✗ v3: should have all fields (identical to HEAD)")
		}
		if !results.TransformHEADCorrect {
			fmt.Println("    ✗ HEAD: should have all fields")
		}
	}

	results.TransformationsCorrect = results.TransformV1Correct &&
		results.TransformV2Correct &&
		results.TransformV3Correct &&
		results.TransformHEADCorrect

	fmt.Println()

	// Verify Request Schema Transformations
	fmt.Println("  Request Schema Transformations:")

	// Note: Unlike response schemas, request schemas use bare names (not versioned)
	// Each version gets its own transformed CreateUserRequest schema

	// Check v1 request transformations
	if v1Spec != nil {
		v1RequestSchema := v1Spec.Components.Schemas["CreateUserRequest"]
		if v1RequestSchema != nil && v1RequestSchema.Value != nil {
			hasEmail := v1RequestSchema.Value.Properties["email"] != nil
			hasStatus := v1RequestSchema.Value.Properties["status"] != nil
			hasPhone := v1RequestSchema.Value.Properties["phone"] != nil
			hasName := v1RequestSchema.Value.Properties["name"] != nil
			hasFullName := v1RequestSchema.Value.Properties["full_name"] != nil
			v1RequestCorrect := !hasEmail && !hasStatus && !hasPhone && hasName && !hasFullName
			if v1RequestCorrect {
				fmt.Println("    ✓ v1 request: name only (no email, status, phone, full_name)")
			} else {
				fmt.Printf("    ✗ v1 request: should have name only (has email:%v status:%v phone:%v name:%v full_name:%v)\n",
					hasEmail, hasStatus, hasPhone, hasName, hasFullName)
			}
		}
	}

	// Check v2 request transformations
	if v2Spec != nil {
		v2RequestSchema := v2Spec.Components.Schemas["CreateUserRequest"]
		if v2RequestSchema != nil && v2RequestSchema.Value != nil {
			hasEmail := v2RequestSchema.Value.Properties["email"] != nil
			hasStatus := v2RequestSchema.Value.Properties["status"] != nil
			hasPhone := v2RequestSchema.Value.Properties["phone"] != nil
			hasName := v2RequestSchema.Value.Properties["name"] != nil
			hasFullName := v2RequestSchema.Value.Properties["full_name"] != nil
			v2RequestCorrect := hasEmail && hasStatus && !hasPhone && hasName && !hasFullName
			if v2RequestCorrect {
				fmt.Println("    ✓ v2 request: name, email, status (no phone, full_name)")
			} else {
				fmt.Printf("    ✗ v2 request: should have name, email, status (has email:%v status:%v phone:%v name:%v full_name:%v)\n",
					hasEmail, hasStatus, hasPhone, hasName, hasFullName)
			}
		}
	}

	// Check v3 and HEAD request transformations
	if v3Spec != nil {
		v3RequestSchema := v3Spec.Components.Schemas["CreateUserRequest"]
		if v3RequestSchema != nil && v3RequestSchema.Value != nil {
			hasEmail := v3RequestSchema.Value.Properties["email"] != nil
			hasStatus := v3RequestSchema.Value.Properties["status"] != nil
			hasPhone := v3RequestSchema.Value.Properties["phone"] != nil
			hasFullName := v3RequestSchema.Value.Properties["full_name"] != nil
			v3RequestCorrect := hasEmail && hasStatus && hasPhone && hasFullName
			if v3RequestCorrect {
				fmt.Println("    ✓ v3 request: all fields with full_name")
			} else {
				fmt.Printf("    ✗ v3 request: should have all fields (has email:%v status:%v phone:%v full_name:%v)\n",
					hasEmail, hasStatus, hasPhone, hasFullName)
			}
		}
	}

	if headSpec != nil {
		headRequestSchema := headSpec.Components.Schemas["CreateUserRequest"]
		if headRequestSchema != nil && headRequestSchema.Value != nil {
			hasEmail := headRequestSchema.Value.Properties["email"] != nil
			hasStatus := headRequestSchema.Value.Properties["status"] != nil
			hasPhone := headRequestSchema.Value.Properties["phone"] != nil
			hasFullName := headRequestSchema.Value.Properties["full_name"] != nil
			headRequestCorrect := hasEmail && hasStatus && hasPhone && hasFullName
			if headRequestCorrect {
				fmt.Println("    ✓ HEAD request: all fields with full_name")
			} else {
				fmt.Printf("    ✗ HEAD request: should have all fields (has email:%v status:%v phone:%v full_name:%v)\n",
					hasEmail, hasStatus, hasPhone, hasFullName)
			}
		}
	}

	fmt.Println()

	// ============================================================================
	// NESTED TRANSFORMATION VERIFICATION
	// ============================================================================

	fmt.Println("  Nested Response Transformations:")

	// Verify UserProfile nested transformations (bio ↔ biography)
	nestedResponseV1Correct := false
	nestedResponseV2Correct := false

	if v1Spec != nil {
		profileSchema := v1Spec.Components.Schemas["UserProfile"]
		if profileSchema != nil && profileSchema.Value != nil {
			hasBiography := profileSchema.Value.Properties["biography"] != nil
			hasBio := profileSchema.Value.Properties["bio"] != nil
			nestedResponseV1Correct = hasBiography && !hasBio
			if nestedResponseV1Correct {
				fmt.Println("    ✓ v1 UserProfile: bio → biography")
			} else {
				fmt.Printf("    ✗ v1 UserProfile: should have biography (has bio:%v biography:%v)\n", hasBio, hasBiography)
			}
		} else {
			fmt.Println("    ⚠ v1 UserProfile schema not found")
		}
	}

	if v2Spec != nil {
		profileSchema := v2Spec.Components.Schemas["UserProfile"]
		if profileSchema != nil && profileSchema.Value != nil {
			hasBio := profileSchema.Value.Properties["bio"] != nil
			hasBiography := profileSchema.Value.Properties["biography"] != nil
			nestedResponseV2Correct = hasBio && !hasBiography
			if nestedResponseV2Correct {
				fmt.Println("    ✓ v2+ UserProfile: bio (unchanged)")
			} else {
				fmt.Printf("    ✗ v2+ UserProfile: should have bio (has bio:%v biography:%v)\n", hasBio, hasBiography)
			}
		}
	}

	// Verify Skill nested array item transformations (name ↔ skill_name, level)
	nestedArrayV1Correct := false
	nestedArrayV2Correct := false

	if v1Spec != nil {
		skillSchema := v1Spec.Components.Schemas["Skill"]
		if skillSchema != nil && skillSchema.Value != nil {
			hasSkillName := skillSchema.Value.Properties["skill_name"] != nil
			hasName := skillSchema.Value.Properties["name"] != nil
			hasLevel := skillSchema.Value.Properties["level"] != nil
			nestedArrayV1Correct = hasSkillName && !hasName && !hasLevel
			if nestedArrayV1Correct {
				fmt.Println("    ✓ v1 Skill: name → skill_name, level removed")
			} else {
				fmt.Printf("    ✗ v1 Skill: should have skill_name only (has name:%v skill_name:%v level:%v)\n",
					hasName, hasSkillName, hasLevel)
			}
		} else {
			fmt.Println("    ⚠ v1 Skill schema not found")
		}
	}

	if v2Spec != nil {
		skillSchema := v2Spec.Components.Schemas["Skill"]
		if skillSchema != nil && skillSchema.Value != nil {
			hasName := skillSchema.Value.Properties["name"] != nil
			hasLevel := skillSchema.Value.Properties["level"] != nil
			hasSkillName := skillSchema.Value.Properties["skill_name"] != nil
			nestedArrayV2Correct = hasName && hasLevel && !hasSkillName
			if nestedArrayV2Correct {
				fmt.Println("    ✓ v2+ Skill: name + level (unchanged)")
			} else {
				fmt.Printf("    ✗ v2+ Skill: should have name + level (has name:%v level:%v skill_name:%v)\n",
					hasName, hasLevel, hasSkillName)
			}
		}
	}

	// Verify ProfileSettings deeply nested transformations (theme ↔ color_theme)
	nestedDeepV1Correct := false
	nestedDeepV2Correct := false

	if v1Spec != nil {
		settingsSchema := v1Spec.Components.Schemas["ProfileSettings"]
		if settingsSchema != nil && settingsSchema.Value != nil {
			hasColorTheme := settingsSchema.Value.Properties["color_theme"] != nil
			hasTheme := settingsSchema.Value.Properties["theme"] != nil
			nestedDeepV1Correct = hasColorTheme && !hasTheme
			if nestedDeepV1Correct {
				fmt.Println("    ✓ v1 ProfileSettings: theme → color_theme")
			} else {
				fmt.Printf("    ✗ v1 ProfileSettings: should have color_theme (has theme:%v color_theme:%v)\n",
					hasTheme, hasColorTheme)
			}
		} else {
			fmt.Println("    ⚠ v1 ProfileSettings schema not found")
		}
	}

	if v2Spec != nil {
		settingsSchema := v2Spec.Components.Schemas["ProfileSettings"]
		if settingsSchema != nil && settingsSchema.Value != nil {
			hasTheme := settingsSchema.Value.Properties["theme"] != nil
			hasColorTheme := settingsSchema.Value.Properties["color_theme"] != nil
			nestedDeepV2Correct = hasTheme && !hasColorTheme
			if nestedDeepV2Correct {
				fmt.Println("    ✓ v2+ ProfileSettings: theme (unchanged)")
			} else {
				fmt.Printf("    ✗ v2+ ProfileSettings: should have theme (has theme:%v color_theme:%v)\n",
					hasTheme, hasColorTheme)
			}
		}
	}

	fmt.Println()

	fmt.Println("  Nested Request Transformations:")

	// Verify ProfileRequest nested transformations
	nestedReqV1Correct := false

	if v1Spec != nil {
		profileReqSchema := v1Spec.Components.Schemas["ProfileRequest"]
		if profileReqSchema != nil && profileReqSchema.Value != nil {
			hasBiography := profileReqSchema.Value.Properties["biography"] != nil
			hasBio := profileReqSchema.Value.Properties["bio"] != nil
			nestedReqV1Correct = hasBiography && !hasBio
			if nestedReqV1Correct {
				fmt.Println("    ✓ v1 ProfileRequest: bio → biography")
			} else {
				fmt.Printf("    ✗ v1 ProfileRequest: should have biography (has bio:%v biography:%v)\n", hasBio, hasBiography)
			}
		} else {
			fmt.Println("    ⚠ v1 ProfileRequest schema not found")
		}
	}

	// Verify SkillRequest nested transformations
	nestedSkillReqV1Correct := false

	if v1Spec != nil {
		skillReqSchema := v1Spec.Components.Schemas["SkillRequest"]
		if skillReqSchema != nil && skillReqSchema.Value != nil {
			hasSkillName := skillReqSchema.Value.Properties["skill_name"] != nil
			hasName := skillReqSchema.Value.Properties["name"] != nil
			hasLevel := skillReqSchema.Value.Properties["level"] != nil
			nestedSkillReqV1Correct = hasSkillName && !hasName && !hasLevel
			if nestedSkillReqV1Correct {
				fmt.Println("    ✓ v1 SkillRequest: name → skill_name, level removed")
			} else {
				fmt.Printf("    ✗ v1 SkillRequest: should have skill_name only (has name:%v skill_name:%v level:%v)\n",
					hasName, hasSkillName, hasLevel)
			}
		} else {
			fmt.Println("    ⚠ v1 SkillRequest schema not found")
		}
	}

	// Verify ProfileSettingsRequest nested transformations
	nestedSettingsReqV1Correct := false

	if v1Spec != nil {
		settingsReqSchema := v1Spec.Components.Schemas["ProfileSettingsRequest"]
		if settingsReqSchema != nil && settingsReqSchema.Value != nil {
			hasColorTheme := settingsReqSchema.Value.Properties["color_theme"] != nil
			hasTheme := settingsReqSchema.Value.Properties["theme"] != nil
			nestedSettingsReqV1Correct = hasColorTheme && !hasTheme
			if nestedSettingsReqV1Correct {
				fmt.Println("    ✓ v1 ProfileSettingsRequest: theme → color_theme")
			} else {
				fmt.Printf("    ✗ v1 ProfileSettingsRequest: should have color_theme (has theme:%v color_theme:%v)\n",
					hasTheme, hasColorTheme)
			}
		} else {
			fmt.Println("    ⚠ v1 ProfileSettingsRequest schema not found")
		}
	}

	// Overall nested transformations check
	nestedTransformationsCorrect := nestedResponseV1Correct && nestedResponseV2Correct &&
		nestedArrayV1Correct && nestedArrayV2Correct &&
		nestedDeepV1Correct && nestedDeepV2Correct &&
		nestedReqV1Correct && nestedSkillReqV1Correct && nestedSettingsReqV1Correct

	fmt.Println()

	// Verify Smart Merging (description preservation)
	fmt.Println("  Smart Merging:")

	// Check v1 descriptions preserved (uses bare name when transforming existing schema)
	if v1Spec != nil {
		v1Schema := v1Spec.Components.Schemas["UserResponse"]
		if v1Schema != nil && v1Schema.Value != nil && v1Schema.Value.Properties["id"] != nil {
			idDesc := v1Schema.Value.Properties["id"].Value.Description
			results.SmartMergeV1Correct = idDesc == "User ID from base spec"
			if results.SmartMergeV1Correct {
				fmt.Println("    ✓ Base descriptions preserved in v1")
			} else {
				fmt.Println("    ✗ Base descriptions should be preserved in v1")
			}
		}
	}

	// Check v2 descriptions preserved (uses bare name when transforming existing schema)
	if v2Spec != nil {
		v2Schema := v2Spec.Components.Schemas["UserResponse"]
		if v2Schema != nil && v2Schema.Value != nil && v2Schema.Value.Properties["id"] != nil {
			idDesc := v2Schema.Value.Properties["id"].Value.Description
			results.SmartMergeV2Correct = idDesc == "User ID from base spec"
			if results.SmartMergeV2Correct {
				fmt.Println("    ✓ Base descriptions preserved in v2")
			} else {
				fmt.Println("    ✗ Base descriptions should be preserved in v2")
			}
		}
	}

	// Check v3/HEAD descriptions preserved (both use bare name)
	if v3Spec != nil {
		v3Schema := v3Spec.Components.Schemas["UserResponse"]
		if v3Schema != nil && v3Schema.Value != nil && v3Schema.Value.Properties["id"] != nil {
			idDesc := v3Schema.Value.Properties["id"].Value.Description
			results.SmartMergeV3Correct = idDesc == "User ID from base spec"
		}
	}

	if headSpec != nil {
		headSchema := headSpec.Components.Schemas["UserResponse"]
		if headSchema != nil && headSchema.Value != nil && headSchema.Value.Properties["id"] != nil {
			idDesc := headSchema.Value.Properties["id"].Value.Description
			results.SmartMergeHEADCorrect = idDesc == "User ID from base spec"
		}
	}

	if results.SmartMergeV3Correct && results.SmartMergeHEADCorrect {
		fmt.Println("    ✓ Base descriptions preserved in v3/HEAD")
	} else {
		fmt.Println("    ✗ Base descriptions should be preserved in v3/HEAD")
	}

	results.SmartMergingCorrect = results.SmartMergeV1Correct &&
		results.SmartMergeV2Correct &&
		results.SmartMergeV3Correct &&
		results.SmartMergeHEADCorrect

	fmt.Println()

	// Verify Unmanaged Schemas
	fmt.Println("  Unmanaged Schemas:")

	errorResponseCount := 0
	paginationMetaCount := 0

	for key, spec := range specs {
		if spec.Components != nil && spec.Components.Schemas != nil {
			if spec.Components.Schemas["ErrorResponse"] != nil {
				errorResponseCount++
			}
			if spec.Components.Schemas["PaginationMeta"] != nil {
				paginationMetaCount++
			}
		}
		_ = key // Avoid unused variable
	}

	results.ErrorResponsePreserved = errorResponseCount == 4 // Only in with_existing_spec specs
	results.PaginationMetaPreserved = paginationMetaCount == 4

	if results.ErrorResponsePreserved {
		fmt.Println("    ✓ ErrorResponse in all 4 with_existing_spec specs")
	} else {
		fmt.Printf("    ✗ ErrorResponse found in %d/4 specs\n", errorResponseCount)
	}

	if results.PaginationMetaPreserved {
		fmt.Println("    ✓ PaginationMeta in all 4 with_existing_spec specs")
	} else {
		fmt.Printf("    ✗ PaginationMeta found in %d/4 specs\n", paginationMetaCount)
	}

	results.UnmanagedSchemasCorrect = results.ErrorResponsePreserved && results.PaginationMetaPreserved

	fmt.Println()

	// Verify Schema Naming Convention
	// When transforming existing schemas from base spec, bare names are preserved
	fmt.Println("  Schema Naming:")

	if headSpec != nil && headSpec.Components != nil {
		hasBareName := headSpec.Components.Schemas["UserResponse"] != nil
		hasNoVersionedName := headSpec.Components.Schemas["UserResponseV20250101"] == nil
		results.HEADUsesBareName = hasBareName && hasNoVersionedName
		if results.HEADUsesBareName {
			fmt.Println("    ✓ HEAD uses bare names")
		} else {
			fmt.Println("    ✗ HEAD should use bare names (not versioned)")
		}
	}

	if v1Spec != nil && v1Spec.Components != nil {
		// When transforming existing schemas, bare names are preserved
		hasBareName := v1Spec.Components.Schemas["UserResponse"] != nil
		hasNoVersionedName := v1Spec.Components.Schemas["UserResponseV20240101"] == nil
		results.V1UsesVersionedOnly = hasBareName && hasNoVersionedName
		if results.V1UsesVersionedOnly {
			fmt.Println("    ✓ v1 uses bare names (transformed from base)")
		} else {
			fmt.Println("    ✗ v1 should use bare names when transforming existing schemas")
		}
	}

	if v2Spec != nil && v2Spec.Components != nil {
		// When transforming existing schemas, bare names are preserved
		hasBareName := v2Spec.Components.Schemas["UserResponse"] != nil
		hasNoVersionedName := v2Spec.Components.Schemas["UserResponseV20240601"] == nil
		results.V2UsesVersionedOnly = hasBareName && hasNoVersionedName
		if results.V2UsesVersionedOnly {
			fmt.Println("    ✓ v2 uses bare names (transformed from base)")
		} else {
			fmt.Println("    ✗ v2 should use bare names when transforming existing schemas")
		}
	}

	if v3Spec != nil && v3Spec.Components != nil {
		// When transforming existing schemas, bare names are preserved
		hasBareName := v3Spec.Components.Schemas["UserResponse"] != nil
		hasNoVersionedName := v3Spec.Components.Schemas["UserResponseV20250101"] == nil
		results.V3UsesVersionedOnly = hasBareName && hasNoVersionedName
		if results.V3UsesVersionedOnly {
			fmt.Println("    ✓ v3 uses bare names (transformed from base)")
		} else {
			fmt.Println("    ✗ v3 should use bare names when transforming existing schemas")
		}
	}

	results.NamingConventionCorrect = results.HEADUsesBareName &&
		results.V1UsesVersionedOnly &&
		results.V2UsesVersionedOnly &&
		results.V3UsesVersionedOnly

	fmt.Println()

	// Verify Full Spec Preservation
	fmt.Println("  Spec Preservation:")

	if headSpec != nil {
		if headSpec.Paths != nil {
			results.PathsPreserved = len(headSpec.Paths.Map())
		}

		if headSpec.Components != nil && headSpec.Components.SecuritySchemes != nil {
			results.SecurityPreserved = headSpec.Components.SecuritySchemes["BearerAuth"] != nil
		}

		if headSpec.Tags != nil {
			results.TagsPreserved = len(headSpec.Tags) > 0
		}

		if headSpec.Paths != nil {
			getUsersPath := headSpec.Paths.Find("/users")
			if getUsersPath != nil && getUsersPath.Get != nil {
				results.OperationDescPreserved = getUsersPath.Get.Description != ""
			}
		}
	}

	if results.PathsPreserved > 0 {
		fmt.Printf("    ✓ Paths preserved (%d paths)\n", results.PathsPreserved)
	} else {
		fmt.Println("    ✗ Paths should be preserved")
	}

	if results.SecurityPreserved {
		fmt.Println("    ✓ Security schemes preserved")
	} else {
		fmt.Println("    ✗ Security schemes should be preserved")
	}

	if results.TagsPreserved {
		fmt.Println("    ✓ Tags preserved")
	} else {
		fmt.Println("    ✗ Tags should be preserved")
	}

	if results.OperationDescPreserved {
		fmt.Println("    ✓ Operation descriptions preserved")
	} else {
		fmt.Println("    ✗ Operation descriptions should be preserved")
	}

	results.SpecPreservationCorrect = results.PathsPreserved > 0 &&
		results.SecurityPreserved &&
		results.TagsPreserved &&
		results.OperationDescPreserved

	fmt.Println()

	// Nested transformations summary
	if nestedTransformationsCorrect {
		fmt.Println("  Nested Transformations: ✓ All nested schemas transformed correctly")
	} else {
		fmt.Println("  Nested Transformations: ✗ Some nested transformations failed")
	}
	fmt.Println()

	// Final summary
	allPassed := results.TransformationsCorrect &&
		results.SmartMergingCorrect &&
		results.UnmanagedSchemasCorrect &&
		results.NamingConventionCorrect &&
		results.SpecPreservationCorrect &&
		nestedTransformationsCorrect

	if allPassed {
		fmt.Println("🎉 All tests passed! Smart merging and nested transformations working correctly.")
	} else {
		fmt.Println("⚠️  Some tests failed. Check output above for details.")
	}

	fmt.Printf("📁 Generated files in: %s\n", filepath.Join("examples", "schema-generation", "output"))
}
