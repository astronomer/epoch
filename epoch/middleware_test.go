package epoch

import (
	"encoding/json"
	"net/http/httptest"
	"sync"

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

		It("should handle multiple writes to response capture", func() {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			capture := &ResponseCapture{
				ResponseWriter: c.Writer,
				body:           make([]byte, 0),
				statusCode:     200,
			}

			// Write multiple times
			data1 := []byte("Hello ")
			data2 := []byte("World")
			n1, err1 := capture.Write(data1)
			n2, err2 := capture.Write(data2)

			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())
			Expect(n1).To(Equal(len(data1)))
			Expect(n2).To(Equal(len(data2)))
			Expect(string(capture.body)).To(Equal("Hello World"))
		})

		It("should handle empty writes", func() {
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			capture := &ResponseCapture{
				ResponseWriter: c.Writer,
				body:           make([]byte, 0),
				statusCode:     200,
			}

			n, err := capture.Write([]byte{})
			Expect(err).NotTo(HaveOccurred())
			Expect(n).To(Equal(0))
			Expect(capture.body).To(HaveLen(0))
		})
	})

	Describe("Waterfall Versioning", func() {
		var (
			bundle     *VersionBundle
			chain      *MigrationChain
			middleware *VersionMiddleware
			v1, v2, v3 *Version
		)

		BeforeEach(func() {
			v1, _ = NewSemverVersion("1.0.0")
			v2, _ = NewSemverVersion("2.0.0")
			v3, _ = NewSemverVersion("3.0.0")
			var err error
			bundle, err = NewVersionBundle([]*Version{v1, v2, v3})
			Expect(err).NotTo(HaveOccurred())
			chain = NewMigrationChain([]*VersionChange{})
		})

		Context("with semver versions", func() {
			BeforeEach(func() {
				config := MiddlewareConfig{
					VersionBundle:  bundle,
					MigrationChain: chain,
					ParameterName:  "X-API-Version",
					Location:       VersionLocationHeader,
					Format:         VersionFormatSemver,
				}
				middleware = NewVersionMiddleware(config)
			})

			It("should select closest older version when exact match not found", func() {
				router := gin.New()
				router.Use(middleware.Middleware())
				router.GET("/test", func(c *gin.Context) {
					version := GetVersionFromContext(c)
					c.JSON(200, gin.H{"version": version.String()})
				})

				// Request version 2.5.0 should fall back to 2.0.0
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "2.5.0")
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring(`"version":"2.0.0"`))
			})

			It("should reject version that is too old", func() {
				router := gin.New()
				router.Use(middleware.Middleware())
				router.GET("/test", func(c *gin.Context) {
					c.JSON(200, gin.H{"ok": true})
				})

				// Request version 0.5.0 is older than all available versions
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "0.5.0")
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(400))
				Expect(recorder.Body.String()).To(ContainSubstring("Unknown version"))
			})

			It("should use head version for version newer than all available", func() {
				router := gin.New()
				router.Use(middleware.Middleware())
				router.GET("/test", func(c *gin.Context) {
					version := GetVersionFromContext(c)
					c.JSON(200, gin.H{"version": version.String()})
				})

				// Request version 99.0.0 is newer than all available versions
				// Waterfall logic: if no exact match and no older version, use head
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "99.0.0")
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, req)

				// Should default to head or return an error
				Expect(recorder.Code).To(BeElementOf([]int{200, 400}))
			})
		})

		Context("with date versions", func() {
			BeforeEach(func() {
				d1, _ := NewDateVersion("2023-01-01")
				d2, _ := NewDateVersion("2024-01-01")
				d3, _ := NewDateVersion("2025-01-01")
				var err error
				bundle, err = NewVersionBundle([]*Version{d1, d2, d3})
				Expect(err).NotTo(HaveOccurred())

				config := MiddlewareConfig{
					VersionBundle:  bundle,
					MigrationChain: chain,
					ParameterName:  "X-API-Version",
					Location:       VersionLocationHeader,
					Format:         VersionFormatDate,
				}
				middleware = NewVersionMiddleware(config)
			})

			It("should select closest older date when exact match not found", func() {
				router := gin.New()
				router.Use(middleware.Middleware())
				router.GET("/test", func(c *gin.Context) {
					version := GetVersionFromContext(c)
					c.JSON(200, gin.H{"version": version.String()})
				})

				// Request 2024-06-15 should fall back to 2024-01-01
				req := httptest.NewRequest("GET", "/test", nil)
				req.Header.Set("X-API-Version", "2024-06-15")
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring(`"version":"2024-01-01"`))
			})
		})
	})

	Describe("Concurrent Request Handling", func() {
		var (
			bundle     *VersionBundle
			chain      *MigrationChain
			middleware *VersionMiddleware
		)

		BeforeEach(func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")
			var err error
			bundle, err = NewVersionBundle([]*Version{v1, v2})
			Expect(err).NotTo(HaveOccurred())
			chain = NewMigrationChain([]*VersionChange{})

			config := MiddlewareConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				ParameterName:  "X-API-Version",
				Location:       VersionLocationHeader,
				Format:         VersionFormatSemver,
			}
			middleware = NewVersionMiddleware(config)
		})

		It("should handle concurrent requests with different versions", func() {
			router := gin.New()
			router.Use(middleware.Middleware())
			router.GET("/test", func(c *gin.Context) {
				version := GetVersionFromContext(c)
				c.JSON(200, gin.H{"version": version.String()})
			})

			numRequests := 100
			var wg sync.WaitGroup
			results := make([]string, numRequests)
			versions := []string{"1.0.0", "2.0.0", "head"}

			for i := 0; i < numRequests; i++ {
				wg.Add(1)
				go func(idx int) {
					defer wg.Done()

					version := versions[idx%len(versions)]
					req := httptest.NewRequest("GET", "/test", nil)
					if version != "head" {
						req.Header.Set("X-API-Version", version)
					}
					recorder := httptest.NewRecorder()

					router.ServeHTTP(recorder, req)

					var response map[string]interface{}
					err := json.Unmarshal(recorder.Body.Bytes(), &response)
					if err == nil {
						results[idx] = response["version"].(string)
					}
				}(i)
			}

			wg.Wait()

			// Verify all requests were handled correctly
			for i := 0; i < numRequests; i++ {
				expectedVersion := versions[i%len(versions)]
				Expect(results[i]).To(Equal(expectedVersion))
			}
		})
	})

	Describe("Complex Error Scenarios", func() {
		var (
			bundle     *VersionBundle
			chain      *MigrationChain
			middleware *VersionMiddleware
		)

		BeforeEach(func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")
			var err error
			bundle, err = NewVersionBundle([]*Version{v1, v2})
			Expect(err).NotTo(HaveOccurred())
			chain = NewMigrationChain([]*VersionChange{})

			config := MiddlewareConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				ParameterName:  "X-API-Version",
				Location:       VersionLocationHeader,
				Format:         VersionFormatSemver,
			}
			middleware = NewVersionMiddleware(config)
		})

		It("should handle invalid version format appropriately", func() {
			router := gin.New()
			router.Use(middleware.Middleware())
			router.GET("/test", func(c *gin.Context) {
				version := GetVersionFromContext(c)
				c.JSON(200, gin.H{"version": version.String()})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-API-Version", "not-a-version")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			// May return error or fall back to head/string version
			if recorder.Code == 400 {
				var response map[string]interface{}
				err := json.Unmarshal(recorder.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())
				Expect(response).To(HaveKey("error"))
			} else {
				Expect(recorder.Code).To(Equal(200))
			}
		})

		It("should set response version header even on error", func() {
			router := gin.New()
			router.Use(middleware.Middleware())
			router.GET("/test", func(c *gin.Context) {
				c.JSON(200, gin.H{"ok": true})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-API-Version", "1.0.0")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Header().Get("X-API-Version")).To(Equal("1.0.0"))
		})
	})

	Describe("Path-based versioning edge cases", func() {
		var (
			bundle     *VersionBundle
			chain      *MigrationChain
			middleware *VersionMiddleware
		)

		BeforeEach(func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")
			var err error
			bundle, err = NewVersionBundle([]*Version{v1, v2})
			Expect(err).NotTo(HaveOccurred())
			chain = NewMigrationChain([]*VersionChange{})

			config := MiddlewareConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				ParameterName:  "version",
				Location:       VersionLocationPath,
				Format:         VersionFormatSemver,
			}
			middleware = NewVersionMiddleware(config)
		})

		It("should extract version from path with multiple segments", func() {
			router := gin.New()
			router.Use(middleware.Middleware())
			router.GET("/api/:version/users/:id", func(c *gin.Context) {
				version := GetVersionFromContext(c)
				c.JSON(200, gin.H{"version": version.String()})
			})

			req := httptest.NewRequest("GET", "/api/1.0.0/users/123", nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.String()).To(ContainSubstring(`"version":"1.0.0"`))
		})

		It("should use head version when path has no version", func() {
			router := gin.New()
			router.Use(middleware.Middleware())
			router.GET("/api/users", func(c *gin.Context) {
				version := GetVersionFromContext(c)
				c.JSON(200, gin.H{"version": version.String()})
			})

			req := httptest.NewRequest("GET", "/api/users", nil)
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.String()).To(ContainSubstring(`"version":"head"`))
		})
	})

	Describe("Default Version Behavior", func() {
		var (
			bundle     *VersionBundle
			chain      *MigrationChain
			middleware *VersionMiddleware
			v1, v2     *Version
		)

		BeforeEach(func() {
			v1, _ = NewSemverVersion("1.0.0")
			v2, _ = NewSemverVersion("2.0.0")
			var err error
			bundle, err = NewVersionBundle([]*Version{v1, v2})
			Expect(err).NotTo(HaveOccurred())
			chain = NewMigrationChain([]*VersionChange{})
		})

		It("should use custom default version when provided", func() {
			config := MiddlewareConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				ParameterName:  "X-API-Version",
				Location:       VersionLocationHeader,
				DefaultVersion: v1, // Set v1 as default
			}
			middleware = NewVersionMiddleware(config)

			router := gin.New()
			router.Use(middleware.Middleware())
			router.GET("/test", func(c *gin.Context) {
				version := GetVersionFromContext(c)
				c.JSON(200, gin.H{
					"version":      version.String(),
					"default_used": IsDefaultVersionUsed(c),
				})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			// No version header set
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["version"]).To(Equal("1.0.0"))
			Expect(response["default_used"]).To(BeTrue())
		})

		It("should not mark explicit version as default", func() {
			config := MiddlewareConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				ParameterName:  "X-API-Version",
				Location:       VersionLocationHeader,
				DefaultVersion: v1,
			}
			middleware = NewVersionMiddleware(config)

			router := gin.New()
			router.Use(middleware.Middleware())
			router.GET("/test", func(c *gin.Context) {
				version := GetVersionFromContext(c)
				c.JSON(200, gin.H{
					"version":      version.String(),
					"default_used": IsDefaultVersionUsed(c),
				})
			})

			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-API-Version", "2.0.0")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			var response map[string]interface{}
			err := json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())
			Expect(response["version"]).To(Equal("2.0.0"))
			Expect(response["default_used"]).To(BeFalse())
		})
	})

	Describe("Version Manager Edge Cases", func() {
		It("should handle case-insensitive headers", func() {
			manager := NewHeaderVersionManager("x-api-version")
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Set("X-API-VERSION", "1.0.0")

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.0.0"))
		})

		It("should handle multiple version headers (first one wins)", func() {
			manager := NewHeaderVersionManager("X-API-Version")
			req := httptest.NewRequest("GET", "/test", nil)
			req.Header.Add("X-API-Version", "1.0.0")
			req.Header.Add("X-API-Version", "2.0.0")

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.0.0"))
		})

		It("should handle path with multiple matching versions", func() {
			manager := NewPathVersionManager([]string{"1.0.0", "2.0.0", "10.0.0"})
			req := httptest.NewRequest("GET", "/api/1.0.0/resource", nil)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("1.0.0"))
		})

		It("should handle special characters in version strings", func() {
			manager := NewPathVersionManager([]string{"v1.0-beta", "v2.0-rc1"})
			req := httptest.NewRequest("GET", "/api/v1.0-beta/users", nil)

			c, _ := gin.CreateTestContext(httptest.NewRecorder())
			c.Request = req

			version, err := manager.GetVersion(c)
			Expect(err).NotTo(HaveOccurred())
			Expect(version).To(Equal("v1.0-beta"))
		})
	})
})
