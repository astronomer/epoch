package openapi

import (
	"context"
	"reflect"

	"github.com/getkin/kin-openapi/openapi3"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type exampleTagStruct struct {
	Name        string  `json:"name" example:"My token"`
	Description string  `json:"description" example:"This is my API token"`
	ExpiryDays  *int    `json:"expiryDays" binding:"omitempty,min=1,max=3650" example:"30"`
	Enabled     bool    `json:"enabled" example:"true"`
	Ratio       float64 `json:"ratio" example:"1.5"`
}

var _ = Describe("example struct tags", func() {
	properties := func() openapi3.Schemas {
		tp := NewTypeParser()
		_, err := tp.ParseType(reflect.TypeOf(exampleTagStruct{}))
		Expect(err).NotTo(HaveOccurred())
		schema := tp.components["exampleTagStruct"]
		Expect(schema).NotTo(BeNil())
		return schema.Value.Properties
	}

	It("keep each field's own example", func() {
		props := properties()
		Expect(props["name"].Value.Example).To(Equal("My token"))
		Expect(props["description"].Value.Example).To(Equal("This is my API token"))
	})

	It("coerce examples to the declared schema type", func() {
		props := properties()
		Expect(props["expiryDays"].Value.Example).To(BeEquivalentTo(30))
		Expect(props["enabled"].Value.Example).To(Equal(true))
		Expect(props["ratio"].Value.Example).To(BeEquivalentTo(1.5))
	})

	It("produce a spec that passes OpenAPI validation", func() {
		props := properties()
		for name, prop := range props {
			Expect(prop.Value.Validate(context.Background())).To(Succeed(), "property %q", name)
		}
	})
})
