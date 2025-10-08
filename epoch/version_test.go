package epoch

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Version", func() {
	Describe("NewDateVersion", func() {
		Context("with valid date strings", func() {
			It("should create a date-based version", func() {
				version, err := NewDateVersion("2023-01-15")
				Expect(err).NotTo(HaveOccurred())
				Expect(version.Type).To(Equal(VersionTypeDate))
				Expect(version.Raw).To(Equal("2023-01-15"))
				Expect(version.IsHead).To(BeFalse())
				Expect(version.Date).NotTo(BeNil())
				Expect(version.Date.Year()).To(Equal(2023))
				Expect(version.Date.Month()).To(Equal(time.January))
				Expect(version.Date.Day()).To(Equal(15))
			})
		})

		Context("with invalid date strings", func() {
			It("should return an error", func() {
				_, err := NewDateVersion("invalid-date")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid date format"))
			})

			It("should return an error for wrong format", func() {
				_, err := NewDateVersion("01-15-2023")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("NewSemverVersion", func() {
		Context("with valid semver strings", func() {
			It("should create a semantic version with major.minor.patch", func() {
				version, err := NewSemverVersion("1.2.3")
				Expect(err).NotTo(HaveOccurred())
				Expect(version.Type).To(Equal(VersionTypeSemver))
				Expect(version.Raw).To(Equal("1.2.3"))
				Expect(version.Major).To(Equal(1))
				Expect(version.Minor).To(Equal(2))
				Expect(version.Patch).To(Equal(3))
				Expect(version.IsHead).To(BeFalse())
			})

			It("should create a semantic version with major.minor (patch defaults to 0)", func() {
				version, err := NewSemverVersion("2.1")
				Expect(err).NotTo(HaveOccurred())
				Expect(version.Type).To(Equal(VersionTypeSemver))
				Expect(version.Raw).To(Equal("2.1"))
				Expect(version.Major).To(Equal(2))
				Expect(version.Minor).To(Equal(1))
				Expect(version.Patch).To(Equal(0))
			})

			It("should handle version with 'v' prefix", func() {
				version, err := NewSemverVersion("v1.0.0")
				Expect(err).NotTo(HaveOccurred())
				Expect(version.Major).To(Equal(1))
				Expect(version.Minor).To(Equal(0))
				Expect(version.Patch).To(Equal(0))
			})
		})

		Context("with invalid semver strings", func() {
			It("should return an error for invalid format", func() {
				_, err := NewSemverVersion("invalid")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid semver format"))
			})

			It("should return an error for single number", func() {
				_, err := NewSemverVersion("1")
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("NewStringVersion", func() {
		It("should create a string-based version", func() {
			version := NewStringVersion("alpha")
			Expect(version.Type).To(Equal(VersionTypeString))
			Expect(version.Raw).To(Equal("alpha"))
			Expect(version.IsHead).To(BeFalse())
		})

		It("should handle empty string", func() {
			version := NewStringVersion("")
			Expect(version.Type).To(Equal(VersionTypeString))
			Expect(version.Raw).To(Equal(""))
		})
	})

	Describe("NewHeadVersion", func() {
		It("should create a head version", func() {
			version := NewHeadVersion()
			Expect(version.Type).To(Equal(VersionTypeHead))
			Expect(version.Raw).To(Equal("head"))
			Expect(version.IsHead).To(BeTrue())
		})
	})

	Describe("NewVersion", func() {
		It("should auto-detect date version", func() {
			version, err := NewVersion("2023-01-01")
			Expect(err).NotTo(HaveOccurred())
			Expect(version.Type).To(Equal(VersionTypeDate))
		})

		It("should auto-detect semver version", func() {
			version, err := NewVersion("1.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(version.Type).To(Equal(VersionTypeSemver))
		})

		It("should fallback to string version", func() {
			version, err := NewVersion("alpha")
			Expect(err).NotTo(HaveOccurred())
			Expect(version.Type).To(Equal(VersionTypeString))
		})
	})

	Describe("String", func() {
		It("should return 'head' for head versions", func() {
			version := NewHeadVersion()
			Expect(version.String()).To(Equal("head"))
		})

		It("should return raw value for non-head versions", func() {
			version := NewStringVersion("alpha")
			Expect(version.String()).To(Equal("alpha"))
		})
	})

	Describe("Compare", func() {
		var v1, v2, v3, head *Version

		BeforeEach(func() {
			v1, _ = NewSemverVersion("1.0.0")
			v2, _ = NewSemverVersion("2.0.0")
			v3, _ = NewSemverVersion("1.1.0")
			head = NewHeadVersion()
		})

		It("should compare head versions correctly", func() {
			Expect(head.Compare(v1)).To(Equal(1))   // head > v1
			Expect(v1.Compare(head)).To(Equal(-1))  // v1 < head
			Expect(head.Compare(head)).To(Equal(0)) // head == head
		})

		It("should compare semantic versions correctly", func() {
			Expect(v1.Compare(v2)).To(Equal(-1)) // 1.0.0 < 2.0.0
			Expect(v2.Compare(v1)).To(Equal(1))  // 2.0.0 > 1.0.0
			Expect(v1.Compare(v3)).To(Equal(-1)) // 1.0.0 < 1.1.0
		})

		It("should compare equal versions", func() {
			v1Copy, _ := NewSemverVersion("1.0.0")
			Expect(v1.Compare(v1Copy)).To(Equal(0))
		})
	})

	Describe("IsOlderThan", func() {
		It("should return true when version is older", func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")
			Expect(v1.IsOlderThan(v2)).To(BeTrue())
			Expect(v2.IsOlderThan(v1)).To(BeFalse())
		})
	})

	Describe("IsNewerThan", func() {
		It("should return true when version is newer", func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")
			Expect(v2.IsNewerThan(v1)).To(BeTrue())
			Expect(v1.IsNewerThan(v2)).To(BeFalse())
		})
	})

	Describe("Equal", func() {
		It("should return true for equal versions", func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("1.0.0")
			Expect(v1.Equal(v2)).To(BeTrue())
		})

		It("should return false for different versions", func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")
			Expect(v1.Equal(v2)).To(BeFalse())
		})
	})

	Describe("VersionType String", func() {
		It("should return correct string representations", func() {
			Expect(VersionTypeDate.String()).To(Equal("date"))
			Expect(VersionTypeSemver.String()).To(Equal("semver"))
			Expect(VersionTypeString.String()).To(Equal("string"))
			Expect(VersionTypeHead.String()).To(Equal("head"))
		})
	})
})
