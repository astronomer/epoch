package openapi

import (
	"reflect"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

// Helper functions

// createTestVersions creates the standard v1, v2, v3 versions used across tests
func createTestVersions() (*epoch.Version, *epoch.Version, *epoch.Version) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")
	v3, _ := epoch.NewDateVersion("2025-01-01")
	return v1, v2, v3
}

// createTestMigrations creates the standard v1→v2 and v2→v3 migrations
func createTestMigrations(v1, v2, v3 *epoch.Version) (*epoch.VersionChange, *epoch.VersionChange) {
	// v1→v2: Add email and status fields
	v1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email and status fields to User").
		ForType(swagTestCreateUserRequest{}).
		RequestToNextVersion().
		AddField("email", "").
		AddField("status", "active").
		ForType(swagTestUserResponse{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		RemoveField("status").
		Build()

	// v2→v3: Rename name to full_name, add phone
	v2ToV3 := epoch.NewVersionChangeBuilder(v2, v3).
		Description("Rename name to full_name, add phone").
		ForType(swagTestCreateUserRequest{}).
		RequestToNextVersion().
		RenameField("name", "full_name").
		AddField("phone", "").
		ForType(swagTestUserResponse{}).
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
func verifySchemaFields(schema *openapi3.SchemaRef, versionName string, expectedFields, unexpectedFields []string) {
	Expect(schema).NotTo(BeNil(), "%s: schema is nil", versionName)
	Expect(schema.Value).NotTo(BeNil(), "%s: schema value is nil", versionName)

	for _, field := range expectedFields {
		Expect(schema.Value.Properties[field]).NotTo(BeNil(), "%s: expected field %s to be present", versionName, field)
	}

	for _, field := range unexpectedFields {
		Expect(schema.Value.Properties[field]).To(BeNil(), "%s: should not have field %s", versionName, field)
	}
}

// getSchemaOrFail retrieves a schema from a spec or fails the test
func getSchemaOrFail(spec *openapi3.T, schemaName, versionName string) *openapi3.SchemaRef {
	schema := spec.Components.Schemas[schemaName]
	Expect(schema).NotTo(BeNil(), "%s: schema '%s' not found", versionName, schemaName)
	return schema
}

// dummyHandler is a no-op handler for registration testing
func dummyHandler(c *gin.Context) {
	// No-op handler - only used for type registration via WrapHandler
}

var _ = Describe("Swag Integration", func() {
	Describe("Debug", func() {
		It("should print out what each version actually contains", func() {
			// Setup versions and migrations using helpers
			v1, v2, v3 := createTestVersions()
			v1ToV2, v2ToV3 := createTestMigrations(v1, v2, v3)

			epochInstance, err := epoch.NewEpoch().
				WithHeadVersion().
				WithVersions(v1, v2, v3).
				WithChanges(v1ToV2, v2ToV3).
				WithTypes(swagTestUserResponse{}).
				Build()

			Expect(err).NotTo(HaveOccurred())

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
			Expect(err).NotTo(HaveOccurred())

			// Print out what each version has in sorted order
			versionOrder := []string{"head", "2025-01-01", "2024-06-01", "2024-01-01"}
			for _, versionStr := range versionOrder {
				spec, ok := versionedSpecs[versionStr]
				if !ok {
					GinkgoWriter.Printf("%s: not found\n", versionStr)
					continue
				}
				schema := spec.Components.Schemas["versionedapi.swagTestUserResponse"]
				if schema != nil && schema.Value != nil {
					var fields []string
					for fieldName := range schema.Value.Properties {
						fields = append(fields, fieldName)
					}
					GinkgoWriter.Printf("%s: %v\n", versionStr, fields)
				} else {
					GinkgoWriter.Printf("%s: schema not found or nil\n", versionStr)
				}
			}

			// Also check what versions are in the bundle
			GinkgoWriter.Printf("Versions in bundle:\n")
			for _, v := range epochInstance.VersionBundle().GetVersions() {
				GinkgoWriter.Printf("  - %s (changes: %d)\n", v.String(), len(v.Changes))
				for _, c := range v.Changes {
					if vc, ok := c.(*epoch.VersionChange); ok {
						GinkgoWriter.Printf("    Change: %s -> %s\n", vc.FromVersion().String(), vc.ToVersion().String())
					}
				}
			}
		})
	})

	Describe("Transform In Place", func() {
		var (
			epochInstance  *epoch.Epoch
			versionedSpecs map[string]*openapi3.T
		)

		BeforeEach(func() {
			// Setup versions and migrations using helpers
			v1, v2, v3 := createTestVersions()
			v1ToV2, v2ToV3 := createTestMigrations(v1, v2, v3)

			var err error
			epochInstance, err = epoch.NewEpoch().
				WithHeadVersion().
				WithVersions(v1, v2, v3).
				WithChanges(v1ToV2, v2ToV3).
				WithTypes(
					swagTestCreateUserRequest{},
					swagTestUserResponse{},
					swagTestOrganizationResponse{},
				).
				Build()

			Expect(err).NotTo(HaveOccurred())

			// Register types via Gin router (production pattern)
			dummyRouter := gin.New()
			routerGroup := dummyRouter.Group("")

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
			versionedSpecs, err = generator.GenerateVersionedSpecs(swagSpec)
			Expect(err).NotTo(HaveOccurred())

			// Verify we got all 4 versions (v1, v2, v3, head)
			Expect(versionedSpecs).To(HaveLen(4))
		})

		It("should preserve original name and fields for HEAD version", func() {
			headSpec, ok := versionedSpecs["head"]
			Expect(ok).To(BeTrue())

			schema := getSchemaOrFail(headSpec, "versionedapi.swagTestUserResponse", "HEAD")
			verifySchemaFields(schema, "HEAD",
				[]string{"id", "full_name", "email", "phone", "status", "created_at"},
				nil)
		})

		It("should make v3 equivalent to HEAD", func() {
			v3Spec, ok := versionedSpecs["2025-01-01"]
			Expect(ok).To(BeTrue())

			schema := getSchemaOrFail(v3Spec, "versionedapi.swagTestUserResponse", "v3")
			verifySchemaFields(schema, "v3",
				[]string{"id", "full_name", "email", "phone", "status", "created_at"},
				nil)
		})

		It("should apply v2ToV3 transformations for v2", func() {
			v2Spec, ok := versionedSpecs["2024-06-01"]
			Expect(ok).To(BeTrue())

			schema := getSchemaOrFail(v2Spec, "versionedapi.swagTestUserResponse", "v2")
			// v2 should have: id, name (renamed from full_name), email, status, created_at
			// v2 should NOT have: phone (removed by v2ToV3), full_name (renamed to name)
			verifySchemaFields(schema, "v2",
				[]string{"id", "name", "email", "status", "created_at"},
				[]string{"phone", "full_name"})
		})

		It("should apply cumulative transformations for v1", func() {
			v1Spec, ok := versionedSpecs["2024-01-01"]
			Expect(ok).To(BeTrue())

			schema := getSchemaOrFail(v1Spec, "versionedapi.swagTestUserResponse", "v1")
			// v1 should have: id, name (from v2ToV3), created_at
			// v1 should NOT have: email, status (removed by v1ToV2), phone, full_name
			verifySchemaFields(schema, "v1",
				[]string{"id", "name", "created_at"},
				[]string{"email", "status", "phone", "full_name"})
		})

		It("should preserve schema names from Swag", func() {
			for versionStr, spec := range versionedSpecs {
				// All versions should have the Swag-prefixed name
				Expect(spec.Components.Schemas["versionedapi.swagTestUserResponse"]).NotTo(BeNil(),
					"%s: expected schema 'versionedapi.swagTestUserResponse' to exist", versionStr)

				// Should NOT have versioned names like UserResponseV20240101
				for name := range spec.Components.Schemas {
					Expect(name).To(BeElementOf(
						"versionedapi.swagTestUserResponse",
						"versionedapi.swagTestCreateUserRequest",
						"versionedapi.swagTestOrganizationResponse",
					), "%s: unexpected schema name %s", versionStr, name)
				}
			}
		})

		It("should transform request schemas correctly", func() {
			// HEAD and v3: should have all fields with full_name
			headSpec, ok := versionedSpecs["head"]
			Expect(ok).To(BeTrue())
			headRequestSchema := getSchemaOrFail(headSpec, "versionedapi.swagTestCreateUserRequest", "HEAD")
			verifySchemaFields(headRequestSchema, "HEAD request",
				[]string{"full_name", "email", "phone", "status"},
				[]string{"name"})

			v3Spec, ok := versionedSpecs["2025-01-01"]
			Expect(ok).To(BeTrue())
			v3RequestSchema := getSchemaOrFail(v3Spec, "versionedapi.swagTestCreateUserRequest", "v3")
			verifySchemaFields(v3RequestSchema, "v3 request",
				[]string{"full_name", "email", "phone", "status"},
				[]string{"name"})

			// v2: should have name (not full_name), email, status, but not phone
			v2Spec, ok := versionedSpecs["2024-06-01"]
			Expect(ok).To(BeTrue())
			v2RequestSchema := getSchemaOrFail(v2Spec, "versionedapi.swagTestCreateUserRequest", "v2")
			verifySchemaFields(v2RequestSchema, "v2 request",
				[]string{"name", "email", "status"},
				[]string{"full_name", "phone"})

			// v1: should have name only (no email, status, phone, full_name)
			v1Spec, ok := versionedSpecs["2024-01-01"]
			Expect(ok).To(BeTrue())
			v1RequestSchema := getSchemaOrFail(v1Spec, "versionedapi.swagTestCreateUserRequest", "v1")
			verifySchemaFields(v1RequestSchema, "v1 request",
				[]string{"name"},
				[]string{"full_name", "email", "status", "phone"})
		})
	})

	Describe("Preserves Metadata", func() {
		It("should preserve descriptions and metadata from Swag", func() {
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
				WithVersions(v1).
				WithChanges(change).
				WithTypes(swagTestUserResponse{}).
				Build()

			Expect(err).NotTo(HaveOccurred())

			// Register endpoint via Gin router (production pattern)
			dummyRouter := gin.New()
			routerGroup := dummyRouter.Group("")

			routerGroup.GET("/users/:id",
				epochInstance.WrapHandler(dummyHandler).
					Returns(swagTestUserResponse{}).
					ToHandlerFunc("GET", "/users/:id"))

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
			Expect(err).NotTo(HaveOccurred())

			// Test HEAD: all metadata preserved
			headSpec := versionedSpecs["head"]
			schema := headSpec.Components.Schemas["versionedapi.swagTestUserResponse"]

			Expect(schema.Value.Description).To(Equal("User response with rich metadata from Swag"))

			idSchema := schema.Value.Properties["id"]
			Expect(idSchema.Value.Description).To(Equal("Unique user identifier from Swag"))
			Expect(idSchema.Value.Example).To(Equal(float64(42)))

			// Test v1: metadata preserved even after field removal
			v1Spec := versionedSpecs["2024-01-01"]
			v1Schema := v1Spec.Components.Schemas["versionedapi.swagTestUserResponse"]

			// Schema description should be preserved
			Expect(v1Schema.Value.Description).To(Equal("User response with rich metadata from Swag"))

			// Remaining fields should keep their metadata
			v1IdSchema := v1Schema.Value.Properties["id"]
			Expect(v1IdSchema.Value.Description).To(Equal("Unique user identifier from Swag"))
			Expect(v1IdSchema.Value.Example).To(Equal(float64(42)))

			// email field should be removed
			Expect(v1Schema.Value.Properties["email"]).To(BeNil())
		})
	})

	Describe("Production Pattern", func() {
		It("should replicate the production setup pattern", func() {
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

			Expect(err).NotTo(HaveOccurred())

			// Create dummy Gin router
			dummyRouter := gin.New()
			routerGroup := dummyRouter.Group("")

			// Register handlers via WrapHandler pattern
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

			// Create Swag-style base spec
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

			// Configure generator
			generator := NewSchemaGenerator(SchemaGeneratorConfig{
				VersionBundle:    epochInstance.VersionBundle(),
				TypeRegistry:     epochInstance.EndpointRegistry(),
				OutputFormat:     "yaml",
				SchemaNameMapper: swagSchemaNameMapper(),
			})

			// Generate versioned specs
			versionedSpecs, err := generator.GenerateVersionedSpecs(swagSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(versionedSpecs).To(HaveLen(4))

			// Verify endpoint registry was populated
			registry := epochInstance.EndpointRegistry()

			getUserDef, err := registry.Lookup("GET", "/users/:id")
			Expect(err).NotTo(HaveOccurred())
			Expect(getUserDef.ResponseType).To(Equal(reflect.TypeOf(swagTestUserResponse{})))

			createUserDef, err := registry.Lookup("POST", "/users")
			Expect(err).NotTo(HaveOccurred())
			Expect(createUserDef.RequestType).To(Equal(reflect.TypeOf(swagTestCreateUserRequest{})))
			Expect(createUserDef.ResponseType).To(Equal(reflect.TypeOf(swagTestUserResponse{})))

			getOrgDef, err := registry.Lookup("GET", "/organizations/:id")
			Expect(err).NotTo(HaveOccurred())
			Expect(getOrgDef.ResponseType).To(Equal(reflect.TypeOf(swagTestOrganizationResponse{})))
		})
	})
})
