package epoch

import (
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type TestUser struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type TestUserRequest struct {
	Name string `json:"name"`
}

var _ = Describe("Cadwyn", func() {
	var (
		v1, v2 *Version
		change *VersionChange
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		change = NewVersionChange("Test change", v1, v2)
	})

	Describe("NewCadwyn", func() {
		It("should create a new EpochBuilder", func() {
			builder := NewEpoch()
			Expect(builder).NotTo(BeNil())
		})
	})

	Describe("EpochBuilder", func() {
		var builder *EpochBuilder

		BeforeEach(func() {
			builder = NewEpoch()
		})

		Describe("WithVersions", func() {
			It("should add versions to the builder", func() {
				result := builder.WithVersions(v1, v2)
				Expect(result).To(Equal(builder)) // Should return self for chaining
			})
		})

		Describe("WithDateVersions", func() {
			It("should create and add date versions", func() {
				result := builder.WithDateVersions("2023-01-01", "2024-01-01")
				Expect(result).To(Equal(builder))
			})

			It("should skip invalid date versions", func() {
				result := builder.WithDateVersions("invalid-date", "2024-01-01")
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithSemverVersions", func() {
			It("should create and add semver versions", func() {
				result := builder.WithSemverVersions("1.0.0", "2.0")
				Expect(result).To(Equal(builder))
			})

			It("should skip invalid semver versions", func() {
				result := builder.WithSemverVersions("invalid", "2.0.0")
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithStringVersions", func() {
			It("should create and add string versions", func() {
				result := builder.WithStringVersions("alpha", "beta", "stable")
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithHeadVersion", func() {
			It("should add a head version", func() {
				result := builder.WithHeadVersion()
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithChanges", func() {
			It("should add version changes", func() {
				result := builder.WithChanges(change)
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithVersionParameter", func() {
			It("should set version parameter name", func() {
				result := builder.WithVersionParameter("API-Version")
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithVersionFormat", func() {
			It("should set version format", func() {
				result := builder.WithVersionFormat(VersionFormatSemver)
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithDefaultVersion", func() {
			It("should set default version", func() {
				result := builder.WithDefaultVersion(v1)
				Expect(result).To(Equal(builder))
			})
		})

		Describe("WithTypes", func() {
			It("should register types for schema generation", func() {
				result := builder.WithTypes(TestUser{}, TestUserRequest{})
				Expect(result).To(Equal(builder))
			})

			It("should handle pointer types", func() {
				user := &TestUser{}
				result := builder.WithTypes(user)
				Expect(result).To(Equal(builder))
			})
		})

		Describe("Build", func() {
			Context("with valid configuration", func() {
				It("should create a Cadwyn instance", func() {
					cadwynInstance, err := builder.
						WithVersions(v1, v2).
						WithTypes(TestUser{}).
						Build()

					Expect(err).NotTo(HaveOccurred())
					Expect(cadwynInstance).NotTo(BeNil())
					Expect(cadwynInstance.GetVersions()).To(HaveLen(2))
				})
			})

			Context("with no versions", func() {
				It("should return an error", func() {
					_, err := builder.Build()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("at least one version must be specified"))
				})
			})
		})
	})

	Describe("Cadwyn instance", func() {
		var cadwynInstance *Epoch

		BeforeEach(func() {
			var err error
			cadwynInstance, err = NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithTypes(TestUser{}).
				Build()
			Expect(err).NotTo(HaveOccurred())
		})

		Describe("Middleware", func() {
			It("should return a Gin middleware function", func() {
				middleware := cadwynInstance.Middleware()
				Expect(middleware).NotTo(BeNil())
			})
		})

		Describe("WrapHandler", func() {
			It("should return a wrapped Gin handler function", func() {
				handler := func(c *gin.Context) {}
				wrappedHandler := cadwynInstance.WrapHandler(handler)
				Expect(wrappedHandler).NotTo(BeNil())
			})
		})

		Describe("GetVersionBundle", func() {
			It("should return the version bundle", func() {
				bundle := cadwynInstance.GetVersionBundle()
				Expect(bundle).NotTo(BeNil())
			})
		})

		Describe("GetMigrationChain", func() {
			It("should return the migration chain", func() {
				chain := cadwynInstance.GetMigrationChain()
				Expect(chain).NotTo(BeNil())
			})
		})

		Describe("GetVersions", func() {
			It("should return all versions", func() {
				versions := cadwynInstance.GetVersions()
				Expect(versions).To(HaveLen(2))
			})
		})

		Describe("GetHeadVersion", func() {
			It("should return the head version", func() {
				head := cadwynInstance.GetHeadVersion()
				Expect(head).NotTo(BeNil())
				Expect(head.IsHead).To(BeTrue())
			})
		})

		Describe("ParseVersion", func() {
			It("should parse version strings", func() {
				version, err := cadwynInstance.ParseVersion("1.0.0")
				Expect(err).NotTo(HaveOccurred())
				Expect(version.String()).To(Equal("1.0.0"))
			})
		})

	})

	Describe("Convenience functions", func() {
		Describe("QuickStart", func() {
			It("should create a Cadwyn instance with date versions", func() {
				cadwynInstance, err := QuickStart("2023-01-01", "2024-01-01")
				Expect(err).NotTo(HaveOccurred())
				Expect(cadwynInstance).NotTo(BeNil())
			})
		})

		Describe("WithSemver", func() {
			It("should create a Cadwyn instance with semver versions", func() {
				cadwynInstance, err := WithSemver("1.0.0", "2.0.0")
				Expect(err).NotTo(HaveOccurred())
				Expect(cadwynInstance).NotTo(BeNil())
			})
		})

		Describe("WithStrings", func() {
			It("should create a Cadwyn instance with string versions", func() {
				cadwynInstance, err := WithStrings("alpha", "beta")
				Expect(err).NotTo(HaveOccurred())
				Expect(cadwynInstance).NotTo(BeNil())
			})
		})

		Describe("Simple", func() {
			It("should create a Cadwyn instance with just head version", func() {
				cadwynInstance, err := Simple()
				Expect(err).NotTo(HaveOccurred())
				Expect(cadwynInstance).NotTo(BeNil())
			})
		})
	})

	Describe("Version helpers", func() {
		Describe("StringVersion", func() {
			It("should create a string version", func() {
				version := StringVersion("alpha")
				Expect(version.Type).To(Equal(VersionTypeString))
			})
		})

		Describe("HeadVersion", func() {
			It("should create a head version", func() {
				version := HeadVersion()
				Expect(version.IsHead).To(BeTrue())
			})
		})
	})

	Describe("Automatic Type Registration", func() {
		It("should register types immediately without HTTP request", func() {
			type TestRequest struct {
				Name string `json:"name"`
			}
			type TestResponse struct {
				ID   int    `json:"id"`
				Name string `json:"name"`
			}

			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithHeadVersion().
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.POST("/test",
				epochInstance.WrapHandler(func(c *gin.Context) {
					c.JSON(200, TestResponse{ID: 1, Name: "test"})
				}).
					Accepts(TestRequest{}).
					Returns(TestResponse{}).
					ToHandlerFunc("POST", "/test"))

			// Verify registered WITHOUT making HTTP request
			endpoints := epochInstance.EndpointRegistry().GetAll()
			Expect(endpoints).To(HaveLen(1))

			def, exists := endpoints["POST:/test"]
			Expect(exists).To(BeTrue())
			Expect(def.Method).To(Equal("POST"))
			Expect(def.PathPattern).To(Equal("/test"))
			Expect(def.RequestType.Name()).To(Equal("TestRequest"))
			Expect(def.ResponseType.Name()).To(Equal("TestResponse"))
		})
	})
})
