package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/astronomer/epoch/epoch/openapi"
	"github.com/getkin/kin-openapi/openapi3"
)

// Example types that match the advanced example

type CreateUserRequest struct {
	FullName string `json:"full_name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone,omitempty"`
	Status   string `json:"status" binding:"required,oneof=active inactive pending suspended"`
}

type UserResponse struct {
	ID        int        `json:"id,omitempty" validate:"required"`
	FullName  string     `json:"full_name" validate:"required"`
	Email     string     `json:"email,omitempty" validate:"required,email"`
	Phone     string     `json:"phone,omitempty"`
	Status    string     `json:"status,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty" format:"date-time"`
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
		},
	})

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
	v1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email and status fields to User").
		ForType(UserResponse{}, CreateUserRequest{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		RemoveField("status").
		Build()

	v2ToV3 := epoch.NewVersionChangeBuilder(v2, v3).
		Description("Rename name to full_name, add phone").
		ForType(UserResponse{}, CreateUserRequest{}).
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
			CreateUserRequest{},
			UserResponse{},
			OrganizationResponse{},
		).
		Build()

	if err != nil {
		log.Fatalf("Failed to create Epoch instance: %v", err)
	}

	fmt.Println("✓ Created Epoch instance with 3 versions + HEAD")
	fmt.Println("  Versions: 2024-01-01, 2024-06-01, 2025-01-01, head")

	// Register types in endpoint registry for schema generation
	// In a real application, these would be registered via WrapHandler().Returns()/.Accepts()
	// but for this example, we register them directly
	epochInstance.EndpointRegistry().Register("GET", "/users/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/users/:id",
		ResponseType: reflect.TypeOf(UserResponse{}),
	})
	epochInstance.EndpointRegistry().Register("POST", "/users", &epoch.EndpointDefinition{
		Method:       "POST",
		PathPattern:  "/users",
		RequestType:  reflect.TypeOf(CreateUserRequest{}),
		ResponseType: reflect.TypeOf(UserResponse{}),
	})
	epochInstance.EndpointRegistry().Register("GET", "/organizations/:id", &epoch.EndpointDefinition{
		Method:       "GET",
		PathPattern:  "/organizations/:id",
		ResponseType: reflect.TypeOf(OrganizationResponse{}),
	})

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

	// Check v1 transformations
	if v1Spec != nil {
		v1Schema := v1Spec.Components.Schemas["UserResponseV20240101"]
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

	// Check v2 transformations
	if v2Spec != nil {
		v2Schema := v2Spec.Components.Schemas["UserResponseV20240601"]
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

	// Check v3 and HEAD have all fields
	if v3Spec != nil {
		v3Schema := v3Spec.Components.Schemas["UserResponseV20250101"]
		if v3Schema != nil && v3Schema.Value != nil {
			// v3 is between v2 and HEAD, so transformations from both v1ToV2 and v2ToV3 apply
			// v3 should have: id, name, created_at (missing email, status, phone, and using 'name' not 'full_name')
			hasExpectedFields := v3Schema.Value.Properties["id"] != nil &&
				v3Schema.Value.Properties["name"] != nil &&
				v3Schema.Value.Properties["created_at"] != nil
			hasEmail := v3Schema.Value.Properties["email"] != nil
			hasStatus := v3Schema.Value.Properties["status"] != nil
			hasPhone := v3Schema.Value.Properties["phone"] != nil
			hasFullName := v3Schema.Value.Properties["full_name"] != nil
			results.TransformV3Correct = hasExpectedFields && !hasEmail && !hasStatus && !hasPhone && !hasFullName
		}
	}

	if headSpec != nil {
		headSchema := headSpec.Components.Schemas["UserResponse"]
		if headSchema != nil && headSchema.Value != nil {
			hasAllFields := headSchema.Value.Properties["id"] != nil &&
				headSchema.Value.Properties["full_name"] != nil &&
				headSchema.Value.Properties["email"] != nil &&
				headSchema.Value.Properties["phone"] != nil &&
				headSchema.Value.Properties["status"] != nil
			results.TransformHEADCorrect = hasAllFields
		}
	}

	if results.TransformV3Correct && results.TransformHEADCorrect {
		fmt.Println("    ✓ v3: name only (cumulative transforms applied)")
		fmt.Println("    ✓ HEAD: all fields present with full_name")
	} else {
		if !results.TransformV3Correct {
			fmt.Println("    ✗ v3: should have id, name, created_at only")
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

	// Verify Smart Merging (description preservation)
	fmt.Println("  Smart Merging:")

	// Check v1 descriptions preserved
	if v1Spec != nil {
		v1Schema := v1Spec.Components.Schemas["UserResponseV20240101"]
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

	// Check v2 descriptions preserved
	if v2Spec != nil {
		v2Schema := v2Spec.Components.Schemas["UserResponseV20240601"]
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

	// Check v3/HEAD descriptions preserved
	if v3Spec != nil {
		v3Schema := v3Spec.Components.Schemas["UserResponseV20250101"]
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
		hasVersionedName := v1Spec.Components.Schemas["UserResponseV20240101"] != nil
		hasNoBareName := v1Spec.Components.Schemas["UserResponse"] == nil
		results.V1UsesVersionedOnly = hasVersionedName && hasNoBareName
		if results.V1UsesVersionedOnly {
			fmt.Println("    ✓ v1 uses versioned names only")
		} else {
			fmt.Println("    ✗ v1 should use versioned names only")
		}
	}

	if v2Spec != nil && v2Spec.Components != nil {
		hasVersionedName := v2Spec.Components.Schemas["UserResponseV20240601"] != nil
		hasNoBareName := v2Spec.Components.Schemas["UserResponse"] == nil
		results.V2UsesVersionedOnly = hasVersionedName && hasNoBareName
		if results.V2UsesVersionedOnly {
			fmt.Println("    ✓ v2 uses versioned names only")
		} else {
			fmt.Println("    ✗ v2 should use versioned names only")
		}
	}

	if v3Spec != nil && v3Spec.Components != nil {
		hasVersionedName := v3Spec.Components.Schemas["UserResponseV20250101"] != nil
		hasNoBareName := v3Spec.Components.Schemas["UserResponse"] == nil
		results.V3UsesVersionedOnly = hasVersionedName && hasNoBareName
		if results.V3UsesVersionedOnly {
			fmt.Println("    ✓ v3 uses versioned names only")
		} else {
			fmt.Println("    ✗ v3 should use versioned names only")
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

	// Final summary
	allPassed := results.TransformationsCorrect &&
		results.SmartMergingCorrect &&
		results.UnmanagedSchemasCorrect &&
		results.NamingConventionCorrect &&
		results.SpecPreservationCorrect

	if allPassed {
		fmt.Println("🎉 All tests passed! Smart merging working correctly.")
	} else {
		fmt.Println("⚠️  Some tests failed. Check output above for details.")
	}

	fmt.Printf("📁 Generated files in: %s\n", filepath.Join("examples", "schema-generation", "output"))
	fmt.Println()
	fmt.Println("📚 This example demonstrates:")
	fmt.Println("  ✓ Smart schema merging with base spec metadata")
	fmt.Println("  ✓ Preservation of unmanaged schemas")
	fmt.Println("  ✓ Version-specific transformations")
	fmt.Println("  ✓ Correct naming conventions (HEAD vs versioned)")
	fmt.Println("  ✓ Full spec preservation (paths, security, tags)")
	fmt.Println()
}
