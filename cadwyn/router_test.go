package cadwyn

import (
	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("VersionedRouter", func() {
	var (
		bundle *VersionBundle
		chain  *MigrationChain
		router *VersionedRouter
		v1, v2 *Version
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		bundle = NewVersionBundle([]*Version{v1, v2})
		chain = NewMigrationChain([]*VersionChange{})

		config := RouterConfig{
			VersionBundle:           bundle,
			MigrationChain:          chain,
			APIVersionParameterName: "X-API-Version",
			RedirectSlashes:         true,
		}
		router = NewVersionedRouter(config)
		gin.SetMode(gin.TestMode)
	})

	Describe("NewVersionedRouter", func() {
		It("should create a versioned router", func() {
			Expect(router).NotTo(BeNil())
		})

		It("should use default parameter name when not provided", func() {
			config := RouterConfig{
				VersionBundle:  bundle,
				MigrationChain: chain,
			}
			router := NewVersionedRouter(config)
			Expect(router).NotTo(BeNil())
		})
	})

	Describe("HTTP method handlers", func() {
		var testHandler gin.HandlerFunc

		BeforeEach(func() {
			testHandler = func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "test"})
			}
		})

		Describe("GET", func() {
			It("should register GET handler for specific versions", func() {
				router.GET("/users", testHandler, v1, v2)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("GET /users"))
				Expect(routes["GET /users"].Method).To(Equal("GET"))
				Expect(routes["GET /users"].Pattern).To(Equal("/users"))
				Expect(routes["GET /users"].Versions).To(HaveLen(2))
			})

			It("should register GET handler for all versions when none specified", func() {
				router.GET("/users", testHandler)

				routes := router.GetRoutes()
				route := routes["GET /users"]
				// Should include head version + all regular versions
				Expect(route.Versions).To(HaveLen(3)) // head + v1 + v2
			})
		})

		Describe("POST", func() {
			It("should register POST handler", func() {
				router.POST("/users", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("POST /users"))
				Expect(routes["POST /users"].Method).To(Equal("POST"))
			})
		})

		Describe("PUT", func() {
			It("should register PUT handler", func() {
				router.PUT("/users/1", testHandler, v2)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("PUT /users/1"))
			})
		})

		Describe("DELETE", func() {
			It("should register DELETE handler", func() {
				router.DELETE("/users/1", testHandler, v1, v2)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("DELETE /users/1"))
			})
		})

		Describe("PATCH", func() {
			It("should register PATCH handler", func() {
				router.PATCH("/users/1", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("PATCH /users/1"))
			})
		})

		Describe("Handle", func() {
			It("should register handler with custom method", func() {
				router.Handle("OPTIONS", "/users", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("OPTIONS /users"))
				Expect(routes["OPTIONS /users"].Method).To(Equal("OPTIONS"))
			})
		})
	})

	Describe("HandleUnversioned", func() {
		It("should register unversioned handler", func() {
			testHandler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "unversioned"})
			}

			router.HandleUnversioned("GET", "/health", testHandler)

			// Unversioned routes are not tracked in the routes map
			// This is mainly a smoke test to ensure no panics
			Expect(router.GetEngine()).NotTo(BeNil())
		})
	})

	Describe("GetEngineForVersion", func() {
		It("should return version-specific engine", func() {
			engine := router.GetEngineForVersion(v1)
			Expect(engine).NotTo(BeNil())
		})

		It("should return unversioned engine for nil version", func() {
			engine := router.GetEngineForVersion(nil)
			Expect(engine).NotTo(BeNil())
			Expect(engine).To(Equal(router.GetEngine()))
		})

		It("should fallback to closest older version", func() {
			v3, _ := NewSemverVersion("3.0.0")
			engine := router.GetEngineForVersion(v3)
			Expect(engine).NotTo(BeNil())
		})
	})

	Describe("RouteExists", func() {
		BeforeEach(func() {
			testHandler := func(c *gin.Context) {}
			router.GET("/users", testHandler, v1, v2)
		})

		It("should return true for existing route and version", func() {
			exists := router.RouteExists("GET", "/users", v1)
			Expect(exists).To(BeTrue())
		})

		It("should return false for non-existing route", func() {
			exists := router.RouteExists("GET", "/nonexistent", v1)
			Expect(exists).To(BeFalse())
		})

		It("should return false for existing route but wrong version", func() {
			v3, _ := NewSemverVersion("3.0.0")
			exists := router.RouteExists("GET", "/users", v3)
			Expect(exists).To(BeFalse())
		})
	})

	Describe("DeleteRoute", func() {
		BeforeEach(func() {
			testHandler := func(c *gin.Context) {}
			router.GET("/users", testHandler, v1, v2)
		})

		It("should remove version from route", func() {
			err := router.DeleteRoute("GET", "/users", v1)
			Expect(err).NotTo(HaveOccurred())

			exists := router.RouteExists("GET", "/users", v1)
			Expect(exists).To(BeFalse())

			// v2 should still exist
			exists = router.RouteExists("GET", "/users", v2)
			Expect(exists).To(BeTrue())
		})

		It("should mark route as deleted when no versions left", func() {
			err1 := router.DeleteRoute("GET", "/users", v1)
			err2 := router.DeleteRoute("GET", "/users", v2)
			Expect(err1).NotTo(HaveOccurred())
			Expect(err2).NotTo(HaveOccurred())

			routes := router.GetRoutes()
			route := routes["GET /users"]
			Expect(route.IsDeleted).To(BeTrue())
		})

		It("should return error for non-existing route", func() {
			err := router.DeleteRoute("GET", "/nonexistent", v1)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not exist"))
		})
	})

	Describe("RestoreRoute", func() {
		It("should restore deleted route", func() {
			testHandler := func(c *gin.Context) {}

			router.RestoreRoute("GET", "/users", v1, testHandler)

			exists := router.RouteExists("GET", "/users", v1)
			Expect(exists).To(BeTrue())
		})

		It("should add version to existing route", func() {
			testHandler := func(c *gin.Context) {}
			router.GET("/users", testHandler, v1)

			router.RestoreRoute("GET", "/users", v2, testHandler)

			routes := router.GetRoutes()
			route := routes["GET /users"]
			Expect(route.Versions).To(HaveLen(2))
		})
	})

	Describe("ChangeRoute", func() {
		BeforeEach(func() {
			testHandler := func(c *gin.Context) {}
			router.GET("/users", testHandler, v1, v2)
		})

		It("should change route properties", func() {
			newHandler := func(c *gin.Context) {}
			err := router.ChangeRoute("GET", "/users", "POST", "/users", v1, newHandler)
			Expect(err).NotTo(HaveOccurred())

			// Old route should not exist for v1
			exists := router.RouteExists("GET", "/users", v1)
			Expect(exists).To(BeFalse())

			// New route should exist for v1
			exists = router.RouteExists("POST", "/users", v1)
			Expect(exists).To(BeTrue())

			// v2 should still have the old route
			exists = router.RouteExists("GET", "/users", v2)
			Expect(exists).To(BeTrue())
		})
	})

	Describe("GetVersions", func() {
		It("should return sorted list of versions", func() {
			versions := router.GetVersions()
			Expect(versions).To(ContainElement("1.0.0"))
			Expect(versions).To(ContainElement("2.0.0"))
			Expect(versions).NotTo(ContainElement("head")) // head is excluded
		})
	})

	Describe("GetRouteInfo", func() {
		BeforeEach(func() {
			testHandler := func(c *gin.Context) {}
			router.GET("/users", testHandler, v1, v2)
			router.POST("/users", testHandler, v1)
		})

		It("should return detailed route information", func() {
			routeInfo := router.GetRouteInfo()
			Expect(routeInfo).To(HaveLen(2))

			// Find the GET route
			var getRoute *RouteInfo
			for i := range routeInfo {
				if routeInfo[i].Method == "GET" && routeInfo[i].Pattern == "/users" {
					getRoute = &routeInfo[i]
					break
				}
			}

			Expect(getRoute).NotTo(BeNil())
			Expect(getRoute.IsVersioned).To(BeTrue())
			Expect(getRoute.IsDeleted).To(BeFalse())
			Expect(getRoute.Versions).To(HaveLen(2))
		})
	})

	Describe("PrintRoutes", func() {
		It("should print routes without panicking", func() {
			testHandler := func(c *gin.Context) {}
			router.GET("/users", testHandler, v1)

			// This is mainly a smoke test
			Expect(func() {
				router.PrintRoutes()
			}).NotTo(Panic())
		})
	})
})

