package openapi

import (
	"context"
	"os"
	"path/filepath"

	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Writer", func() {
	Describe("Initialization", func() {
		Context("with different formats", func() {
			It("should create writer with yaml format", func() {
				writer := NewWriter("yaml")
				Expect(writer.format).To(Equal("yaml"))
			})

			It("should create writer with json format", func() {
				writer := NewWriter("json")
				Expect(writer.format).To(Equal("json"))
			})

			It("should default to yaml for invalid format", func() {
				writer := NewWriter("xml")
				Expect(writer.format).To(Equal("yaml"))
			})

			It("should default to yaml for empty format", func() {
				writer := NewWriter("")
				Expect(writer.format).To(Equal("yaml"))
			})
		})
	})

	Describe("Write Spec", func() {
		Context("YAML format", func() {
			It("should write spec to file", func() {
				tmpDir := GinkgoT().TempDir()
				filePath := filepath.Join(tmpDir, "test_spec.yaml")

				spec := &openapi3.T{
					OpenAPI: "3.0.0",
					Info: &openapi3.Info{
						Title:   "Test API",
						Version: "1.0.0",
					},
					Paths: openapi3.NewPaths(),
				}

				writer := NewWriter("yaml")
				err := writer.WriteSpec(spec, filePath)
				Expect(err).NotTo(HaveOccurred())

				// Verify file exists
				_, err = os.Stat(filePath)
				Expect(err).NotTo(HaveOccurred())

				// Read and verify content
				data, err := os.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())

				// Parse YAML to verify it's valid
				var parsed map[string]interface{}
				err = yaml.Unmarshal(data, &parsed)
				Expect(err).NotTo(HaveOccurred())

				// Check basic structure
				Expect(parsed["openapi"]).To(Equal("3.0.0"))

				info, ok := parsed["info"].(map[string]interface{})
				Expect(ok).To(BeTrue())
				Expect(info["title"]).To(Equal("Test API"))
			})
		})

		Context("JSON format", func() {
			It("should write spec to file", func() {
				tmpDir := GinkgoT().TempDir()
				filePath := filepath.Join(tmpDir, "test_spec.json")

				spec := &openapi3.T{
					OpenAPI: "3.0.0",
					Info: &openapi3.Info{
						Title:   "Test API",
						Version: "1.0.0",
					},
					Paths: openapi3.NewPaths(),
				}

				writer := NewWriter("json")
				err := writer.WriteSpec(spec, filePath)
				Expect(err).NotTo(HaveOccurred())

				// Verify file exists
				_, err = os.Stat(filePath)
				Expect(err).NotTo(HaveOccurred())

				// Read and verify it's valid JSON
				data, err := os.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())

				loader := openapi3.NewLoader()
				_, err = loader.LoadFromData(data)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("with invalid spec", func() {
			It("should return error", func() {
				tmpDir := GinkgoT().TempDir()
				filePath := filepath.Join(tmpDir, "invalid_spec.yaml")

				// Create an invalid spec (missing required fields)
				spec := &openapi3.T{
					OpenAPI: "3.0.0",
					// Missing Info which is required
					Paths: openapi3.NewPaths(),
				}

				writer := NewWriter("yaml")
				err := writer.WriteSpec(spec, filePath)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("with components", func() {
			It("should write spec with components correctly", func() {
				tmpDir := GinkgoT().TempDir()
				filePath := filepath.Join(tmpDir, "spec_with_components.yaml")

				spec := &openapi3.T{
					OpenAPI: "3.0.0",
					Info: &openapi3.Info{
						Title:   "API with Components",
						Version: "1.0.0",
					},
					Paths: openapi3.NewPaths(),
					Components: &openapi3.Components{
						Schemas: openapi3.Schemas{
							"User": openapi3.NewSchemaRef("", &openapi3.Schema{
								Type: &openapi3.Types{"object"},
								Properties: map[string]*openapi3.SchemaRef{
									"id":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
									"name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
								},
							}),
						},
					},
				}

				writer := NewWriter("yaml")
				err := writer.WriteSpec(spec, filePath)
				Expect(err).NotTo(HaveOccurred())

				// Read and verify
				data, err := os.ReadFile(filePath)
				Expect(err).NotTo(HaveOccurred())

				// Parse and validate
				loader := openapi3.NewLoader()
				loader.IsExternalRefsAllowed = true
				loadedSpec, err := loader.LoadFromData(data)
				Expect(err).NotTo(HaveOccurred())

				// Validate
				err = loadedSpec.Validate(context.Background())
				Expect(err).NotTo(HaveOccurred())

				// Check that User schema exists
				Expect(loadedSpec.Components).NotTo(BeNil())
				Expect(loadedSpec.Components.Schemas).NotTo(BeNil())

				userSchema, exists := loadedSpec.Components.Schemas["User"]
				Expect(exists).To(BeTrue())
				Expect(userSchema.Value).NotTo(BeNil())
				Expect(userSchema.Value.Properties).To(HaveLen(2))
			})
		})
	})

	Describe("Validate Spec", func() {
		var writer *Writer

		BeforeEach(func() {
			writer = NewWriter("yaml")
		})

		It("should validate a valid spec", func() {
			spec := &openapi3.T{
				OpenAPI: "3.0.0",
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(),
			}

			err := writer.ValidateSpec(spec)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return error for spec missing info", func() {
			spec := &openapi3.T{
				OpenAPI: "3.0.0",
				Paths:   openapi3.NewPaths(),
			}

			err := writer.ValidateSpec(spec)
			Expect(err).To(HaveOccurred())
		})

		It("should return error for spec missing openapi version", func() {
			spec := &openapi3.T{
				Info: &openapi3.Info{
					Title:   "Test API",
					Version: "1.0.0",
				},
				Paths: openapi3.NewPaths(),
			}

			err := writer.ValidateSpec(spec)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Write Versioned Specs", func() {
		It("should write multiple versioned specs", func() {
			tmpDir := GinkgoT().TempDir()
			filenamePattern := filepath.Join(tmpDir, "api_%s.yaml")

			specs := map[string]*openapi3.T{
				"v1": {
					OpenAPI: "3.0.0",
					Info: &openapi3.Info{
						Title:   "Test API v1",
						Version: "1.0.0",
					},
					Paths: openapi3.NewPaths(),
				},
				"v2": {
					OpenAPI: "3.0.0",
					Info: &openapi3.Info{
						Title:   "Test API v2",
						Version: "2.0.0",
					},
					Paths: openapi3.NewPaths(),
				},
				"head": {
					OpenAPI: "3.0.0",
					Info: &openapi3.Info{
						Title:   "Test API HEAD",
						Version: "head",
					},
					Paths: openapi3.NewPaths(),
				},
			}

			writer := NewWriter("yaml")
			err := writer.WriteVersionedSpecs(specs, filenamePattern)
			Expect(err).NotTo(HaveOccurred())

			// Verify all files were created
			for version := range specs {
				filename := filepath.Join(tmpDir, "api_"+version+".yaml")
				_, err := os.Stat(filename)
				Expect(err).NotTo(HaveOccurred(), "spec file for version %s was not created", version)
			}

			// Verify content of one file
			v1File := filepath.Join(tmpDir, "api_v1.yaml")
			data, err := os.ReadFile(v1File)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]interface{}
			err = yaml.Unmarshal(data, &parsed)
			Expect(err).NotTo(HaveOccurred())

			info, ok := parsed["info"].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(info["title"]).To(Equal("Test API v1"))
		})
	})
})
