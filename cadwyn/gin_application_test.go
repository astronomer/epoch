package cadwyn

import (
	"net/http/httptest"
	"reflect"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Application", func() {
	var (
		bundle *VersionBundle
		chain  *MigrationChain
		v1, v2 *Version
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		bundle = NewVersionBundle([]*Version{v1, v2})
		chain = NewMigrationChain([]*VersionChange{})
		gin.SetMode(gin.TestMode)
	})

	Describe("NewApplication", func() {
		Context("with valid configuration", func() {
			It("should create a new application", func() {
				config := &ApplicationConfig{
					VersionBundle:          bundle,
					MigrationChain:         chain,
					EnableSchemaGeneration: true,
					EnableChangelog:        true,
					EnableDebugLogging:     false,
					GinMode:                gin.TestMode,
				}

				app, err := NewApplication(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(app).NotTo(BeNil())
			})

			It("should apply default values", func() {
				config := &ApplicationConfig{
					VersionBundle:  bundle,
					MigrationChain: chain,
				}

				app, err := NewApplication(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(app).NotTo(BeNil())
			})

			It("should create schema generator when enabled", func() {
				config := &ApplicationConfig{
					VersionBundle:          bundle,
					MigrationChain:         chain,
					EnableSchemaGeneration: true,
				}

				app, err := NewApplication(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(app.GetSchemaGenerator()).NotTo(BeNil())
			})

			It("should not create schema generator when disabled", func() {
				config := &ApplicationConfig{
					VersionBundle:          bundle,
					MigrationChain:         chain,
					EnableSchemaGeneration: false,
				}

				app, err := NewApplication(config)
				Expect(err).NotTo(HaveOccurred())
				Expect(app.GetSchemaGenerator()).To(BeNil())
			})
		})

		Context("with invalid configuration", func() {
			It("should return error when version bundle is nil", func() {
				config := &ApplicationConfig{
					MigrationChain: chain,
				}

				_, err := NewApplication(config)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("version bundle is required"))
			})
		})
	})

	Describe("HTTP method handlers", func() {
		var app *Application

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				GinMode:        gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should register GET handler", func() {
			handler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "test"})
			}

			routes := app.GET("/test", handler)
			Expect(routes).NotTo(BeNil())
		})

		It("should register POST handler", func() {
			handler := func(c *gin.Context) {
				c.JSON(201, gin.H{"message": "created"})
			}

			routes := app.POST("/test", handler)
			Expect(routes).NotTo(BeNil())
		})

		It("should register PUT handler", func() {
			handler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "updated"})
			}

			routes := app.PUT("/test", handler)
			Expect(routes).NotTo(BeNil())
		})

		It("should register DELETE handler", func() {
			handler := func(c *gin.Context) {
				c.Status(204)
			}

			routes := app.DELETE("/test", handler)
			Expect(routes).NotTo(BeNil())
		})

		It("should register PATCH handler", func() {
			handler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "patched"})
			}

			routes := app.PATCH("/test", handler)
			Expect(routes).NotTo(BeNil())
		})

		It("should register HEAD handler", func() {
			handler := func(c *gin.Context) {
				c.Status(200)
			}

			routes := app.HEAD("/test", handler)
			Expect(routes).NotTo(BeNil())
		})

		It("should register OPTIONS handler", func() {
			handler := func(c *gin.Context) {
				c.Status(200)
			}

			routes := app.OPTIONS("/test", handler)
			Expect(routes).NotTo(BeNil())
		})

		It("should register Any handler", func() {
			handler := func(c *gin.Context) {
				c.JSON(200, gin.H{"method": c.Request.Method})
			}

			routes := app.Any("/test", handler)
			Expect(routes).NotTo(BeNil())
		})
	})

	Describe("Group", func() {
		var app *Application

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				GinMode:        gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should create a route group", func() {
			group := app.Group("/api/v1")
			Expect(group).NotTo(BeNil())
		})

		It("should create group with middleware", func() {
			middleware := func(c *gin.Context) {
				c.Next()
			}

			group := app.Group("/api/v1", middleware)
			Expect(group).NotTo(BeNil())
		})
	})

	Describe("Use", func() {
		var app *Application

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				GinMode:        gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should add global middleware", func() {
			middleware := func(c *gin.Context) {
				c.Next()
			}

			routes := app.Use(middleware)
			Expect(routes).NotTo(BeNil())
		})
	})

	Describe("GetEngine", func() {
		var app *Application

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				GinMode:        gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the underlying Gin engine", func() {
			engine := app.GetEngine()
			Expect(engine).NotTo(BeNil())
		})
	})

	Describe("GetVersionedRouter", func() {
		var app *Application

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				GinMode:        gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the versioned router", func() {
			router := app.GetVersionedRouter()
			Expect(router).NotTo(BeNil())
		})
	})

	Describe("GetVersionBundle", func() {
		var app *Application

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
				GinMode:        gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the version bundle", func() {
			vb := app.GetVersionBundle()
			Expect(vb).To(Equal(bundle))
		})
	})

	Describe("GenerateStructForVersion", func() {
		var app *Application

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:          bundle,
				MigrationChain:         chain,
				EnableSchemaGeneration: true,
				GinMode:                gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
		})

		It("should generate struct code when schema generation is enabled", func() {
			// Register a type first
			err := app.GetSchemaGenerator().RegisterType(reflect.TypeOf(TestUser{}))
			Expect(err).NotTo(HaveOccurred())

			code, err := app.GenerateStructForVersion(TestUser{}, "1.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(ContainSubstring("TestUser"))
		})

		It("should return error when schema generation is disabled", func() {
			config := &ApplicationConfig{
				VersionBundle:          bundle,
				MigrationChain:         chain,
				EnableSchemaGeneration: false,
				GinMode:                gin.TestMode,
			}
			app, err := NewApplication(config)
			Expect(err).NotTo(HaveOccurred())

			_, err = app.GenerateStructForVersion(TestUser{}, "1.0.0")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("schema generation is not enabled"))
		})
	})

	Describe("utility endpoints", func() {
		var app *Application
		var recorder *httptest.ResponseRecorder

		BeforeEach(func() {
			config := &ApplicationConfig{
				VersionBundle:   bundle,
				MigrationChain:  chain,
				EnableChangelog: true,
				GinMode:         gin.TestMode,
			}
			var err error
			app, err = NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
			recorder = httptest.NewRecorder()
		})

		Describe("/versions endpoint", func() {
			It("should return version information", func() {
				req := httptest.NewRequest("GET", "/versions", nil)
				app.GetEngine().ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring("versions"))
				Expect(recorder.Body.String()).To(ContainSubstring("head"))
				Expect(recorder.Body.String()).To(ContainSubstring("default"))
			})
		})

		Describe("/health endpoint", func() {
			It("should return health status", func() {
				req := httptest.NewRequest("GET", "/health", nil)
				app.GetEngine().ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring("healthy"))
				Expect(recorder.Body.String()).To(ContainSubstring("versions"))
			})
		})

		Describe("/changelog endpoint", func() {
			It("should return changelog when enabled", func() {
				req := httptest.NewRequest("GET", "/changelog", nil)
				app.GetEngine().ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring("versions"))
			})
		})

		Describe("/openapi.json endpoint", func() {
			It("should return OpenAPI specification", func() {
				req := httptest.NewRequest("GET", "/openapi.json", nil)
				app.GetEngine().ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring("openapi"))
				Expect(recorder.Body.String()).To(ContainSubstring("3.0.0"))
			})

			It("should handle version query parameter", func() {
				req := httptest.NewRequest("GET", "/openapi.json?version=1.0.0", nil)
				app.GetEngine().ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring("1.0.0"))
			})
		})

		Describe("/docs endpoint", func() {
			It("should return documentation dashboard", func() {
				req := httptest.NewRequest("GET", "/docs", nil)
				app.GetEngine().ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Header().Get("Content-Type")).To(ContainSubstring("text/html"))
				Expect(recorder.Body.String()).To(ContainSubstring("API Documentation"))
			})

			It("should return version-specific docs", func() {
				req := httptest.NewRequest("GET", "/docs?version=1.0.0", nil)
				app.GetEngine().ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(200))
				Expect(recorder.Body.String()).To(ContainSubstring("Version 1.0.0"))
			})
		})
	})

	Describe("configuration defaults", func() {
		It("should apply header version location default", func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
			}

			app, err := NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(app).NotTo(BeNil())
		})

		It("should apply query version location with correct parameter", func() {
			config := &ApplicationConfig{
				VersionBundle:   bundle,
				MigrationChain:  chain,
				VersionLocation: VersionLocationQuery,
			}

			app, err := NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(app).NotTo(BeNil())
		})

		It("should apply path version location with correct parameter", func() {
			config := &ApplicationConfig{
				VersionBundle:   bundle,
				MigrationChain:  chain,
				VersionLocation: VersionLocationPath,
			}

			app, err := NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(app).NotTo(BeNil())
		})

		It("should use head version as default when not specified", func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
			}

			app, err := NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(app).NotTo(BeNil())
		})

		It("should apply default title and version", func() {
			config := &ApplicationConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
			}

			app, err := NewApplication(config)
			Expect(err).NotTo(HaveOccurred())
			Expect(app).NotTo(BeNil())
		})
	})
})