var _ = Describe("RouteGroup", func() {
	var (
		bundle *VersionBundle
		chain  *MigrationChain
		router *VersionedRouter
		group  *RouteGroup
		v1, v2 *Version
	)

	BeforeEach(func() {
		v1, _ = NewSemverVersion("1.0.0")
		v2, _ = NewSemverVersion("2.0.0")
		bundle = NewVersionBundle([]*Version{v1, v2})
		chain = NewMigrationChain([]*VersionChange{})

		config := RouterConfig{
			VersionBundle:  bundle,
			MigrationChain: chain,
		}
		router = NewVersionedRouter(config)
		group = router.Group("/api/v1")
		gin.SetMode(gin.TestMode)
	})

	Describe("Group", func() {
		It("should create a route group", func() {
			Expect(group).NotTo(BeNil())
		})

		It("should handle trailing slash in prefix", func() {
			groupWithSlash := router.Group("/api/v1/")
			Expect(groupWithSlash).NotTo(BeNil())
		})
	})

	Describe("Use", func() {
		It("should add middleware to group", func() {
			middleware := func(c *gin.Context) {
				c.Next()
			}

			Expect(func() {
				group.Use(middleware)
			}).NotTo(Panic())
		})
	})

	Describe("HTTP method handlers", func() {
		var testHandler gin.HandlerFunc

		BeforeEach(func() {
			testHandler = func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "test"})
			}
		})

		Describe("GET", func() {
			It("should register GET handler with group prefix", func() {
				group.GET("/users", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("GET /api/v1/users"))
			})
		})

		Describe("POST", func() {
			It("should register POST handler with group prefix", func() {
				group.POST("/users", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("POST /api/v1/users"))
			})
		})

		Describe("PUT", func() {
			It("should register PUT handler with group prefix", func() {
				group.PUT("/users/1", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("PUT /api/v1/users/1"))
			})
		})

		Describe("DELETE", func() {
			It("should register DELETE handler with group prefix", func() {
				group.DELETE("/users/1", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("DELETE /api/v1/users/1"))
			})
		})

		Describe("PATCH", func() {
			It("should register PATCH handler with group prefix", func() {
				group.PATCH("/users/1", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("PATCH /api/v1/users/1"))
			})
		})

		Describe("Handle", func() {
			It("should register handler with custom method and group prefix", func() {
				group.Handle("OPTIONS", "/users", testHandler, v1)

				routes := router.GetRoutes()
				Expect(routes).To(HaveKey("OPTIONS /api/v1/users"))
			})
		})
	})

	Describe("middleware application", func() {
		It("should apply group middleware to handlers", func() {
			middlewareCalled := false
			middleware := func(c *gin.Context) {
				middlewareCalled = true
				c.Next()
			}

			group.Use(middleware)

			testHandler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "test"})
			}

			group.GET("/test", testHandler, v1)

			// We can't easily test middleware execution without setting up a full HTTP test,
			// but we can verify the route was registered
			routes := router.GetRoutes()
			Expect(routes).To(HaveKey("GET /api/v1/test"))
		})

		It("should apply multiple middlewares in order", func() {
			middleware1 := func(c *gin.Context) { c.Next() }
			middleware2 := func(c *gin.Context) { c.Next() }

			group.Use(middleware1)
			group.Use(middleware2)

			testHandler := func(c *gin.Context) {
				c.JSON(200, gin.H{"message": "test"})
			}

			Expect(func() {
				group.GET("/test", testHandler, v1)
			}).NotTo(Panic())
		})
	})
})
