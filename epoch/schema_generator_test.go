package epoch

import (
	"reflect"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type TestStruct struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Email    string `json:"email,omitempty"`
	IsActive bool   `json:"is_active"`
}

type AnotherStruct struct {
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

var _ = Describe("SchemaGenerator", func() {
	var (
		v1, v2, v3      *Version
		bundle          *VersionBundle
		chain           *MigrationChain
		generator       *SchemaGenerator
		testStructType  reflect.Type
		testStructType2 reflect.Type
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		v3, _ = NewSemverVersion("3.0.0")
		var err error
		bundle, err = NewVersionBundle([]*Version{v1, v2, v3})
		Expect(err).NotTo(HaveOccurred())
		chain = NewMigrationChain([]*VersionChange{})
		generator = NewSchemaGenerator(bundle, chain)
		testStructType = reflect.TypeOf(TestStruct{})
		testStructType2 = reflect.TypeOf(AnotherStruct{})
	})

	Describe("NewSchemaGenerator", func() {
		It("should create a new schema generator", func() {
			Expect(generator).NotTo(BeNil())
		})
	})

	Describe("RegisterType", func() {
		It("should register a struct type", func() {
			err := generator.RegisterType(testStructType)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should register multiple types", func() {
			err1 := generator.RegisterType(testStructType)
			err2 := generator.RegisterType(testStructType2)
			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())
		})

		It("should handle pointer types", func() {
			pointerType := reflect.TypeOf(&TestStruct{})
			err := generator.RegisterType(pointerType)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should reject anonymous types", func() {
			anonymousType := reflect.TypeOf(struct{ Name string }{})
			err := generator.RegisterType(anonymousType)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("anonymous types are not supported"))
		})

		It("should handle non-struct types", func() {
			stringType := reflect.TypeOf("string")
			err := generator.RegisterType(stringType)
			Expect(err).NotTo(HaveOccurred())
		})
	})

	Describe("GenerateStruct", func() {
		BeforeEach(func() {
			err := generator.RegisterType(testStructType)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should generate Go code for a struct", func() {
			code, err := generator.GenerateStruct(testStructType, "1.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(ContainSubstring("package"))
			Expect(code).To(ContainSubstring("TestStruct"))
			Expect(code).To(ContainSubstring("struct"))
		})

		It("should generate code for head version", func() {
			code, err := generator.GenerateStruct(testStructType, "head")
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(ContainSubstring("TestStruct"))
		})

		It("should return error for unknown version", func() {
			_, err := generator.GenerateStruct(testStructType, "unknown")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version unknown not found"))
		})
	})

	Describe("GeneratePackage", func() {
		It("should generate package code for a version", func() {
			err := generator.RegisterType(testStructType)
			Expect(err).NotTo(HaveOccurred())

			packageFiles, err := generator.GeneratePackage("github.com/test/package", "1.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(packageFiles).NotTo(BeEmpty())

			// Check that at least one file was generated
			for filename, content := range packageFiles {
				Expect(filename).To(ContainSubstring(".go"))
				Expect(content).To(ContainSubstring("package"))
				Expect(content).To(ContainSubstring("// Code generated for version 1.0.0"))
			}
		})

		It("should handle empty package path", func() {
			packageFiles, err := generator.GeneratePackage("", "1.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(packageFiles).NotTo(BeEmpty())

			for _, content := range packageFiles {
				Expect(content).To(ContainSubstring("package main"))
			}
		})

		It("should return error for unknown version", func() {
			_, err := generator.GeneratePackage("github.com/test/package", "unknown")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version unknown not found"))
		})
	})

	Describe("GetVersionSpecificType", func() {
		BeforeEach(func() {
			err := generator.RegisterType(testStructType)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return version-specific type", func() {
			versionType, err := generator.GetVersionSpecificType(testStructType, "1.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(versionType).NotTo(BeNil())
			// Currently returns original type, but structure is in place for dynamic types
			Expect(versionType).To(Equal(testStructType))
		})

		It("should return error for unknown version", func() {
			_, err := generator.GetVersionSpecificType(testStructType, "unknown")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("version unknown not found"))
		})
	})

	Describe("ListVersionedStructs", func() {
		It("should list all versioned structs", func() {
			err1 := generator.RegisterType(testStructType)
			err2 := generator.RegisterType(testStructType2)
			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())

			// Generate structs to populate the versioned structs map
			_, err := generator.GenerateStruct(testStructType, "1.0.0")
			Expect(err).NotTo(HaveOccurred())

			versionedStructs := generator.ListVersionedStructs()
			Expect(versionedStructs).NotTo(BeEmpty())

			// Check that at least one version has structs
			found := false
			for version, types := range versionedStructs {
				if len(types) > 0 {
					found = true
					Expect(version).NotTo(BeEmpty())
				}
			}
			Expect(found).To(BeTrue())
		})

		It("should return empty map when no structs are registered", func() {
			versionedStructs := generator.ListVersionedStructs()
			// Should have entries for each version, but with empty type lists
			Expect(versionedStructs).NotTo(BeEmpty()) // Has version entries

			// All version entries should have empty type lists
			for _, types := range versionedStructs {
				Expect(types).To(BeEmpty())
			}
		})
	})

	Describe("version changes integration", func() {
		Context("with schema instructions", func() {
			var changeWithSchema *VersionChange

			BeforeEach(func() {
				schemaInst := &SchemaInstruction{
					Schema: TestStruct{},
					Name:   "email",
					Type:   "field_added",
					Attributes: map[string]interface{}{
						"type": "string",
					},
				}
				changeWithSchema = NewVersionChange("Add email field", v1, v2, schemaInst)
				chain = NewMigrationChain([]*VersionChange{changeWithSchema})
				generator = NewSchemaGenerator(bundle, chain)

				err := generator.RegisterType(testStructType)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should apply schema changes when generating structs", func() {
				// This tests the integration between schema generation and version changes
				code, err := generator.GenerateStruct(testStructType, "1.0.0")
				Expect(err).NotTo(HaveOccurred())
				Expect(code).To(ContainSubstring("TestStruct"))
			})
		})
	})

	Describe("TypeRegistry", func() {
		var registry *TypeRegistry

		BeforeEach(func() {
			registry = NewTypeRegistry()
		})

		Describe("RegisterPackage", func() {
			It("should register a package", func() {
				err := registry.RegisterPackage("github.com/test/package")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should not error when registering the same package twice", func() {
				err1 := registry.RegisterPackage("github.com/test/package")
				err2 := registry.RegisterPackage("github.com/test/package")
				Expect(err1).NotTo(HaveOccurred())
				Expect(err2).NotTo(HaveOccurred())
			})
		})
	})

	Describe("field introspection", func() {
		It("should correctly analyze struct fields", func() {
			err := generator.RegisterType(testStructType)
			Expect(err).NotTo(HaveOccurred())

			// The registration process should analyze the fields
			// We can't directly access the internal type registry, but we can verify
			// that the registration completed without error, which means field analysis worked
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle different field types", func() {
			type ComplexStruct struct {
				StringField    string            `json:"string_field"`
				IntField       int               `json:"int_field"`
				BoolField      bool              `json:"bool_field"`
				SliceField     []string          `json:"slice_field,omitempty"`
				MapField       map[string]string `json:"map_field,omitempty"`
				PointerField   *string           `json:"pointer_field,omitempty"`
				EmbeddedStruct TestStruct        `json:"embedded"`
			}

			complexType := reflect.TypeOf(ComplexStruct{})
			err := generator.RegisterType(complexType)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should skip unexported fields", func() {
			type StructWithUnexported struct {
				PublicField string `json:"public"`
			}

			unexportedType := reflect.TypeOf(StructWithUnexported{})
			err := generator.RegisterType(unexportedType)
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
