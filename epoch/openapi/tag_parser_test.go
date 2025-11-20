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

	DescribeTable("JSON Tags",
		func(tag, expectedFieldName string, expectedOmitempty bool) {
			fieldName, omitempty := tp.ParseJSONTag(tag)
			Expect(fieldName).To(Equal(expectedFieldName))
			Expect(omitempty).To(Equal(expectedOmitempty))
		},
		Entry("simple field name", "user_id", "user_id", false),
		Entry("field name with omitempty", "email,omitempty", "email", true),
		Entry("skip field tag", "-", "-", false),
		Entry("empty tag", "", "", false),
	)

	DescribeTable("Validation Tags",
		func(bindingTag, validateTag string, checkFunc func(*openapi3.Schema)) {
			schema := &openapi3.Schema{Type: &openapi3.Types{"string"}}
			tp.ApplyValidationTags(schema, bindingTag, validateTag)
			checkFunc(schema)
		},
		Entry("email format", "email", "", func(s *openapi3.Schema) {
			Expect(s.Format).To(Equal("email"))
		}),
		Entry("max length", "max=50", "", func(s *openapi3.Schema) {
			Expect(s.MaxLength).NotTo(BeNil())
			Expect(*s.MaxLength).To(Equal(uint64(50)))
		}),
		Entry("min length", "min=1", "", func(s *openapi3.Schema) {
			Expect(s.MinLength).To(Equal(uint64(1)))
		}),
		Entry("enum values", "oneof=active inactive pending", "", func(s *openapi3.Schema) {
			Expect(s.Enum).To(HaveLen(3))
		}),
		Entry("validate tag", "", "required,email", func(s *openapi3.Schema) {
			Expect(s.Format).To(Equal("email"))
		}),
	)

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

	DescribeTable("Required Field Detection",
		func(bindingTag, validateTag string, omitempty, expected bool) {
			isRequired := tp.IsRequired(bindingTag, validateTag, omitempty)
			Expect(isRequired).To(Equal(expected))
		},
		Entry("required from binding tag", "required", "", false, true),
		Entry("required from validate tag", "", "required", false, true),
		Entry("omitempty overrides required", "required", "", true, false),
		Entry("not required", "", "", false, false),
	)
})
