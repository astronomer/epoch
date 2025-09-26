package cadwyn

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionBundle", func() {
	var (
		v1, v2, v3 *Version
		bundle     *VersionBundle
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		v3, _ = NewSemverVersion("3.0.0")
	})

	Describe("NewVersionBundle", func() {
		Context("with regular versions", func() {
			BeforeEach(func() {
				bundle = NewVersionBundle([]*Version{v1, v2, v3})
			})

			It("should create a bundle with head version", func() {
				Expect(bundle.GetHeadVersion()).NotTo(BeNil())
				Expect(bundle.GetHeadVersion().IsHead).To(BeTrue())
			})

			It("should store all versions", func() {
				versions := bundle.GetVersions()
				Expect(versions).To(HaveLen(3))
				Expect(versions).To(ContainElement(v1))
				Expect(versions).To(ContainElement(v2))
				Expect(versions).To(ContainElement(v3))
			})

			It("should create version values", func() {
				values := bundle.GetVersionValues()
				Expect(values).To(ContainElement("1.0.0"))
				Expect(values).To(ContainElement("2.0.0"))
				Expect(values).To(ContainElement("3.0.0"))
			})
		})

		Context("with head version first", func() {
			It("should use the provided head version", func() {
				head := NewHeadVersion()
				bundle = NewVersionBundle([]*Version{head, v1, v2})
				Expect(bundle.GetHeadVersion()).To(Equal(head))
				Expect(bundle.GetVersions()).To(HaveLen(2))
			})
		})

		Context("with empty versions", func() {
			It("should panic", func() {
				Expect(func() {
					NewVersionBundle([]*Version{})
				}).To(Panic())
			})
		})

		Context("with duplicate versions", func() {
			It("should panic", func() {
				v1Duplicate, _ := NewSemverVersion("1.0.0")
				Expect(func() {
					NewVersionBundle([]*Version{v1, v1Duplicate})
				}).To(Panic())
			})
		})
	})

	Describe("ParseVersion", func() {
		BeforeEach(func() {
			bundle = NewVersionBundle([]*Version{v1, v2, v3})
		})

		Context("with valid version strings", func() {
			It("should return the correct version", func() {
				version, err := bundle.ParseVersion("1.0.0")
				Expect(err).NotTo(HaveOccurred())
				Expect(version).To(Equal(v1))
			})

			It("should return head version for 'head'", func() {
				version, err := bundle.ParseVersion("head")
				Expect(err).NotTo(HaveOccurred())
				Expect(version.IsHead).To(BeTrue())
			})

			It("should return head version for empty string", func() {
				version, err := bundle.ParseVersion("")
				Expect(err).NotTo(HaveOccurred())
				Expect(version.IsHead).To(BeTrue())
			})
		})

		Context("with invalid version strings", func() {
			It("should return an error", func() {
				_, err := bundle.ParseVersion("4.0.0")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unknown version"))
			})
		})
	})

	Describe("GetClosestLesserVersion", func() {
		BeforeEach(func() {
			bundle = NewVersionBundle([]*Version{v1, v2, v3})
		})

		It("should find closest lesser version", func() {
			version, err := bundle.GetClosestLesserVersion("2.5.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("2.0.0"))
		})

		It("should return error when no lesser version exists", func() {
			_, err := bundle.GetClosestLesserVersion("0.5.0")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no version found"))
		})
	})

	Describe("IsVersionDefined", func() {
		BeforeEach(func() {
			bundle = NewVersionBundle([]*Version{v1, v2, v3})
		})

		It("should return true for defined versions", func() {
			Expect(bundle.IsVersionDefined("1.0.0")).To(BeTrue())
			Expect(bundle.IsVersionDefined("head")).To(BeTrue())
		})

		It("should return false for undefined versions", func() {
			Expect(bundle.IsVersionDefined("4.0.0")).To(BeFalse())
		})
	})

	Describe("GetVersionedSchemas", func() {
		It("should return empty map when no schemas are defined", func() {
			bundle = NewVersionBundle([]*Version{v1})
			schemas := bundle.GetVersionedSchemas()
			Expect(schemas).NotTo(BeNil())
		})
	})

	Describe("GetVersionedEnums", func() {
		It("should return empty map when no enums are defined", func() {
			bundle = NewVersionBundle([]*Version{v1})
			enums := bundle.GetVersionedEnums()
			Expect(enums).NotTo(BeNil())
		})
	})

	Describe("Iterator", func() {
		BeforeEach(func() {
			bundle = NewVersionBundle([]*Version{v1, v2, v3})
		})

		It("should return all versions", func() {
			versions := bundle.Iterator()
			Expect(versions).To(HaveLen(3))
			Expect(versions).To(ContainElement(v1))
			Expect(versions).To(ContainElement(v2))
			Expect(versions).To(ContainElement(v3))
		})
	})
})
