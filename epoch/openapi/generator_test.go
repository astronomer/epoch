package openapi

import (
	"reflect"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

var _ = Describe("SchemaGenerator", func() {
	Describe("Initialization", func() {
		It("should create a new SchemaGenerator", func() {
			v1, _ := epoch.NewDateVersion("2024-01-01")
			versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

			registry := epoch.NewEndpointRegistry()

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
				OutputFormat:  "yaml",
			}

			generator := NewSchemaGenerator(config)

			Expect(generator).NotTo(BeNil())
			Expect(generator.config.OutputFormat).To(Equal("yaml"))
		})
	})

	Describe("Type Registration", func() {
		It("should get registered types", func() {
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

			Expect(types).To(HaveLen(2))

			// Check that both types are present
			typeMap := make(map[reflect.Type]bool)
			for _, typ := range types {
				typeMap[typ] = true
			}

			Expect(typeMap[reflect.TypeOf(TestUserRequest{})]).To(BeTrue())
			Expect(typeMap[reflect.TypeOf(TestUserResponse{})]).To(BeTrue())
		})
	})

	Describe("Version Suffix", func() {
		var generator *SchemaGenerator

		BeforeEach(func() {
			versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{})
			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
			}
			generator = NewSchemaGenerator(config)
		})

		It("should return empty suffix for HEAD version", func() {
			version := epoch.NewHeadVersion()
			suffix := generator.getVersionSuffix(version)
			Expect(suffix).To(Equal(""))
		})

		It("should return correct suffix for date version", func() {
			v, _ := epoch.NewDateVersion("2024-01-01")
			suffix := generator.getVersionSuffix(v)
			Expect(suffix).To(Equal("V20240101"))
		})

		It("should return correct suffix for semver version", func() {
			v, _ := epoch.NewSemverVersion("1.2.3")
			suffix := generator.getVersionSuffix(v)
			Expect(suffix).To(Equal("V123"))
		})
	})

	Describe("Spec Cloning", func() {
		It("should clone spec correctly", func() {
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
			Expect(clone).NotTo(BeIdenticalTo(original))

			// Check that values match
			Expect(clone.OpenAPI).To(Equal(original.OpenAPI))
			Expect(clone.Info.Title).To(Equal(original.Info.Title))

			// Check that components.schemas are preserved from base spec
			Expect(clone.Components.Schemas).To(HaveLen(1))
			Expect(clone.Components.Schemas["TestType"]).NotTo(BeNil())
		})
	})

	Describe("Integration", func() {
		It("should generate schema for type with migrations", func() {
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

			// Create version bundle
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

			Expect(err).NotTo(HaveOccurred())
			Expect(schema).NotTo(BeNil())

			// Check that required fields are present
			Expect(schema.Properties).NotTo(BeEmpty())
			Expect(schema.Properties["id"]).NotTo(BeNil())
			Expect(schema.Properties["name"]).NotTo(BeNil())
		})
	})

	Describe("Smart Merging", func() {
		It("should preserve base schemas and apply transformations", func() {
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
			Expect(err).NotTo(HaveOccurred())

			// Verify HEAD spec has bare schema name
			Expect(headSpec.Components.Schemas["TestUserResponse"]).NotTo(BeNil())

			// Verify description is preserved
			headSchema := headSpec.Components.Schemas["TestUserResponse"].Value
			Expect(headSchema.Properties["id"].Value.Description).To(Equal("User ID from base spec"))

			// Verify unmanaged schema is preserved
			Expect(headSpec.Components.Schemas["ErrorResponse"]).NotTo(BeNil())

			// Generate v1 spec
			v1Spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Schema exists in base spec, so it's transformed in place
			Expect(v1Spec.Components.Schemas["TestUserResponse"]).NotTo(BeNil())

			// Verify transformation was applied (email removed)
			v1Schema := v1Spec.Components.Schemas["TestUserResponse"].Value
			Expect(v1Schema).NotTo(BeNil())
			Expect(v1Schema.Properties["email"]).To(BeNil())

			// Verify description is still preserved after transformation
			if v1Schema.Properties["id"] != nil {
				Expect(v1Schema.Properties["id"].Value.Description).To(Equal("User ID from base spec"))
			}

			// Verify unmanaged schema is preserved in v1
			Expect(v1Spec.Components.Schemas["ErrorResponse"]).NotTo(BeNil())
		})
	})

	Describe("Swag Integration", func() {
		Context("Transform existing Swag schemas", func() {
			It("should transform schemas with package prefix", func() {
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

				// Migration from v1 to HEAD
				change := epoch.NewVersionChangeBuilder(v1, headVersion).
					Description("Add betterNewName and timezone fields").
					ForType(UpdateExampleRequest{}).
					ResponseToPreviousVersion().
					RenameField("betterNewName", "name").
					RemoveField("timezone").
					RequestToNextVersion().
					RenameField("name", "betterNewName").
					AddField("timezone", "").
					Build()

				vb, err := epoch.NewVersionBundle([]*epoch.Version{headVersion, v1})
				Expect(err).NotTo(HaveOccurred())
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
				Expect(err).NotTo(HaveOccurred())

				// Verify: v1 schema transformed in place with correct fields
				v1Schema := v1Spec.Components.Schemas["versionedapi.UpdateExampleRequest"]
				Expect(v1Schema).NotTo(BeNil())
				Expect(v1Schema.Value).NotTo(BeNil())

				// Should have "name" (renamed from betterNewName)
				Expect(v1Schema.Value.Properties["name"]).NotTo(BeNil())
				// Should NOT have "betterNewName"
				Expect(v1Schema.Value.Properties["betterNewName"]).To(BeNil())
				// Should NOT have "timezone" (removed)
				Expect(v1Schema.Value.Properties["timezone"]).To(BeNil())

				// Generate HEAD spec
				headSpec, err := generator.GenerateSpecForVersion(baseSpec, headVersion)
				Expect(err).NotTo(HaveOccurred())

				// Verify: HEAD schema is unchanged (no transformations)
				headSchema := headSpec.Components.Schemas["versionedapi.UpdateExampleRequest"]
				Expect(headSchema).NotTo(BeNil())
				Expect(headSchema.Value.Properties["betterNewName"]).NotTo(BeNil())
				Expect(headSchema.Value.Properties["timezone"]).NotTo(BeNil())
			})
		})

		Context("Fallback to generation", func() {
			It("should generate schema when it doesn't exist in base spec", func() {
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
				Expect(err).NotTo(HaveOccurred())

				// Should generate schema from scratch with versioned name
				generatedSchema := v1Spec.Components.Schemas["UpdateExampleRequestV10"]
				Expect(generatedSchema).NotTo(BeNil())
				Expect(generatedSchema.Value).NotTo(BeNil())
			})
		})

		Context("Mixed scenario", func() {
			It("should handle schemas that exist and don't exist", func() {
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
				Expect(err).NotTo(HaveOccurred())

				// ExistingRequest: should be transformed in place
				Expect(v1Spec.Components.Schemas["versionedapi.ExistingRequest"]).NotTo(BeNil())

				// MissingRequest: should be generated with versioned name
				Expect(v1Spec.Components.Schemas["MissingRequestV10"]).NotTo(BeNil())
			})
		})

		Context("No package prefix", func() {
			It("should work with identity mapper", func() {
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
				Expect(err).NotTo(HaveOccurred())

				// Should transform with same name
				Expect(v1Spec.Components.Schemas["UpdateExampleRequest"]).NotTo(BeNil())
			})
		})

		Context("Metadata preservation", func() {
			It("should preserve descriptions from base spec", func() {
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
				Expect(err).NotTo(HaveOccurred())

				schema := v1Spec.Components.Schemas["versionedapi.TestUserResponse"]
				Expect(schema).NotTo(BeNil())
				Expect(schema.Value.Description).To(Equal("User response from Swag"))
				if schema.Value.Properties["name"] != nil {
					Expect(schema.Value.Properties["name"].Value.Description).To(Equal("User name from Swag"))
				}
			})
		})
	})
})
