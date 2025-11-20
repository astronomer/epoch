package openapi

import (
	"reflect"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TypeParser", func() {
	var tp *TypeParser

	BeforeEach(func() {
		tp = NewTypeParser()
	})

	Describe("Primitive Types", func() {
		Context("string type", func() {
			It("should parse string correctly", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(""))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("string")).To(BeTrue())
			})
		})

		Context("integer types", func() {
			It("should parse int as integer with int64 format", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(int(0)))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("integer")).To(BeTrue())
				Expect(schemaRef.Value.Format).To(Equal("int64"))
			})

			It("should parse int64 as integer with int64 format", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(int64(0)))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("integer")).To(BeTrue())
				Expect(schemaRef.Value.Format).To(Equal("int64"))
			})

			It("should parse int32 as integer with int32 format", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(int32(0)))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("integer")).To(BeTrue())
				Expect(schemaRef.Value.Format).To(Equal("int32"))
			})
		})

		Context("floating point types", func() {
			It("should parse float32 as number with float format", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(float32(0)))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("number")).To(BeTrue())
				Expect(schemaRef.Value.Format).To(Equal("float"))
			})

			It("should parse float64 as number with double format", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(float64(0)))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("number")).To(BeTrue())
				Expect(schemaRef.Value.Format).To(Equal("double"))
			})
		})

		Context("boolean type", func() {
			It("should parse bool correctly", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(false))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("boolean")).To(BeTrue())
			})
		})

		Context("time type", func() {
			It("should parse time.Time as string with date-time format", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(time.Time{}))
				Expect(err).NotTo(HaveOccurred())
				Expect(schemaRef.Value).NotTo(BeNil())
				Expect(schemaRef.Value.Type.Is("string")).To(BeTrue())
				Expect(schemaRef.Value.Format).To(Equal("date-time"))
			})
		})
	})

	Describe("Struct Parsing", func() {
		It("should parse struct with fields", func() {
			type SimpleStruct struct {
				Name  string `json:"name" binding:"required,max=50"`
				Email string `json:"email" binding:"email"`
				Age   int    `json:"age,omitempty"`
			}

			schemaRef, err := tp.ParseType(reflect.TypeOf(SimpleStruct{}))
			Expect(err).NotTo(HaveOccurred())

			// Should create a component reference
			Expect(schemaRef.Ref).NotTo(BeEmpty())

			// Check component was created
			components := tp.GetComponents()
			Expect(components).NotTo(BeEmpty())

			schema := components["SimpleStruct"].Value
			Expect(schema).NotTo(BeNil())

			// Check properties
			Expect(schema.Properties).To(HaveLen(3))

			// Check name field
			nameSchema := schema.Properties["name"]
			Expect(nameSchema).NotTo(BeNil())
			Expect(nameSchema.Value.MaxLength).NotTo(BeNil())
			Expect(*nameSchema.Value.MaxLength).To(Equal(uint64(50)))

			// Check required fields (only 'name' has binding:"required")
			Expect(schema.Required).To(HaveLen(1))
			if len(schema.Required) > 0 {
				Expect(schema.Required[0]).To(Equal("name"))
			}
		})
	})

	Describe("Collections", func() {
		Context("slices", func() {
			It("should parse slice of structs", func() {
				type Item struct {
					ID string `json:"id"`
				}

				schemaRef, err := tp.ParseType(reflect.TypeOf([]Item{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(schemaRef.Value.Type.Is("array")).To(BeTrue())
				Expect(schemaRef.Value.Items).NotTo(BeNil())

				// Should have a $ref to Item component
				Expect(schemaRef.Value.Items.Ref).NotTo(BeEmpty())
			})
		})

		Context("maps", func() {
			It("should parse map[string]string", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(map[string]string{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(schemaRef.Value.Type.Is("object")).To(BeTrue())
				Expect(schemaRef.Value.AdditionalProperties.Schema).NotTo(BeNil())
			})

			It("should parse map[string]interface{}", func() {
				schemaRef, err := tp.ParseType(reflect.TypeOf(map[string]interface{}{}))
				Expect(err).NotTo(HaveOccurred())

				Expect(schemaRef.Value.Type.Is("object")).To(BeTrue())
				Expect(schemaRef.Value.AdditionalProperties.Schema).NotTo(BeNil())
			})
		})
	})

	Describe("Special Cases", func() {
		Context("pointers", func() {
			It("should unwrap pointer to struct", func() {
				type TestStruct struct {
					Value string
				}

				ptrType := reflect.TypeOf(&TestStruct{})
				schemaRef, err := tp.ParseType(ptrType)
				Expect(err).NotTo(HaveOccurred())

				// Should create reference to TestStruct component
				Expect(schemaRef.Ref).NotTo(BeEmpty())
			})
		})

		Context("embedded structs", func() {
			It("should promote fields from embedded struct", func() {
				type BaseStruct struct {
					ID   int    `json:"id"`
					Name string `json:"name"`
				}

				type ExtendedStruct struct {
					BaseStruct
					Email string `json:"email"`
				}

				_, err := tp.ParseType(reflect.TypeOf(ExtendedStruct{}))
				Expect(err).NotTo(HaveOccurred())

				components := tp.GetComponents()
				schema := components["ExtendedStruct"].Value

				// Should have all fields from embedded struct promoted
				Expect(schema.Properties).To(HaveLen(3))
				Expect(schema.Properties["id"]).NotTo(BeNil())
				Expect(schema.Properties["name"]).NotTo(BeNil())
				Expect(schema.Properties["email"]).NotTo(BeNil())
			})
		})

		Context("interface{} type", func() {
			It("should parse interface{} as object", func() {
				type TestStruct struct {
					Data interface{} `json:"data"`
				}

				_, err := tp.ParseType(reflect.TypeOf(TestStruct{}))
				Expect(err).NotTo(HaveOccurred())

				components := tp.GetComponents()
				schema := components["TestStruct"].Value

				dataField := schema.Properties["data"]
				Expect(dataField).NotTo(BeNil())

				// interface{} should be type: object
				Expect(dataField.Value.Type.Is("object")).To(BeTrue())
			})
		})

		Context("caching", func() {
			It("should cache parsed types", func() {
				type TestStruct struct {
					Value string
				}

				typ := reflect.TypeOf(TestStruct{})

				// Parse once
				ref1, err := tp.ParseType(typ)
				Expect(err).NotTo(HaveOccurred())

				// Parse again - should use cache
				ref2, err := tp.ParseType(typ)
				Expect(err).NotTo(HaveOccurred())

				// Should return same reference
				Expect(ref1.Ref).To(Equal(ref2.Ref))

				// Reset and parse again
				tp.Reset()
				ref3, err := tp.ParseType(typ)
				Expect(err).NotTo(HaveOccurred())

				// After reset, should create new component
				Expect(ref3.Ref).To(Equal(ref1.Ref))
			})
		})

		Context("circular references", func() {
			It("should handle circular references without panic", func() {
				type Node struct {
					Value string
					Next  *Node
				}

				_, err := tp.ParseType(reflect.TypeOf(Node{}))
				Expect(err).NotTo(HaveOccurred())

				// Should not panic and should create component
				components := tp.GetComponents()
				Expect(components).NotTo(BeEmpty())
			})
		})
	})
})
