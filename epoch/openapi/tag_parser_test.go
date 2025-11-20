package openapi

import (
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("TagParser", func() {
	var tp *TagParser

	BeforeEach(func() {
		tp = NewTagParser()
	})

	Describe("JSON Tags", func() {
		It("should parse simple field name", func() {
			fieldName, omitempty := tp.ParseJSONTag("user_id")
			Expect(fieldName).To(Equal("user_id"))
			Expect(omitempty).To(BeFalse())
		})

		It("should parse field name with omitempty", func() {
			fieldName, omitempty := tp.ParseJSONTag("email,omitempty")
			Expect(fieldName).To(Equal("email"))
			Expect(omitempty).To(BeTrue())
		})

		It("should handle skip field tag", func() {
			fieldName, omitempty := tp.ParseJSONTag("-")
			Expect(fieldName).To(Equal("-"))
			Expect(omitempty).To(BeFalse())
		})

		It("should handle empty tag", func() {
			fieldName, omitempty := tp.ParseJSONTag("")
			Expect(fieldName).To(Equal(""))
			Expect(omitempty).To(BeFalse())
		})
	})

	Describe("Validation Tags", func() {
		Context("email format", func() {
			It("should apply email format", func() {
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyValidationTags(schema, "email", "")
				Expect(schema.Format).To(Equal("email"))
			})
		})

		Context("max length", func() {
			It("should apply max length constraint", func() {
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyValidationTags(schema, "max=50", "")
				Expect(schema.MaxLength).NotTo(BeNil())
				Expect(*schema.MaxLength).To(Equal(uint64(50)))
			})
		})

		Context("min length", func() {
			It("should apply min length constraint", func() {
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyValidationTags(schema, "min=1", "")
				Expect(schema.MinLength).To(Equal(uint64(1)))
			})
		})

		Context("enum values", func() {
			It("should apply enum values from oneof", func() {
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyValidationTags(schema, "oneof=active inactive pending", "")
				Expect(schema.Enum).To(HaveLen(3))
			})
		})

		Context("validate tag", func() {
			It("should apply validation from validate tag", func() {
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyValidationTags(schema, "", "required,email")
				Expect(schema.Format).To(Equal("email"))
			})
		})
	})

	Describe("Common Tags", func() {
		var testStructType reflect.Type

		BeforeEach(func() {
			type testStruct struct {
				Field1 string `json:"field1" example:"test value"`
				Field2 string `json:"field2" enums:"A,B,C"`
				Field3 string `json:"field3" format:"date-time"`
				Field4 string `json:"field4" description:"A test field"`
			}
			testStructType = reflect.TypeOf(testStruct{})
		})

		Context("example tag", func() {
			It("should apply example value", func() {
				field, _ := testStructType.FieldByName("Field1")
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyCommonTags(schema, field)
				Expect(schema.Example).To(Equal("test value"))
			})
		})

		Context("enums tag", func() {
			It("should apply enum values", func() {
				field, _ := testStructType.FieldByName("Field2")
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyCommonTags(schema, field)
				Expect(schema.Enum).To(HaveLen(3))
			})
		})

		Context("format tag", func() {
			It("should apply format", func() {
				field, _ := testStructType.FieldByName("Field3")
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyCommonTags(schema, field)
				Expect(schema.Format).To(Equal("date-time"))
			})
		})

		Context("description tag", func() {
			It("should apply description", func() {
				field, _ := testStructType.FieldByName("Field4")
				schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
				tp.ApplyCommonTags(schema, field)
				Expect(schema.Description).To(Equal("A test field"))
			})
		})
	})

	Describe("Required Field Detection", func() {
		It("should detect required from binding tag", func() {
			isRequired := tp.IsRequired("required", "", false)
			Expect(isRequired).To(BeTrue())
		})

		It("should detect required from validate tag", func() {
			isRequired := tp.IsRequired("", "required", false)
			Expect(isRequired).To(BeTrue())
		})

		It("should respect omitempty override", func() {
			isRequired := tp.IsRequired("required", "", true)
			Expect(isRequired).To(BeFalse())
		})

		It("should return false when not required", func() {
			isRequired := tp.IsRequired("", "", false)
			Expect(isRequired).To(BeFalse())
		})
	})
})
