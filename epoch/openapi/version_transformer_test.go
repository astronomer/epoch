package openapi

import (
	"reflect"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test types for version transformation tests
type TestUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Status string `json:"status"`
}

var _ = Describe("VersionTransformer", func() {
	Describe("Initialization", func() {
		It("should create a new VersionTransformer", func() {
			v1, _ := epoch.NewDateVersion("2024-01-01")
			vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

			transformer := NewVersionTransformer(vb)

			Expect(transformer).NotTo(BeNil())
			Expect(transformer.versionBundle).To(Equal(vb))
			Expect(transformer.typeParser).NotTo(BeNil())
		})
	})

	Describe("Schema Transformations", func() {
		Context("HEAD version", func() {
			It("should return schema unchanged for HEAD version", func() {
				head := epoch.NewHeadVersion()
				vb, _ := epoch.NewVersionBundle([]*epoch.Version{head})
				transformer := NewVersionTransformer(vb)

				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":    openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"email": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					head,
					SchemaDirectionResponse,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties).To(HaveLen(3))
			})
		})

		Context("Response transformations", func() {
			It("should remove field when migrating to previous version", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				// Create version change: v1 -> v2 adds email
				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Add email field").
					ForType(TestUser{}).
					ResponseToPreviousVersion().
					RemoveField("email").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				v2.Changes = []epoch.VersionChangeInterface{change}

				transformer := NewVersionTransformer(vb)

				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":    openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"email": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionResponse,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties["email"]).To(BeNil())
				Expect(result.Properties["id"]).NotTo(BeNil())
				Expect(result.Properties["name"]).NotTo(BeNil())
				Expect(result.Properties).To(HaveLen(2))
			})

			It("should rename field when specified", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				// v1 -> v2 renames "full_name" to "name"
				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Rename name to full_name").
					ForType(TestUser{}).
					ResponseToPreviousVersion().
					RenameField("name", "full_name").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				v2.Changes = []epoch.VersionChangeInterface{change}

				transformer := NewVersionTransformer(vb)

				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionResponse,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties["name"]).To(BeNil())
				Expect(result.Properties["full_name"]).NotTo(BeNil())
				Expect(result.Properties).To(HaveLen(2))
			})

			It("should handle field addition transformations", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Add status field").
					ForType(TestUser{}).
					ResponseToPreviousVersion().
					RemoveField("status").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				v2.Changes = []epoch.VersionChangeInterface{change}

				transformer := NewVersionTransformer(vb)

				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":     openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"status": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionResponse,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties["status"]).To(BeNil())
			})

			It("should handle multiple changes in one migration", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				// v1 -> v2: adds email, phone, and status
				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Add multiple fields").
					ForType(TestUser{}).
					ResponseToPreviousVersion().
					RemoveField("email").
					RemoveField("phone").
					RemoveField("status").
					Build()

				v2.Changes = []epoch.VersionChangeInterface{change}

				vb, err := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				Expect(err).NotTo(HaveOccurred())

				transformer := NewVersionTransformer(vb)

				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":     openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"email":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"phone":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"status": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionResponse,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties).To(HaveLen(2))
				Expect(result.Properties["email"]).To(BeNil())
				Expect(result.Properties["phone"]).To(BeNil())
				Expect(result.Properties["status"]).To(BeNil())
			})
		})

		Context("Request transformations", func() {
			It("should add field when transforming request schema for older version", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				// v1 -> v2: add "email" field (v2/HEAD has it, v1 doesn't)
				// When generating v1 request schema, we start from v2 and remove email
				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Add email field").
					ForType(TestUser{}).
					RequestToNextVersion().
					AddField("email", "unknown@example.com").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				v2.Changes = []epoch.VersionChangeInterface{change}

				transformer := NewVersionTransformer(vb)

				// HEAD/v2 schema (has email)
				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":    openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"email": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				// Transform for v1 (should remove email since v1 doesn't have it)
				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionRequest,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties["email"]).To(BeNil())
				Expect(result.Properties["id"]).NotTo(BeNil())
				Expect(result.Properties["name"]).NotTo(BeNil())
				Expect(result.Properties).To(HaveLen(2))
			})

			It("should rename field when transforming request schema", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				// v1 -> v2: rename "name" to "full_name"
				// HEAD has "full_name", v1 has "name"
				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Rename name to full_name").
					ForType(TestUser{}).
					RequestToNextVersion().
					RenameField("name", "full_name").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				v2.Changes = []epoch.VersionChangeInterface{change}

				transformer := NewVersionTransformer(vb)

				// HEAD/v2 schema has "full_name"
				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":        openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"full_name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				// Transform for v1 (should rename full_name -> name)
				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionRequest,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties["full_name"]).To(BeNil())
				Expect(result.Properties["name"]).NotTo(BeNil())
				Expect(result.Properties).To(HaveLen(2))
			})

			It("should remove field when transforming request schema for older version", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				// v1 -> v2: remove deprecated field (v1 has it, v2 doesn't)
				// This means RequestToNextVersion removes it when going from v1 to v2
				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Remove deprecated field").
					ForType(TestUser{}).
					RequestToNextVersion().
					RemoveField("deprecated_field").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				v2.Changes = []epoch.VersionChangeInterface{change}

				transformer := NewVersionTransformer(vb)

				// HEAD/v2 schema (doesn't have deprecated_field)
				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				// Transform for v1 (should add deprecated_field back since v1 has it)
				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionRequest,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties["deprecated_field"]).NotTo(BeNil())
				Expect(result.Properties["id"]).NotTo(BeNil())
				Expect(result.Properties["name"]).NotTo(BeNil())
				Expect(result.Properties).To(HaveLen(3))
			})

			It("should handle multiple request changes in one migration", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				// v1 -> v2: multiple changes
				// - v1 has "old_status", v2 has "status" (rename)
				// - v1 doesn't have "email", v2 has "email" (add)
				// - v1 doesn't have "phone", v2 has "phone" (add)
				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Multiple request changes").
					ForType(TestUser{}).
					RequestToNextVersion().
					AddField("email", "unknown@example.com").
					AddField("phone", "").
					RenameField("old_status", "status").
					Build()

				v2.Changes = []epoch.VersionChangeInterface{change}

				vb, err := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				Expect(err).NotTo(HaveOccurred())

				transformer := NewVersionTransformer(vb)

				// HEAD/v2 schema
				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":     openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"email":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"phone":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
						"status": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				// Transform for v1 (should reverse all changes)
				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionRequest,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties).To(HaveLen(3)) // id, name, old_status
				Expect(result.Properties["email"]).To(BeNil())
				Expect(result.Properties["phone"]).To(BeNil())
				Expect(result.Properties["status"]).To(BeNil())
				Expect(result.Properties["old_status"]).NotTo(BeNil())
			})
		})

		Context("No operations for type", func() {
			It("should return unchanged schema when no operations apply", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				type DifferentType struct {
					Field string
				}

				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Change for different type").
					ForType(DifferentType{}).
					ResponseToPreviousVersion().
					RemoveField("field").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				v2.Changes = []epoch.VersionChangeInterface{change}

				transformer := NewVersionTransformer(vb)

				baseSchema := &openapi3.Schema{
					Type: &openapi3.Types{"object"},
					Properties: map[string]*openapi3.SchemaRef{
						"id":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
						"name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
				}

				result, err := transformer.TransformSchemaForVersion(
					baseSchema,
					reflect.TypeOf(TestUser{}),
					v1,
					SchemaDirectionResponse,
				)

				Expect(err).NotTo(HaveOccurred())
				Expect(result.Properties).To(HaveLen(len(baseSchema.Properties)))
			})
		})
	})

	Describe("Field Operations", func() {
		var (
			transformer *VersionTransformer
			schema      *openapi3.Schema
		)

		BeforeEach(func() {
			v1, _ := epoch.NewDateVersion("2024-01-01")
			vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
			transformer = NewVersionTransformer(vb)

			schema = &openapi3.Schema{
				Type:       &openapi3.Types{"object"},
				Properties: map[string]*openapi3.SchemaRef{},
			}
		})

		Context("Add field", func() {
			It("should add field without required flag", func() {
				fieldSchema := openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}})

				transformer.AddFieldToSchema(schema, "test_field", fieldSchema, false)

				Expect(schema.Properties["test_field"]).NotTo(BeNil())
				Expect(schema.Required).To(HaveLen(0))
			})

			It("should add field with required flag", func() {
				fieldSchema := openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}})

				transformer.AddFieldToSchema(schema, "required_field", fieldSchema, true)

				Expect(schema.Properties["required_field"]).NotTo(BeNil())
				Expect(schema.Required).To(HaveLen(1))
				Expect(schema.Required[0]).To(Equal("required_field"))
			})
		})

		Context("Remove field", func() {
			It("should remove field from properties and required array", func() {
				schema.Properties = map[string]*openapi3.SchemaRef{
					"field1": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					"field2": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
				}
				schema.Required = []string{"field1", "field2"}

				transformer.RemoveFieldFromSchema(schema, "field1")

				Expect(schema.Properties["field1"]).To(BeNil())
				Expect(schema.Required).To(HaveLen(1))
				Expect(schema.Required[0]).To(Equal("field2"))
			})
		})

		Context("Rename field", func() {
			It("should rename field preserving schema and required status", func() {
				stringSchema := openapi3.NewSchemaRef("", &openapi3.Schema{
					Type:   &openapi3.Types{"string"},
					Format: "email",
				})

				schema.Properties = map[string]*openapi3.SchemaRef{
					"old_name": stringSchema,
					"field2":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
				}
				schema.Required = []string{"old_name", "field2"}

				transformer.RenameFieldInSchema(schema, "old_name", "new_name")

				Expect(schema.Properties["old_name"]).To(BeNil())
				Expect(schema.Properties["new_name"]).NotTo(BeNil())
				Expect(schema.Properties["new_name"].Value.Format).To(Equal("email"))

				foundNewName := false
				for _, req := range schema.Required {
					if req == "old_name" {
						Fail("old name should not be in required array")
					}
					if req == "new_name" {
						foundNewName = true
					}
				}
				Expect(foundNewName).To(BeTrue())
			})
		})
	})

	Describe("Utilities", func() {
		Context("CloneSchema", func() {
			It("should create a deep copy of schema", func() {
				original := &openapi3.Schema{
					Type:        &openapi3.Types{"object"},
					Format:      "custom",
					Description: "Test schema",
					Properties: map[string]*openapi3.SchemaRef{
						"field1": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
					},
					Required: []string{"field1"},
				}

				clone := CloneSchema(original)

				// Verify clone is different object
				Expect(clone).NotTo(BeIdenticalTo(original))

				// Verify properties are copied
				Expect(clone.Format).To(Equal(original.Format))
				Expect(clone.Description).To(Equal(original.Description))
				Expect(clone.Properties).To(HaveLen(len(original.Properties)))
				Expect(clone.Required).To(HaveLen(len(original.Required)))

				// Modify clone and verify original is unchanged
				clone.Properties["field2"] = openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}})
				Expect(original.Properties).To(HaveLen(1))
			})
		})

		Context("changeAppliesToType", func() {
			It("should return true when change targets the type and direction", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Test change").
					ForType(TestUser{}).
					ResponseToPreviousVersion().
					RemoveField("email").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				transformer := NewVersionTransformer(vb)

				// Should apply to TestUser
				Expect(transformer.changeAppliesToType(change, reflect.TypeOf(TestUser{}), SchemaDirectionResponse)).To(BeTrue())
			})

			It("should return false for different type", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Test change").
					ForType(TestUser{}).
					ResponseToPreviousVersion().
					RemoveField("email").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				transformer := NewVersionTransformer(vb)

				type OtherType struct {
					ID int
				}

				Expect(transformer.changeAppliesToType(change, reflect.TypeOf(OtherType{}), SchemaDirectionResponse)).To(BeFalse())
			})

			It("should return false for different direction", func() {
				v1, _ := epoch.NewDateVersion("2024-01-01")
				v2, _ := epoch.NewDateVersion("2024-06-01")

				change := epoch.NewVersionChangeBuilder(v1, v2).
					Description("Test change").
					ForType(TestUser{}).
					ResponseToPreviousVersion().
					RemoveField("email").
					Build()

				vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1, v2})
				transformer := NewVersionTransformer(vb)

				Expect(transformer.changeAppliesToType(change, reflect.TypeOf(TestUser{}), SchemaDirectionRequest)).To(BeFalse())
			})
		})
	})
})
