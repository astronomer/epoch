package cadwyn

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Middleware", func() {
	var (
		bundle *VersionBundle
		chain  *MigrationChain
		v1, v2 *Version
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		var err error
		bundle, err = NewVersionBundle([]*Version{v1, v2})
		Expect(err).NotTo(HaveOccurred())
		chain = NewMigrationChain([]*VersionChange{})
		gin.SetMode(gin.TestMode)
	})

	Describe("HeaderVersionManager", func() {
		var manager *HeaderVersionManager

		BeforeEach(func() {
			manager = NewHeaderVersionManager("X-API-Version")
		})

		It("should extract version from header", func() {
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-API-Version", "1.0.0")

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.0.0"))
		})

		It("should return empty string when header is missing", func() {
			req := httptest.NewRequest("GET", "/test", nil)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
		})
	})

	Describe("QueryVersionManager", func() {
		var manager *QueryVersionManager

		BeforeEach(func() {
			manager = NewQueryVersionManager("version")
		})

		It("should extract version from query parameter", func() {
			req := httptest.NewRequest("GET", "/test?version=1.0.0", nil)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.0.0"))
		})

		It("should return empty string when query parameter is missing", func() {
			req := httptest.NewRequest("GET", "/test", nil)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
		})
	})

	Describe("PathVersionManager", func() {
		var manager *PathVersionManager

		BeforeEach(func() {
			manager = NewPathVersionManager([]string{"1.0.0", "2.0.0"})
		})

		It("should extract version from path", func() {
			req := httptest.NewRequest("GET", "/api/1.0.0/users", nil)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.0.0"))
		})

		It("should return empty string when version not in path", func() {
			req := httptest.NewRequest("GET", "/api/users", nil)
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal(""))
		})
	})

	Describe("VersionMiddleware", func() {
		var middleware *VersionMiddleware

		BeforeEach(func() {
			config := MiddlewareConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				ParameterName:  "X-API-Version",
				Location:       VersionLocationHeader,
				Format:         VersionFormatSemver,
				DefaultVersion: nil,
			}
			middleware = NewVersionMiddleware(config)
		})

		Describe("NewVersionMiddleware", func() {
			It("should create middleware with header location", func() {
				config := MiddlewareConfig{
					VersionBundle:  bundle,
					MigrationChain: chain,
					ParameterName:  "X-API-Version",
					Location:       VersionLocationHeader,
				}
				mw := NewVersionMiddleware(config)
				Expect(mw).NotTo(BeNil())
			})

			It("should create middleware with query location", func() {
				config := MiddlewareConfig{
					VersionBundle:  bundle,
					MigrationChain: chain,
					ParameterName:  "version",
					Location:       VersionLocationQuery,
				}
				mw := NewVersionMiddleware(config)
				Expect(mw).NotTo(BeNil())
			})

			It("should create middleware with path location", func() {
				config := MiddlewareConfig{
					VersionBundle:  bundle,
					MigrationChain: chain,
					ParameterName:  "v",
					Location:       VersionLocationPath,
				}
				mw := NewVersionMiddleware(config)
				Expect(mw).NotTo(BeNil())
			})

			It("should default to header when location is unknown", func() {
				config := MiddlewareConfig{
					VersionBundle:  bundle,
					MigrationChain: chain,
					ParameterName:  "X-API-Version",
					Location:       VersionLocation("unknown"),
				}
				mw := NewVersionMiddleware(config)
				Expect(mw).NotTo(BeNil())
			})
		})

		Describe("Middleware", func() {
			var router *gin.Engine
			var recorder *httptest.ResponseRecorder

			BeforeEach(func() {
				router = gin.New()
				router.Use(middleware.Middleware())
				router.GET("/test", func(c *gin.Context) {
					version := GetVersionFromContext(c)
					if version != nil {
						c.JSON(200, gin.H{"version": version.String()})
					} else {
						c.JSON(200, gin.H{"version": "none"})
					}
				})
				recorder = httptest.NewRecorder()
			})

			It("should set version in context when valid version provided", func() {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "1.0.0")

				router.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring(`"version":"1.0.0"`))
			})

			It("should use head version when no version specified", func() {
				req := httptest.NewRequest("GET", "/test", nil)

				router.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring(`"version":"head"`))
			})

			It("should return error for invalid version", func() {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "invalid")

				router.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(400))
				Expect(recorder.Body.String()).To(ContainSubstring("Unknown version"))
			})

			It("should add version header to response", func() {
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "1.0.0")

				router.ServeHTTP(recorder, req)

				Expect(recorder.Header().Get("X-API-Version")).To(Equal("1.0.0"))
			})
		})
	})

	Describe("GetVersionFromContext", func() {
		It("should return version from context", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("cadwyn_api_version", v1)

			version := GetVersionFromContext(c)
			Expect(version).To(Equal(v1))
		})

		It("should return nil when no version in context", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			version := GetVersionFromContext(c)
			Expect(version).To(BeNil())
		})

		It("should return nil when context value is not a version", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("cadwyn_api_version", "not a version")

			version := GetVersionFromContext(c)
			Expect(version).To(BeNil())
		})
	})

	Describe("IsDefaultVersionUsed", func() {
		It("should return true when default version was used", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("cadwyn_default_version_used", true)

			used := IsDefaultVersionUsed(c)
			Expect(used).To(BeTrue())
		})

		It("should return false when explicit version was used", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Set("cadwyn_default_version_used", false)

			used := IsDefaultVersionUsed(c)
			Expect(used).To(BeFalse())
		})

		It("should return false when no flag in context", func() {
			c, _ := gin.CreateTestContext(httptest.NewRecorder())

			used := IsDefaultVersionUsed(c)
			Expect(used).To(BeFalse())
		})
	})

	Describe("VersionAwareHandler", func() {
		var handler *VersionAwareHandler

		BeforeEach(func() {
			ginHandler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "test"})
			}
			handler = NewVersionAwareHandler(ginHandler, bundle, chain)
		})

		It("should create a version-aware handler", func() {
			Expect(handler).NotTo(BeNil())
		})

		It("should return a Gin handler function", func() {
			handlerFunc := handler.HandlerFunc()
			Expect(handlerFunc).NotTo(BeNil())
		})
	})

	Describe("ResponseCapture", func() {
		var capture *ResponseCapture
		var recorder *httptest.ResponseRecorder

		BeforeEach(func() {
			recorder = httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			capture = &ResponseCapture{
				ResponseWriter: c.Writer,
				body:           make([]byte, 0),
				statusCode:     200,
			}
		})

		It("should capture written data", func() {
			data := []byte("test data")
			n, err := capture.Write(data)

			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(len(data)))
		})

		It("should capture status code", func() {
			capture.WriteHeader(404)
			// Note: We can't easily test the captured status code without exposing it
			// This is more of a smoke test to ensure no panics occur
		})
	})
})
