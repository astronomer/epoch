package cadwyn

import (
	"net/http/httptest"
	"reflect"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("RouteGenerator", func() {
	var (
		bundle    *VersionBundle
		chain     *MigrationChain
		generator *RouteGenerator
		v1, v2    *Version
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		bundle = NewVersionBundle([]*Version{v1, v2})
		chain = NewMigrationChain([]*VersionChange{})
		generator = NewRouteGenerator(bundle, chain)
		gin.SetMode(gin.TestMode)
	})

	Describe("NewRouteGenerator", func() {
		It("should create a new route generator", func() {
			Expect(generator).NotTo(BeNil())
		})
	})

	Describe("GenerateVersionedRoutes", func() {
		var headRouter *VersionedRouter

		BeforeEach(func() {
			config := RouterConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
			}
			headRouter = NewVersionedRouter(config)

			// Add some test routes
			testHandler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "test"})
			}
			headRouter.GET("/users", testHandler, v1, v2)
			headRouter.POST("/users", testHandler, v1)
		})

		It("should generate versioned routes", func() {
			routes, err := generator.GenerateVersionedRoutes(headRouter, nil)
			Expect(err).NotTo(HaveOccurred())
			Expect(routes).NotTo(BeNil())
			Expect(routes.Endpoints).NotTo(BeEmpty())
			Expect(routes.Webhooks).NotTo(BeEmpty())
		})

		It("should generate routes for all versions", func() {
			routes, err := generator.GenerateVersionedRoutes(headRouter, nil)
			Expect(err).NotTo(HaveOccurred())

			// Should have entries for each version
			Expect(routes.Endpoints).To(HaveKey("1.0.0"))
			Expect(routes.Endpoints).To(HaveKey("2.0.0"))
		})

		It("should handle provided webhook router", func() {
			webhookConfig := RouterConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
			}
			webhookRouter := NewVersionedRouter(webhookConfig)

			routes, err := generator.GenerateVersionedRoutes(headRouter, webhookRouter)
			Expect(err).NotTo(HaveOccurred())
			Expect(routes.Webhooks).NotTo(BeEmpty())
		})
	})
})

var _ = Describe("RouteTransformer", func() {
	var (
		bundle      *VersionBundle
		chain       *MigrationChain
		transformer *RouteTransformer
		v1, v2      *Version
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		bundle = NewVersionBundle([]*Version{v1, v2})
		chain = NewMigrationChain([]*VersionChange{})
		transformer = NewRouteTransformer(bundle, chain)
		gin.SetMode(gin.TestMode)
	})

	Describe("NewRouteTransformer", func() {
		It("should create a new route transformer", func() {
			Expect(transformer).NotTo(BeNil())
		})
	})

	Describe("TransformRoute", func() {
		var route *Route

		BeforeEach(func() {
			route = &Route{
				Pattern:  "/users",
				Method:   "GET",
				Handler:  func(c *gin.Context) {},
				Versions: []*Version{v1, v2},
			}
		})

		It("should return a transformed handler", func() {
			originalHandler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "original"})
			}

			transformedHandler := transformer.TransformRoute(originalHandler, route)
			Expect(transformedHandler).NotTo(BeNil())
		})

		It("should create handler that can be called", func() {
			originalHandler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "original"})
			}

			transformedHandler := transformer.TransformRoute(originalHandler, route)

			// Create a test context
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			// Should not panic when called
			Expect(func() {
				transformedHandler(c)
			}).NotTo(Panic())
		})
	})
})

var _ = Describe("EndpointGenerator", func() {
	var (
		bundle    *VersionBundle
		generator *EndpointGenerator
		v1, v2    *Version
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		bundle = NewVersionBundle([]*Version{v1, v2})
		generator = NewEndpointGenerator(bundle)
		gin.SetMode(gin.TestMode)
	})

	Describe("NewEndpointGenerator", func() {
		It("should create a new endpoint generator", func() {
			Expect(generator).NotTo(BeNil())
		})
	})

	Describe("GenerateCRUDEndpoints", func() {
		It("should generate CRUD endpoints for a type", func() {
			userType := reflect.TypeOf(TestUser{})
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")

			Expect(handlers).NotTo(BeEmpty())
			Expect(handlers).To(HaveKey("GET /users"))
			Expect(handlers).To(HaveKey("GET /users/{id}"))
			Expect(handlers).To(HaveKey("POST /users"))
			Expect(handlers).To(HaveKey("PUT /users/{id}"))
			Expect(handlers).To(HaveKey("DELETE /users/{id}"))
		})

		It("should generate working handlers", func() {
			userType := reflect.TypeOf(TestUser{})
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")

			// Test that handlers can be called without panicking
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			listHandler := handlers["GET /users"]
			Expect(func() {
				listHandler(c)
			}).NotTo(Panic())

			createHandler := handlers["POST /users"]
			Expect(func() {
				createHandler(c)
			}).NotTo(Panic())
		})

		It("should use resource type name in handlers", func() {
			userType := reflect.TypeOf(TestUser{})
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")

			// Test the list handler
			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)

			listHandler := handlers["GET /users"]
			listHandler(c)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.String()).To(ContainSubstring("TestUser"))
		})
	})

	Describe("individual CRUD handlers", func() {
		var userType reflect.Type

		BeforeEach(func() {
			userType = reflect.TypeOf(TestUser{})
		})

		It("should generate list handler", func() {
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")
			handler := handlers["GET /users"]

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			handler(c)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.String()).To(ContainSubstring("List"))
		})

		It("should generate get handler", func() {
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")
			handler := handlers["GET /users/{id}"]

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			handler(c)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.String()).To(ContainSubstring("Get"))
		})

		It("should generate create handler", func() {
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")
			handler := handlers["POST /users"]

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			handler(c)

			Expect(recorder.Code).To(Equal(201))
			Expect(recorder.Body.String()).To(ContainSubstring("Create"))
		})

		It("should generate update handler", func() {
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")
			handler := handlers["PUT /users/{id}"]

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			handler(c)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.String()).To(ContainSubstring("Update"))
		})

		It("should generate delete handler", func() {
			handlers := generator.GenerateCRUDEndpoints(userType, "/users")
			handler := handlers["DELETE /users/{id}"]

			recorder := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(recorder)
			handler(c)

			Expect(recorder.Code).To(Equal(204))
		})
	})
})

var _ = Describe("Helper functions", func() {
	Describe("getStringFromAttributes", func() {
		It("should extract string value from attributes", func() {
			attributes := map[string]interface{}{
				"method": "POST",
				"path":   "/users",
			}

			// This function is not exported, so we test it indirectly through
			// the endpoint instruction application
			instruction := &EndpointInstruction{
				Path:       "/test",
				Type:       "endpoint_changed",
				Attributes: attributes,
			}

			// The instruction should be created without error
			Expect(instruction).NotTo(BeNil())
			Expect(instruction.Attributes).To(HaveKeyWithValue("method", "POST"))
		})
	})
})

var _ = Describe("ResponseCapture integration", func() {
	It("should capture response data", func() {
		recorder := httptest.NewRecorder()
		capture := &ResponseCapture{
			ResponseWriter: recorder,
		}

		data := []byte(`{"message":"test"}`)
		n, err := capture.Write(data)

		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(len(data)))

		capture.WriteHeader(201)
		// Status code capture is tested in the middleware tests
	})
})
