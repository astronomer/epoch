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

// Test types for nested type handling tests
type NestedMetadata struct {
	Version   string `json:"version"`
	CreatedBy string `json:"created_by"`
}

type NestedSubItem struct {
	ID    int    `json:"id"`
	Label string `json:"label"`
}

type NestedItem struct {
	ID       int             `json:"id"`
	Name     string          `json:"name"`
	SubItems []NestedSubItem `json:"sub_items"`
	Details  NestedMetadata  `json:"details"`
}

type NestedContainer struct {
	Items    []NestedItem   `json:"items"`
	Metadata NestedMetadata `json:"metadata"`
}

// Self-referential type for circular dependency testing
type SelfReferential struct {
	ID    int              `json:"id"`
	Name  string           `json:"name"`
	Child *SelfReferential `json:"child,omitempty"`
}

// Circular dependency types
type CircularA struct {
	ID   int        `json:"id"`
	RefB *CircularB `json:"ref_b,omitempty"`
}

type CircularB struct {
	Name string     `json:"name"`
	RefA *CircularA `json:"ref_a,omitempty"`
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

			// Create a base spec with the type
			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{
						"TestUserResponse": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"object"},
							Properties: map[string]*openapi3.SchemaRef{
								"id": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"integer"},
								}),
								"name": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								}),
								"email": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								}),
							},
						}),
					},
				},
			}

			// Generate spec for HEAD
			headVersion := epoch.NewHeadVersion()
			headSpec, err := generator.GenerateSpecForVersion(baseSpec, headVersion)
			Expect(err).NotTo(HaveOccurred())
			Expect(headSpec).NotTo(BeNil())

			// Get the schema from the generated spec
			schema := headSpec.Components.Schemas["TestUserResponse"]
			Expect(schema).NotTo(BeNil())
			Expect(schema.Value).NotTo(BeNil())

			// Check that required fields are present
			Expect(schema.Value.Properties).NotTo(BeEmpty())
			Expect(schema.Value.Properties["id"]).NotTo(BeNil())
			Expect(schema.Value.Properties["name"]).NotTo(BeNil())
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

				// Should generate schema from scratch using Go type name
				generatedSchema := v1Spec.Components.Schemas["UpdateExampleRequest"]
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

				// MissingRequest: should be generated using Go type name
				Expect(v1Spec.Components.Schemas["MissingRequest"]).NotTo(BeNil())
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

	Describe("Nested Type Handling", func() {
		var (
			generator     *SchemaGenerator
			v1            *epoch.Version
			versionBundle *epoch.VersionBundle
		)

		BeforeEach(func() {
			v1, _ = epoch.NewDateVersion("2024-01-01")
			versionBundle, _ = epoch.NewVersionBundle([]*epoch.Version{v1})
			registry := epoch.NewEndpointRegistry()

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)
		})

		Context("Nested type collection", func() {
			It("should collect nested objects from a struct", func() {
				generator.collectNestedTypesForGeneration(reflect.TypeOf(NestedContainer{}), v1)

				// Check that metadata object was registered
				versionKey := v1.String()
				componentName := generator.getComponentNameForType(versionKey, reflect.TypeOf(NestedMetadata{}))
				Expect(componentName).To(Equal("NestedMetadata"))

				// Check that it's in typesToGenerate
				typesToGen := generator.typesToGenerate[versionKey]
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(NestedMetadata{})))
			})

			It("should collect nested arrays from a struct", func() {
				generator.collectNestedTypesForGeneration(reflect.TypeOf(NestedContainer{}), v1)

				versionKey := v1.String()
				// Check that NestedItem (array element type) was registered
				componentName := generator.getComponentNameForType(versionKey, reflect.TypeOf(NestedItem{}))
				Expect(componentName).To(Equal("NestedItem"))

				typesToGen := generator.typesToGenerate[versionKey]
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(NestedItem{})))
			})

			It("should recursively collect deeply nested types", func() {
				generator.collectNestedTypesForGeneration(reflect.TypeOf(NestedContainer{}), v1)

				versionKey := v1.String()
				typesToGen := generator.typesToGenerate[versionKey]

				// Should collect all levels: NestedItem, NestedMetadata, NestedSubItem
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(NestedItem{})))
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(NestedMetadata{})))
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(NestedSubItem{})))
			})

			It("should handle circular dependencies without infinite recursion", func() {
				// Should complete without hanging
				generator.collectNestedTypesForGeneration(reflect.TypeOf(SelfReferential{}), v1)

				versionKey := v1.String()
				typesToGen := generator.typesToGenerate[versionKey]

				// SelfReferential should only appear once despite circular reference
				count := 0
				for _, typ := range typesToGen {
					if typ == reflect.TypeOf(SelfReferential{}) {
						count++
					}
				}
				Expect(count).To(Equal(1))
			})

			It("should handle mutual circular dependencies", func() {
				generator.collectNestedTypesForGeneration(reflect.TypeOf(CircularA{}), v1)

				versionKey := v1.String()
				typesToGen := generator.typesToGenerate[versionKey]

				// Both types should be collected exactly once
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(CircularB{})))

				countA := 0
				countB := 0
				for _, typ := range typesToGen {
					if typ == reflect.TypeOf(CircularA{}) {
						countA++
					}
					if typ == reflect.TypeOf(CircularB{}) {
						countB++
					}
				}
				Expect(countA).To(Equal(1))
				Expect(countB).To(Equal(1))
			})

			It("should handle pointer types by dereferencing", func() {
				generator.collectNestedTypesForGeneration(reflect.TypeOf(&NestedContainer{}), v1)

				versionKey := v1.String()
				typesToGen := generator.typesToGenerate[versionKey]

				// Should still collect nested types through pointer
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(NestedMetadata{})))
				Expect(typesToGen).To(ContainElement(reflect.TypeOf(NestedItem{})))
			})
		})

		Context("Nested type registration", func() {
			It("should register types with correct component names", func() {
				typ := reflect.TypeOf(NestedMetadata{})
				versionKey := v1.String()
				componentName := "NestedMetadata"

				generator.registerNestedType(versionKey, typ, componentName)

				// Check registration
				registeredName := generator.getComponentNameForType(versionKey, typ)
				Expect(registeredName).To(Equal(componentName))
			})

			It("should track types for generation", func() {
				typ := reflect.TypeOf(NestedMetadata{})
				versionKey := v1.String()

				generator.registerNestedType(versionKey, typ, "NestedMetadata")

				// Check it's in typesToGenerate
				typesToGen := generator.typesToGenerate[versionKey]
				Expect(typesToGen).To(ContainElement(typ))
			})

			It("should not duplicate types in typesToGenerate", func() {
				typ := reflect.TypeOf(NestedMetadata{})
				versionKey := v1.String()

				// Register twice
				generator.registerNestedType(versionKey, typ, "NestedMetadata")
				generator.registerNestedType(versionKey, typ, "NestedMetadata")

				// Should only appear once
				typesToGen := generator.typesToGenerate[versionKey]
				count := 0
				for _, t := range typesToGen {
					if t == typ {
						count++
					}
				}
				Expect(count).To(Equal(1))
			})
		})
	})

	Describe("Component Name Generation", func() {
		var generator *SchemaGenerator

		BeforeEach(func() {
			v1, _ := epoch.NewDateVersion("2024-01-01")
			versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
			registry := epoch.NewEndpointRegistry()

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)
		})

		DescribeTable("should generate correct component names",
			func(typ reflect.Type, expectedName string) {
				componentName := generator.generateComponentNameForType(typ)
				Expect(componentName).To(Equal(expectedName))
			},
			Entry("named struct", reflect.TypeOf(NestedMetadata{}), "NestedMetadata"),
			Entry("pointer to struct", reflect.TypeOf(&NestedMetadata{}), "NestedMetadata"),
			Entry("nested struct", reflect.TypeOf(NestedItem{}), "NestedItem"),
			Entry("slice of struct", reflect.TypeOf([]NestedItem{}), "NestedItemArray"),
			Entry("slice of pointer", reflect.TypeOf([]*NestedItem{}), "NestedItemArray"),
		)

		It("should handle anonymous structs", func() {
			anonymousType := reflect.TypeOf(struct {
				Field string
			}{})

			componentName := generator.generateComponentNameForType(anonymousType)
			// Should generate some name (exact name depends on implementation)
			Expect(componentName).NotTo(BeEmpty())
		})
	})

	Describe("Reference Replacement", func() {
		var (
			generator *SchemaGenerator
			spec      *openapi3.T
		)

		BeforeEach(func() {
			v1, _ := epoch.NewDateVersion("2024-01-01")
			versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
			registry := epoch.NewEndpointRegistry()

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)

			// Create a spec with component schemas
			spec = &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{
						"NestedMetadata": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"object"},
							Properties: map[string]*openapi3.SchemaRef{
								"version": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								}),
								"created_by": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								}),
							},
						}),
					},
				},
			}
		})

		It("should replace inline object schemas with refs", func() {
			// Create a schema with an inline object
			parentSchema := &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"metadata": openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: &openapi3.Types{"object"},
						Properties: map[string]*openapi3.SchemaRef{
							"version": openapi3.NewSchemaRef("", &openapi3.Schema{
								Type: &openapi3.Types{"string"},
							}),
							"created_by": openapi3.NewSchemaRef("", &openapi3.Schema{
								Type: &openapi3.Types{"string"},
							}),
						},
					}),
				},
			}

			err := generator.replaceNestedSchemasWithRefsGeneric(parentSchema, spec)
			Expect(err).NotTo(HaveOccurred())

			// Should replace inline object with $ref
			metadataRef := parentSchema.Properties["metadata"]
			Expect(metadataRef).NotTo(BeNil())
			Expect(metadataRef.Ref).To(Equal("#/components/schemas/NestedMetadata"))
		})

		It("should replace inline objects in array items with refs", func() {
			// Create a schema with array of inline objects
			parentSchema := &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"items": openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: &openapi3.Types{"array"},
						Items: &openapi3.SchemaRef{
							Value: &openapi3.Schema{
								Type: &openapi3.Types{"object"},
								Properties: map[string]*openapi3.SchemaRef{
									"version": openapi3.NewSchemaRef("", &openapi3.Schema{
										Type: &openapi3.Types{"string"},
									}),
									"created_by": openapi3.NewSchemaRef("", &openapi3.Schema{
										Type: &openapi3.Types{"string"},
									}),
								},
							},
						},
					}),
				},
			}

			err := generator.replaceNestedSchemasWithRefsGeneric(parentSchema, spec)
			Expect(err).NotTo(HaveOccurred())

			// Should replace array items with $ref
			itemsRef := parentSchema.Properties["items"]
			Expect(itemsRef).NotTo(BeNil())
			Expect(itemsRef.Value.Items).NotTo(BeNil())
			Expect(itemsRef.Value.Items.Ref).To(Equal("#/components/schemas/NestedMetadata"))
		})

		It("should leave existing refs unchanged", func() {
			// Create a schema that already has $ref
			parentSchema := &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"metadata": openapi3.NewSchemaRef("#/components/schemas/NestedMetadata", nil),
				},
			}

			err := generator.replaceNestedSchemasWithRefsGeneric(parentSchema, spec)
			Expect(err).NotTo(HaveOccurred())

			// Should remain unchanged
			metadataRef := parentSchema.Properties["metadata"]
			Expect(metadataRef.Ref).To(Equal("#/components/schemas/NestedMetadata"))
		})

		It("should not replace if no matching component found", func() {
			// Create a schema with inline object that doesn't match any component
			parentSchema := &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"unknown": openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: &openapi3.Types{"object"},
						Properties: map[string]*openapi3.SchemaRef{
							"unknown_field": openapi3.NewSchemaRef("", &openapi3.Schema{
								Type: &openapi3.Types{"string"},
							}),
						},
					}),
				},
			}

			err := generator.replaceNestedSchemasWithRefsGeneric(parentSchema, spec)
			Expect(err).NotTo(HaveOccurred())

			// Should remain inline since no match
			unknownRef := parentSchema.Properties["unknown"]
			Expect(unknownRef.Ref).To(BeEmpty())
			Expect(unknownRef.Value).NotTo(BeNil())
		})
	})

	Describe("findMatchingComponent", func() {
		var (
			generator *SchemaGenerator
			spec      *openapi3.T
		)

		BeforeEach(func() {
			v1, _ := epoch.NewDateVersion("2024-01-01")
			versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
			registry := epoch.NewEndpointRegistry()

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)

			spec = &openapi3.T{
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{
						"NestedMetadata": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"object"},
							Properties: map[string]*openapi3.SchemaRef{
								"version": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								}),
								"created_by": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								}),
							},
						}),
					},
				},
			}
		})

		It("should find matching component by property names", func() {
			inlineSchema := &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"version": openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: &openapi3.Types{"string"},
					}),
					"created_by": openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: &openapi3.Types{"string"},
					}),
				},
			}

			componentName := generator.findMatchingComponent(inlineSchema, spec)
			Expect(componentName).To(Equal("NestedMetadata"))
		})

		It("should return empty string if no match found", func() {
			inlineSchema := &openapi3.Schema{
				Type: &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{
					"different_field": openapi3.NewSchemaRef("", &openapi3.Schema{
						Type: &openapi3.Types{"string"},
					}),
				},
			}

			componentName := generator.findMatchingComponent(inlineSchema, spec)
			Expect(componentName).To(BeEmpty())
		})

		It("should return empty string for nil schema", func() {
			componentName := generator.findMatchingComponent(nil, spec)
			Expect(componentName).To(BeEmpty())
		})

		It("should return empty string for schema without properties", func() {
			inlineSchema := &openapi3.Schema{
				Type: &openapi3.Types{"object"},
			}

			componentName := generator.findMatchingComponent(inlineSchema, spec)
			Expect(componentName).To(BeEmpty())
		})
	})

	Describe("Multi-Pass Generation", func() {
		var (
			generator     *SchemaGenerator
			registry      *epoch.EndpointRegistry
			v1            *epoch.Version
			versionBundle *epoch.VersionBundle
		)

		BeforeEach(func() {
			v1, _ = epoch.NewDateVersion("2024-01-01")
			versionBundle, _ = epoch.NewVersionBundle([]*epoch.Version{v1})
			registry = epoch.NewEndpointRegistry()
		})

		It("should generate component schemas for nested objects", func() {
			// Register an endpoint with nested types
			registry.Register("GET", "/containers", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/containers",
				ResponseType: reflect.TypeOf(NestedContainer{}),
			})

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Should generate component schemas for all nested types
			Expect(spec.Components.Schemas["NestedMetadata"]).NotTo(BeNil())
			Expect(spec.Components.Schemas["NestedItem"]).NotTo(BeNil())
			Expect(spec.Components.Schemas["NestedSubItem"]).NotTo(BeNil())
		})

		It("should use refs for nested objects in parent schema", func() {
			registry.Register("GET", "/containers", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/containers",
				ResponseType: reflect.TypeOf(NestedContainer{}),
			})

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Parent container should have refs to nested types
			containerSchema := spec.Components.Schemas["NestedContainer"]
			Expect(containerSchema).NotTo(BeNil())

			// Metadata should be a $ref
			metadataRef := containerSchema.Value.Properties["metadata"]
			Expect(metadataRef).NotTo(BeNil())
			Expect(metadataRef.Ref).To(Equal("#/components/schemas/NestedMetadata"))
		})

		It("should use refs for array items", func() {
			registry.Register("GET", "/containers", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/containers",
				ResponseType: reflect.TypeOf(NestedContainer{}),
			})

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			containerSchema := spec.Components.Schemas["NestedContainer"]
			Expect(containerSchema).NotTo(BeNil())

			// Items array should use $ref for array items
			itemsRef := containerSchema.Value.Properties["items"]
			Expect(itemsRef).NotTo(BeNil())
			Expect(itemsRef.Value.Items).NotTo(BeNil())
			Expect(itemsRef.Value.Items.Ref).To(Equal("#/components/schemas/NestedItem"))
		})

		It("should handle deeply nested types (3+ levels)", func() {
			registry.Register("GET", "/containers", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/containers",
				ResponseType: reflect.TypeOf(NestedContainer{}),
			})

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// All three levels should exist: Container -> Item -> SubItem
			Expect(spec.Components.Schemas["NestedContainer"]).NotTo(BeNil())
			Expect(spec.Components.Schemas["NestedItem"]).NotTo(BeNil())
			Expect(spec.Components.Schemas["NestedSubItem"]).NotTo(BeNil())

			// Verify refs at each level
			itemSchema := spec.Components.Schemas["NestedItem"]
			subItemsRef := itemSchema.Value.Properties["sub_items"]
			Expect(subItemsRef.Value.Items.Ref).To(Equal("#/components/schemas/NestedSubItem"))
		})

		It("should preserve existing schemas when generating new nested ones", func() {
			// Register endpoint
			registry.Register("GET", "/containers", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/containers",
				ResponseType: reflect.TypeOf(NestedContainer{}),
			})

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)

			// Base spec has only NestedMetadata
			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{
						"ExistingSchema": openapi3.NewSchemaRef("", &openapi3.Schema{
							Type: &openapi3.Types{"object"},
							Properties: map[string]*openapi3.SchemaRef{
								"field": openapi3.NewSchemaRef("", &openapi3.Schema{
									Type: &openapi3.Types{"string"},
								}),
							},
						}),
					},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Should preserve existing schema
			Expect(spec.Components.Schemas["ExistingSchema"]).NotTo(BeNil())

			// Should also generate new nested schemas
			Expect(spec.Components.Schemas["NestedMetadata"]).NotTo(BeNil())
			Expect(spec.Components.Schemas["NestedItem"]).NotTo(BeNil())
		})
	})

	Describe("Array Return Types", func() {
		var (
			generator *SchemaGenerator
			registry  *epoch.EndpointRegistry
			v1        *epoch.Version
		)

		BeforeEach(func() {
			v1, _ = epoch.NewDateVersion("2024-01-01")
			versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
			registry = epoch.NewEndpointRegistry()

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)
		})

		It("should not create empty-named schemas for array response types", func() {
			// Register endpoint that returns an array
			arrayType := reflect.TypeOf([]TestUserResponse{})
			registry.Register("GET", "/users", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/users",
				ResponseType: arrayType,
			})

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Should NOT have a schema with empty name
			_, hasEmptySchema := spec.Components.Schemas[""]
			Expect(hasEmptySchema).To(BeFalse())
		})

		It("should register element type as component for array responses", func() {
			// Register endpoint that returns an array
			arrayType := reflect.TypeOf([]TestUserResponse{})
			registry.Register("GET", "/users", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/users",
				ResponseType: arrayType,
			})

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Should have TestUserResponse as a component
			userResponseSchema := spec.Components.Schemas["TestUserResponse"]
			Expect(userResponseSchema).NotTo(BeNil())
			Expect(userResponseSchema.Value).NotTo(BeNil())
			Expect(userResponseSchema.Value.Type).NotTo(BeNil())
			Expect((*userResponseSchema.Value.Type)[0]).To(Equal("object"))
		})

		It("should handle array request types", func() {
			// Register endpoint that accepts an array
			arrayType := reflect.TypeOf([]TestUserRequest{})
			registry.Register("POST", "/users/bulk", &epoch.EndpointDefinition{
				Method:      "POST",
				PathPattern: "/users/bulk",
				RequestType: arrayType,
			})

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Should NOT have a schema with empty name
			_, hasEmptySchema := spec.Components.Schemas[""]
			Expect(hasEmptySchema).To(BeFalse())

			// Should have TestUserRequest as a component
			userRequestSchema := spec.Components.Schemas["TestUserRequest"]
			Expect(userRequestSchema).NotTo(BeNil())
		})

		It("should handle nested arrays with custom types", func() {
			// Register endpoint that returns an array of nested items
			arrayType := reflect.TypeOf([]NestedItem{})
			registry.Register("GET", "/items", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/items",
				ResponseType: arrayType,
			})

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Should NOT have a schema with empty name
			_, hasEmptySchema := spec.Components.Schemas[""]
			Expect(hasEmptySchema).To(BeFalse())

			// Should have NestedItem and all its nested types
			Expect(spec.Components.Schemas["NestedItem"]).NotTo(BeNil())
			Expect(spec.Components.Schemas["NestedSubItem"]).NotTo(BeNil())
			Expect(spec.Components.Schemas["NestedMetadata"]).NotTo(BeNil())
		})

		It("should not register primitive array elements as schemas", func() {
			// Register endpoint that returns an array of strings
			arrayType := reflect.TypeOf([]string{})
			registry.Register("GET", "/tags", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/tags",
				ResponseType: arrayType,
			})

			baseSpec := &openapi3.T{
				OpenAPI: "3.0.3",
				Info:    &openapi3.Info{Title: "Test", Version: "1.0"},
				Components: &openapi3.Components{
					Schemas: openapi3.Schemas{},
				},
			}

			spec, err := generator.GenerateSpecForVersion(baseSpec, v1)
			Expect(err).NotTo(HaveOccurred())

			// Should NOT have any schemas since string is primitive
			Expect(len(spec.Components.Schemas)).To(Equal(0))
		})
	})

	Describe("Direction Detection", func() {
		var (
			generator *SchemaGenerator
			registry  *epoch.EndpointRegistry
		)

		BeforeEach(func() {
			v1, _ := epoch.NewDateVersion("2024-01-01")
			versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
			registry = epoch.NewEndpointRegistry()

			config := SchemaGeneratorConfig{
				VersionBundle: versionBundle,
				TypeRegistry:  registry,
			}
			generator = NewSchemaGenerator(config)
		})

		It("should detect request types", func() {
			registry.Register("POST", "/users", &epoch.EndpointDefinition{
				Method:      "POST",
				PathPattern: "/users",
				RequestType: reflect.TypeOf(TestUserRequest{}),
			})

			direction := generator.getDirectionForType(reflect.TypeOf(TestUserRequest{}))
			Expect(direction).To(Equal(SchemaDirectionRequest))
		})

		It("should detect response types", func() {
			registry.Register("GET", "/users", &epoch.EndpointDefinition{
				Method:       "GET",
				PathPattern:  "/users",
				ResponseType: reflect.TypeOf(TestUserResponse{}),
			})

			direction := generator.getDirectionForType(reflect.TypeOf(TestUserResponse{}))
			Expect(direction).To(Equal(SchemaDirectionResponse))
		})

		It("should default to response for unknown types", func() {
			// Type not registered in any endpoint
			direction := generator.getDirectionForType(reflect.TypeOf(NestedMetadata{}))
			Expect(direction).To(Equal(SchemaDirectionResponse))
		})

		It("should handle types used in both request and response", func() {
			// Register same type as both request and response
			registry.Register("POST", "/users", &epoch.EndpointDefinition{
				Method:       "POST",
				PathPattern:  "/users",
				RequestType:  reflect.TypeOf(TestUserRequest{}),
				ResponseType: reflect.TypeOf(TestUserRequest{}),
			})

			// Should return request since it checks request first
			direction := generator.getDirectionForType(reflect.TypeOf(TestUserRequest{}))
			Expect(direction).To(Equal(SchemaDirectionRequest))
		})
	})
})
