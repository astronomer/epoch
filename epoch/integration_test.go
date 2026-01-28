package epoch

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http/httptest"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test models - HEAD version only (what controllers actually work with)
// Migrations describe how older versions differ from HEAD

// Role - nested array item for User.Roles
type Role struct {
	Name     string `json:"name"`     // Renamed to "role_name" in older versions
	Priority int    `json:"priority"` // Added in V2
}

// Profile - nested object for User
type Profile struct {
	Bio    string `json:"bio"`    // Renamed to "biography" in older versions
	Avatar string `json:"avatar"` // Added in V2
}

// User - HEAD version has all fields, extended with nested object and array
type User struct {
	ID       int      `json:"id"`
	FullName string   `json:"full_name"` // V2 renamed from "name"
	Email    string   `json:"email"`     // Added in V2
	Phone    string   `json:"phone"`     // Added in V3
	Profile  Profile  `json:"profile"`   // Nested object
	Roles    []Role   `json:"roles"`     // Nested array
	Tags     []string `json:"tags"`      // Array of primitives
}

// ProductMetadata - nested object for Product
type ProductMetadata struct {
	SKU      string `json:"sku"`
	Supplier string `json:"supplier"` // Renamed to "vendor" in older versions
}

// Product - HEAD version, extended with nested object
type Product struct {
	ID          int             `json:"id"`
	Name        string          `json:"name"`
	Price       float64         `json:"price"`
	Currency    string          `json:"currency"`    // Added in V2
	Description string          `json:"description"` // Added in V3
	Metadata    ProductMetadata `json:"metadata"`    // Nested object
}

// CreateUserRequest - HEAD version
type CreateUserRequest struct {
	FullName string  `json:"full_name"`
	Email    string  `json:"email"`
	Phone    string  `json:"phone"`
	Profile  Profile `json:"profile"` // Nested object in request
}

// ListMetadata - nested object alongside arrays
type ListMetadata struct {
	Page      int    `json:"page"`
	PerPage   int    `json:"per_page"`
	UpdatedBy string `json:"updated_by"` // Renamed to "author" in older versions
}

// UsersListResponse - wrapper with nested array and nested object
type UsersListResponse struct {
	Users    []User       `json:"users"`
	Total    int          `json:"total"`
	Metadata ListMetadata `json:"metadata"` // Nested object alongside array
}

// Helper functions for test setup
func setupBasicEpoch(versions []*Version, changes []*VersionChange) (*Epoch, error) {
	builder := NewEpoch().WithVersions(versions...).WithVersionFormat(VersionFormatDate)
	if len(changes) > 0 {
		builder = builder.WithChanges(changes...)
	}
	return builder.Build()
}

func setupRouterWithMiddleware(epochInstance *Epoch) *gin.Engine {
	router := gin.New()
	router.Use(epochInstance.Middleware())
	return router
}

var _ = Describe("End-to-End Integration Tests", func() {
	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
	})

	Describe("Basic Single-Type Migrations", func() {
		It("should migrate field additions between versions", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// V1→V2: Add email field
			change := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			// Handler works with HEAD version (has email)
			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var user map[string]interface{}
				if err := c.ShouldBindJSON(&user); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				Expect(user).To(HaveKey("full_name"))
				Expect(user).To(HaveKey("email"))

				c.JSON(200, gin.H{
					"id":        1,
					"full_name": user["full_name"],
					"email":     user["email"],
					"phone":     "+1-555-0100",
				})
			}).Accepts(User{}).Returns(User{}).ToHandlerFunc("POST", "/users"))

			// Test with V1 client (no email)
			reqBody := map[string]interface{}{
				"full_name": "John Doe",
				"phone":     "+1-555-0100",
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/users", bytes.NewReader(bodyBytes))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// V1 response should NOT have email field
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("full_name"))
			Expect(response).NotTo(HaveKey("email"))
			Expect(response["full_name"]).To(Equal("John Doe"))
		})

		It("should handle field renaming (V1 name to V2 full_name)", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				Description("Rename name to full_name").
				ForType(User{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var user map[string]interface{}
				c.ShouldBindJSON(&user)

				Expect(user).To(HaveKey("full_name"))
				Expect(user).NotTo(HaveKey("name"))

				c.JSON(200, gin.H{
					"id":        1,
					"full_name": user["full_name"],
					"email":     "test@example.com",
					"phone":     "+1-555-0100",
				})
			}).Accepts(User{}).Returns(User{}).ToHandlerFunc("POST", "/users"))

			// V1 client sends "name"
			reqBody := `{"name": "Alice Johnson"}`
			req := httptest.NewRequest("POST", "/users", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			// V1 response should have "name", not "full_name"
			Expect(response).To(HaveKey("name"))
			Expect(response).NotTo(HaveKey("full_name"))
			Expect(response["name"]).To(Equal("Alice Johnson"))
		})

		It("should support semver version format", func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")

			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "default@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatSemver).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users/1", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"id":        1,
					"full_name": "Bob",
					"email":     "bob@example.com",
					"phone":     "+1-555-0200",
				})
			}).Returns(User{}).ToHandlerFunc("GET", "/users/1"))

			req := httptest.NewRequest("GET", "/users/1", nil)
			req.Header.Set("X-API-Version", "1.0.0")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response).NotTo(HaveKey("email"))
		})
	})

	Describe("Array Response Handling", func() {
		It("should handle top-level array responses", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "full_name": "Alice", "email": "alice@example.com", "phone": "+1-555-0100"},
					{"id": 2, "full_name": "Bob", "email": "bob@example.com", "phone": "+1-555-0200"},
				})
			}).Returns([]User{}).ToHandlerFunc("GET", "/users"))

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response []map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response).To(HaveLen(2))
			Expect(response[0]).NotTo(HaveKey("email"))
			Expect(response[1]).NotTo(HaveKey("email"))
			Expect(response[0]["full_name"]).To(Equal("Alice"))
		})

		It("should handle nested array fields with automatic type discovery", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"users": []gin.H{
						{"id": 1, "full_name": "Alice", "email": "alice@example.com", "phone": "+1-555-0100"},
						{"id": 2, "full_name": "Bob", "email": "bob@example.com", "phone": "+1-555-0200"},
					},
					"total": 2,
				})
			}).Returns(UsersListResponse{}).ToHandlerFunc("GET", "/users"))

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response).To(HaveKey("users"))
			Expect(response).To(HaveKey("total"))
			Expect(response["total"]).To(Equal(float64(2)))

			users := response["users"].([]interface{})
			Expect(users).To(HaveLen(2))

			user0 := users[0].(map[string]interface{})
			user1 := users[1].(map[string]interface{})

			Expect(user0).NotTo(HaveKey("email"))
			Expect(user1).NotTo(HaveKey("email"))
			Expect(user0["full_name"]).To(Equal("Alice"))
		})
	})

	Describe("Multi-Step Migration Chains", func() {
		It("should migrate through three versions (V1->V2->V3)", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")

			// V1->V2: Add currency
			change1 := NewVersionChangeBuilder(v1, v2).
				ForType(Product{}).
				RequestToNextVersion().
				AddField("currency", "USD").
				ResponseToPreviousVersion().
				RemoveField("currency").
				Build()

			// V2->V3: Add description
			change2 := NewVersionChangeBuilder(v2, v3).
				ForType(Product{}).
				RequestToNextVersion().
				AddField("description", "").
				ResponseToPreviousVersion().
				RemoveField("description").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2, v3}, []*VersionChange{change1, change2})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/products", epochInstance.WrapHandler(func(c *gin.Context) {
				var product map[string]interface{}
				c.ShouldBindJSON(&product)

				Expect(product).To(HaveKey("currency"))
				Expect(product).To(HaveKey("description"))

				c.JSON(200, gin.H{
					"id":          1,
					"name":        product["name"],
					"price":       product["price"],
					"currency":    "USD",
					"description": "High-performance laptop",
				})
			}).Accepts(Product{}).Returns(Product{}).ToHandlerFunc("POST", "/products"))

			// V1 client (no currency, no description)
			reqBody := `{"name": "Laptop", "price": 999.99}`
			req := httptest.NewRequest("POST", "/products", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response).To(HaveKey("name"))
			Expect(response).To(HaveKey("price"))
			Expect(response).NotTo(HaveKey("currency"))
			Expect(response).NotTo(HaveKey("description"))
		})

		It("should handle V2 client correctly (intermediate version)", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")

			change1 := NewVersionChangeBuilder(v1, v2).
				ForType(Product{}).
				RequestToNextVersion().
				AddField("currency", "USD").
				ResponseToPreviousVersion().
				RemoveField("currency").
				Build()

			change2 := NewVersionChangeBuilder(v2, v3).
				ForType(Product{}).
				RequestToNextVersion().
				AddField("description", "").
				ResponseToPreviousVersion().
				RemoveField("description").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2, v3}, []*VersionChange{change1, change2})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/products/1", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"id":          1,
					"name":        "Mouse",
					"price":       29.99,
					"currency":    "USD",
					"description": "Wireless mouse",
				})
			}).Returns(Product{}).ToHandlerFunc("GET", "/products/1"))

			req := httptest.NewRequest("GET", "/products/1", nil)
			req.Header.Set("X-API-Version", "2024-06-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response).To(HaveKey("currency"))
			Expect(response).NotTo(HaveKey("description"))
		})
	})

	Describe("Multiple Types in Single Endpoint", func() {
		It("should handle different request and response types", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			reqChange := NewVersionChangeBuilder(v1, v2).
				ForType(CreateUserRequest{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				Build()

			respChange := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				ResponseToPreviousVersion().
				RemoveField("phone").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{reqChange, respChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)

				Expect(req).To(HaveKey("email"))

				c.JSON(200, gin.H{
					"id":        1,
					"full_name": req["full_name"],
					"email":     req["email"],
					"phone":     "+1-555-0100",
				})
			}).Accepts(CreateUserRequest{}).Returns(User{}).ToHandlerFunc("POST", "/users"))

			// V1 client sends request without email
			reqBody := `{"full_name": "Charlie"}`
			req := httptest.NewRequest("POST", "/users", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response).NotTo(HaveKey("phone"))
			Expect(response).To(HaveKey("email"))
		})
	})

	Describe("Error Field Name Transformation", func() {
		var (
			e      *Epoch
			router *gin.Engine
		)

		type ErrorTestRequest struct {
			BetterNewName string `json:"better_new_name" binding:"required"`
			OtherField    string `json:"other_field"`
		}

		type ErrorTestResponse struct {
			ID            int    `json:"id"`
			BetterNewName string `json:"better_new_name"`
			OtherField    string `json:"other_field"`
		}

		setupErrorTestEpoch := func() {
			v1, _ := NewSemverVersion("1.0.0")
			v2, _ := NewSemverVersion("2.0.0")
			v3, _ := NewSemverVersion("3.0.0")

			v1ToV2 := NewVersionChangeBuilder(v1, v2).
				Description("Rename name to newName").
				ForType(ErrorTestRequest{}, ErrorTestResponse{}).
				RequestToNextVersion().
				RenameField("name", "new_name").
				ResponseToPreviousVersion().
				RenameField("new_name", "name").
				Build()

			v2ToV3 := NewVersionChangeBuilder(v2, v3).
				Description("Rename newName to betterNewName").
				ForType(ErrorTestRequest{}, ErrorTestResponse{}).
				RequestToNextVersion().
				RenameField("new_name", "better_new_name").
				ResponseToPreviousVersion().
				RenameField("better_new_name", "new_name").
				Build()

			var err error
			e, err = NewEpoch().
				WithVersions(v1, v2, v3).
				WithHeadVersion().
				WithChanges(v1ToV2, v2ToV3).
				WithVersionParameter("X-API-Version").
				Build()
			Expect(err).NotTo(HaveOccurred())

			gin.SetMode(gin.TestMode)
			router = gin.New()
			router.Use(e.Middleware())
		}

		BeforeEach(func() {
			setupErrorTestEpoch()
		})

		Context("Validation Error Transformation", func() {
			It("should transform field names in Gin validation errors for v1 clients", func() {
				testRouter := gin.New()
				testRouter.Use(e.Middleware())
				testRouter.POST("/test", e.WrapHandler(func(c *gin.Context) {
					var req ErrorTestRequest
					if err := c.ShouldBindJSON(&req); err != nil {
						c.JSON(400, gin.H{"error": err.Error()})
						return
					}
					c.JSON(200, gin.H{"message": "success"})
				}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

				w := httptest.NewRecorder()
				reqBody := strings.NewReader("{}")
				req := httptest.NewRequest("POST", "/test", reqBody)
				req.Header.Set("X-API-Version", "1.0.0")
				req.Header.Set("Content-Type", "application/json")

				testRouter.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(400))
				body := w.Body.String()

				Expect(body).To(ContainSubstring("Name"))
				Expect(body).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should transform field names for v2 clients", func() {
				testRouter := gin.New()
				testRouter.Use(e.Middleware())
				testRouter.POST("/test", e.WrapHandler(func(c *gin.Context) {
					var req ErrorTestRequest
					if err := c.ShouldBindJSON(&req); err != nil {
						c.JSON(400, gin.H{"error": err.Error()})
						return
					}
					c.JSON(200, gin.H{"message": "success"})
				}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

				w := httptest.NewRecorder()
				reqBody := strings.NewReader("{}")
				req := httptest.NewRequest("POST", "/test", reqBody)
				req.Header.Set("X-API-Version", "2.0.0")
				req.Header.Set("Content-Type", "application/json")

				testRouter.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(400))
				body := w.Body.String()

				Expect(body).To(ContainSubstring("NewName"))
				Expect(body).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should not transform for HEAD version clients", func() {
				testRouter := gin.New()
				testRouter.Use(e.Middleware())
				testRouter.POST("/test", e.WrapHandler(func(c *gin.Context) {
					var req ErrorTestRequest
					if err := c.ShouldBindJSON(&req); err != nil {
						c.JSON(400, gin.H{"error": err.Error()})
						return
					}
					c.JSON(200, gin.H{"message": "success"})
				}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

				w := httptest.NewRecorder()
				reqBody := strings.NewReader("{}")
				req := httptest.NewRequest("POST", "/test", reqBody)
				req.Header.Set("X-API-Version", "3.0.0")
				req.Header.Set("Content-Type", "application/json")

				testRouter.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(400))
				body := w.Body.String()

				Expect(body).To(ContainSubstring("BetterNewName"))
			})
		})

		Context("Custom Error Message Transformation", func() {
			It("should transform field names in custom string errors", func() {
				testRouter := gin.New()
				testRouter.Use(e.Middleware())
				testRouter.POST("/test", e.WrapHandler(func(c *gin.Context) {
					c.JSON(400, gin.H{
						"error": "Missing fields: better_new_name",
					})
				}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

				w := httptest.NewRecorder()
				req := httptest.NewRequest("POST", "/test", nil)
				req.Header.Set("X-API-Version", "1.0.0")
				req.Header.Set("Content-Type", "application/json")

				testRouter.ServeHTTP(w, req)

				Expect(w.Code).To(Equal(400))

				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				Expect(err).NotTo(HaveOccurred())

				errorMsg, ok := response["error"].(string)
				Expect(ok).To(BeTrue())
				Expect(errorMsg).To(ContainSubstring("name"))
				Expect(errorMsg).NotTo(ContainSubstring("better_new_name"))
			})
		})

		It("should transform field names in structured error objects", func() {
			router.POST("/test", e.WrapHandler(func(c *gin.Context) {
				c.JSON(400, gin.H{
					"error": map[string]interface{}{
						"message": "Validation failed for field: better_new_name",
						"code":    "VALIDATION_ERROR",
					},
				})
			}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

			w := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/test", nil)
			req.Header.Set("X-API-Version", "2.0.0")
			req.Header.Set("Content-Type", "application/json")

			router.ServeHTTP(w, req)

			Expect(w.Code).To(Equal(400))

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			errorObj, ok := response["error"].(map[string]interface{})
			Expect(ok).To(BeTrue())

			message, ok := errorObj["message"].(string)
			Expect(ok).To(BeTrue())
			Expect(message).To(ContainSubstring("new_name"))
			Expect(message).NotTo(ContainSubstring("better_new_name"))
		})

		Context("Non-Standard Error Formats", func() {
			It("should transform root-level message field for v1", func() {
				testRouter := gin.New()
				testRouter.Use(e.Middleware())
				testRouter.POST("/test", e.WrapHandler(func(c *gin.Context) {
					c.JSON(400, gin.H{
						"message":    "Missing required field: BetterNewName",
						"statusCode": 400,
						"timestamp":  "2024-01-01T00:00:00Z",
					})
				}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

				reqBody := strings.NewReader("{}")
				req := httptest.NewRequest("POST", "/test", reqBody)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-API-Version", "1.0.0")
				w := httptest.NewRecorder()

				testRouter.ServeHTTP(w, req)

				body := w.Body.String()
				Expect(w.Code).To(Equal(400))
				Expect(body).To(ContainSubstring("Name"))
				Expect(body).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should transform RFC 7807 Problem Details for v2", func() {
				testRouter := gin.New()
				testRouter.Use(e.Middleware())
				testRouter.POST("/test", e.WrapHandler(func(c *gin.Context) {
					c.JSON(400, gin.H{
						"type":     "https://example.com/probs/validation-error",
						"title":    "Validation Error",
						"status":   400,
						"detail":   "The field BetterNewName is required but was not provided",
						"instance": "/test",
					})
				}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

				reqBody := strings.NewReader("{}")
				req := httptest.NewRequest("POST", "/test", reqBody)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-API-Version", "2.0.0")
				w := httptest.NewRecorder()

				testRouter.ServeHTTP(w, req)

				body := w.Body.String()
				Expect(w.Code).To(Equal(400))
				Expect(body).To(ContainSubstring("NewName"))
				Expect(body).NotTo(ContainSubstring("BetterNewName"))
			})

			It("should transform nested error structures for v1", func() {
				testRouter := gin.New()
				testRouter.Use(e.Middleware())
				testRouter.POST("/test", e.WrapHandler(func(c *gin.Context) {
					c.JSON(400, gin.H{
						"error": gin.H{
							"code":        "VALIDATION_ERROR",
							"description": "Field BetterNewName is required",
							"details": gin.H{
								"field":  "BetterNewName",
								"reason": "Field BetterNewName cannot be empty",
							},
						},
					})
				}).Accepts(ErrorTestRequest{}).ToHandlerFunc("POST", "/test"))

				reqBody := strings.NewReader("{}")
				req := httptest.NewRequest("POST", "/test", reqBody)
				req.Header.Set("Content-Type", "application/json")
				req.Header.Set("X-API-Version", "1.0.0")
				w := httptest.NewRecorder()

				testRouter.ServeHTTP(w, req)

				body := w.Body.String()
				Expect(w.Code).To(Equal(400))
				Expect(body).To(ContainSubstring("Name"))
				Expect(body).NotTo(ContainSubstring("BetterNewName"))
			})
		})
	})

	Describe("Error Response Handling", func() {
		DescribeTable("should control error migration based on MigrateHTTPErrors flag",
			func(migrateHTTPErrors bool, shouldBeMigrated bool) {
				v1, _ := NewDateVersion("2024-01-01")
				v2, _ := NewDateVersion("2024-06-01")

				change := NewVersionChange(
					"Add version_info field",
					v1, v2,
					&AlterResponseInstruction{
						Schemas:           []interface{}{User{}},
						MigrateHTTPErrors: migrateHTTPErrors,
						Transformer: func(resp *ResponseInfo) error {
							if migrateHTTPErrors {
								resp.DeleteField("version_info")
							} else {
								resp.SetField("migrated", true)
							}
							return nil
						},
					},
				)

				epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
				Expect(err).NotTo(HaveOccurred())

				router := setupRouterWithMiddleware(epochInstance)

				router.GET("/error", epochInstance.WrapHandler(func(c *gin.Context) {
					if migrateHTTPErrors {
						c.JSON(400, gin.H{
							"error":        "Bad request",
							"version_info": "v2",
						})
					} else {
						c.JSON(400, gin.H{"error": "Bad request"})
					}
				}).Returns(User{}).ToHandlerFunc("GET", "/error"))

				req := httptest.NewRequest("GET", "/error", nil)
				req.Header.Set("X-API-Version", "2024-01-01")
				recorder := httptest.NewRecorder()

				router.ServeHTTP(recorder, req)

				Expect(recorder.Code).To(Equal(400))

				var response map[string]interface{}
				json.Unmarshal(recorder.Body.Bytes(), &response)

				if shouldBeMigrated {
					Expect(response).NotTo(HaveKey("version_info"))
				} else {
					Expect(response).NotTo(HaveKey("migrated"))
					Expect(response).To(HaveKey("error"))
				}
			},
			Entry("when MigrateHTTPErrors is false", false, false),
			Entry("when MigrateHTTPErrors is true", true, true),
		)
	})

	Describe("Endpoint Registry Integration", func() {
		It("should handle parameterized paths like /users/:id", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users/:id", epochInstance.WrapHandler(func(c *gin.Context) {
				id := c.Param("id")
				c.JSON(200, gin.H{
					"id":        id,
					"full_name": "Test User",
					"email":     "test@example.com",
					"phone":     "+1-555-0100",
				})
			}).Returns(User{}).ToHandlerFunc("GET", "/users/:id"))

			req := httptest.NewRequest("GET", "/users/123", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			Expect(response["id"]).To(Equal("123"))
			Expect(response).NotTo(HaveKey("email"))
		})

		It("should handle HTTP methods independently", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			userChange := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			createChange := NewVersionChangeBuilder(v1, v2).
				ForType(CreateUserRequest{}).
				RequestToNextVersion().
				AddField("email", "default@example.com").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{userChange, createChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "full_name": "Alice", "email": "alice@example.com", "phone": "+1-555-0100"},
				})
			}).Returns([]User{}).ToHandlerFunc("GET", "/users"))

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)

				Expect(req).To(HaveKey("email"))

				c.JSON(201, gin.H{
					"id":        2,
					"full_name": req["full_name"],
					"email":     req["email"],
					"phone":     "+1-555-0200",
				})
			}).Accepts(CreateUserRequest{}).Returns(User{}).ToHandlerFunc("POST", "/users"))

			getReq := httptest.NewRequest("GET", "/users", nil)
			getReq.Header.Set("X-API-Version", "2024-01-01")
			getRec := httptest.NewRecorder()
			router.ServeHTTP(getRec, getReq)
			Expect(getRec.Code).To(Equal(200))

			postReq := httptest.NewRequest("POST", "/users", strings.NewReader(`{"full_name":"Bob"}`))
			postReq.Header.Set("X-API-Version", "2024-01-01")
			postReq.Header.Set("Content-Type", "application/json")
			postRec := httptest.NewRecorder()
			router.ServeHTTP(postRec, postReq)
			Expect(postRec.Code).To(Equal(201))

			var postResp map[string]interface{}
			json.Unmarshal(postRec.Body.Bytes(), &postResp)
			Expect(postResp).NotTo(HaveKey("email"))
		})
	})

	Describe("Builder Pattern", func() {
		It("should build Epoch instance and handle errors", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChange("Test change", v1, v2)

			instance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionParameter("X-API-Version").
				WithVersionFormat(VersionFormatDate).
				WithDefaultVersion(v1).
				Build()

			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())
			Expect(instance.GetVersions()).To(HaveLen(2))

			// Test error accumulation
			_, err = NewEpoch().
				WithSemverVersions("invalid-version", "1.0.0").
				Build()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid semver"))
		})

		It("should work with convenience functions", func() {
			instance, err := WithSemver("1.0.0", "2.0.0")
			Expect(err).NotTo(HaveOccurred())
			Expect(instance).NotTo(BeNil())

			instance2, err2 := QuickStart("2024-01-01", "2024-06-01")
			Expect(err2).NotTo(HaveOccurred())
			Expect(instance2).NotTo(BeNil())

			instance3, err3 := WithStrings("alpha", "beta", "stable")
			Expect(err3).NotTo(HaveOccurred())
			Expect(instance3).NotTo(BeNil())

			instance4, err4 := Simple()
			Expect(err4).NotTo(HaveOccurred())
			Expect(instance4).NotTo(BeNil())
		})
	})

	Describe("Field Order Preservation", func() {
		It("should preserve field order in API responses", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChange(
				"Remove fields",
				v1, v2,
				&AlterResponseInstruction{
					Schemas: []interface{}{User{}},
					Transformer: func(resp *ResponseInfo) error {
						resp.DeleteField("email")
						resp.DeleteField("phone")
						return nil
					},
				},
			)

			instance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(instance)

			router.GET("/users", instance.WrapHandler(func(c *gin.Context) {
				c.Data(200, "application/json", []byte(`{"zebra": "first", "alpha": "second", "full_name": "John", "email": "john@example.com", "phone": "555-1234"}`))
			}).Returns(User{}).ToHandlerFunc("GET", "/users"))

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			responseBody := recorder.Body.String()

			zebraPos := strings.Index(responseBody, `"zebra"`)
			alphaPos := strings.Index(responseBody, `"alpha"`)
			namePos := strings.Index(responseBody, `"full_name"`)

			Expect(zebraPos).To(BeNumerically("<", alphaPos))
			Expect(alphaPos).To(BeNumerically("<", namePos))

			Expect(responseBody).NotTo(ContainSubstring(`"email"`))
			Expect(responseBody).NotTo(ContainSubstring(`"phone"`))
		})
	})

	Describe("Cycle Detection", func() {
		It("should prevent circular migrations at build time", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change1 := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "test@example.com").
				Build()

			change2 := NewVersionChangeBuilder(v2, v1).
				ForType(User{}).
				RequestToNextVersion().
				RemoveField("email").
				Build()

			_, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change1, change2).
				Build()

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("cycle detected"))
		})

		It("should allow valid linear migration chains", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")

			change1 := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "test@example.com").
				Build()

			change2 := NewVersionChangeBuilder(v2, v3).
				ForType(User{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(change1, change2).
				Build()

			Expect(err).NotTo(HaveOccurred())
			Expect(epochInstance).NotTo(BeNil())
		})
	})

	Describe("Complex Multi-Version Scenarios", func() {
		It("should handle complete real-world API scenario", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")

			userChange1 := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			userChange2 := NewVersionChangeBuilder(v2, v3).
				ForType(User{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				AddField("phone", "").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				RemoveField("phone").
				Build()

			productChange := NewVersionChangeBuilder(v2, v3).
				ForType(Product{}).
				RequestToNextVersion().
				AddField("currency", "USD").
				AddField("description", "").
				ResponseToPreviousVersion().
				RemoveField("currency").
				RemoveField("description").
				Build()

			instance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(userChange1, userChange2, productChange).
				WithVersionFormat(VersionFormatDate).
				WithDefaultVersion(v2).
				Build()

			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(instance)

			router.GET("/users", instance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "full_name": "Alice Johnson", "email": "alice@example.com", "phone": "+1-555-0100"},
					{"id": 2, "full_name": "Bob Smith", "email": "bob@example.com", "phone": "+1-555-0200"},
				})
			}).Returns([]User{}).ToHandlerFunc("GET", "/users"))

			router.GET("/products", instance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "name": "Laptop", "price": 999.99, "currency": "USD", "description": "High-performance"},
					{"id": 2, "name": "Mouse", "price": 29.99, "currency": "USD", "description": "Wireless"},
				})
			}).Returns([]Product{}).ToHandlerFunc("GET", "/products"))

			// Test V1 users
			req1 := httptest.NewRequest("GET", "/users", nil)
			req1.Header.Set("X-API-Version", "2024-01-01")
			rec1 := httptest.NewRecorder()
			router.ServeHTTP(rec1, req1)
			Expect(rec1.Code).To(Equal(200))

			var users1 []map[string]interface{}
			json.Unmarshal(rec1.Body.Bytes(), &users1)
			Expect(users1[0]).To(HaveKey("name"))
			Expect(users1[0]).NotTo(HaveKey("email"))
			Expect(users1[0]).NotTo(HaveKey("phone"))
			Expect(users1[0]).NotTo(HaveKey("full_name"))

			// Test V2 users
			req2 := httptest.NewRequest("GET", "/users", nil)
			req2.Header.Set("X-API-Version", "2024-06-01")
			rec2 := httptest.NewRecorder()
			router.ServeHTTP(rec2, req2)
			Expect(rec2.Code).To(Equal(200))

			var users2 []map[string]interface{}
			json.Unmarshal(rec2.Body.Bytes(), &users2)
			Expect(users2[0]).To(HaveKey("name"))
			Expect(users2[0]).To(HaveKey("email"))
			Expect(users2[0]).NotTo(HaveKey("phone"))
			Expect(users2[0]).NotTo(HaveKey("full_name"))

			// Test V2 products (no currency/description)
			req3 := httptest.NewRequest("GET", "/products", nil)
			req3.Header.Set("X-API-Version", "2024-06-01")
			rec3 := httptest.NewRecorder()
			router.ServeHTTP(rec3, req3)
			Expect(rec3.Code).To(Equal(200))

			var products2 []map[string]interface{}
			json.Unmarshal(rec3.Body.Bytes(), &products2)
			Expect(products2[0]).NotTo(HaveKey("currency"))
			Expect(products2[0]).NotTo(HaveKey("description"))

			// Test HEAD version
			req4 := httptest.NewRequest("GET", "/users", nil)
			req4.Header.Set("X-API-Version", "head")
			rec4 := httptest.NewRecorder()
			router.ServeHTTP(rec4, req4)
			Expect(rec4.Code).To(Equal(200))

			var usersHead []map[string]interface{}
			json.Unmarshal(rec4.Body.Bytes(), &usersHead)
			Expect(usersHead[0]).To(HaveKey("full_name"))
			Expect(usersHead[0]).To(HaveKey("email"))
			Expect(usersHead[0]).To(HaveKey("phone"))
		})
	})

	Describe("Response Utility Pattern and HEAD Version Migration", func() {
		It("should handle response utilities and version migrations correctly", func() {
			v1, _ := NewSemverVersion("1.0")
			v2, _ := NewSemverVersion("2.0")

			change := NewVersionChangeBuilder(v1, v2).
				Description("Add phone field to User").
				ForType(User{}).
				RequestToNextVersion().
				AddField("phone", "+1-555-0000").
				ResponseToPreviousVersion().
				RemoveField("phone").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithHeadVersion().
				WithChanges(change).
				WithVersionFormat(VersionFormatSemver).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			testResponseOk := func(ctx *gin.Context, data interface{}) {
				if data != nil {
					ctx.JSON(200, data)
				} else {
					ctx.Status(200)
				}
			}

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				resp := UsersListResponse{
					Users: []User{
						{ID: 1, FullName: "Alice", Email: "alice@example.com", Phone: "+1-555-0100"},
						{ID: 2, FullName: "Bob", Email: "bob@example.com", Phone: "+1-555-0200"},
					},
					Total: 2,
				}
				testResponseOk(c, resp)
			}).Returns(UsersListResponse{}).
				ToHandlerFunc("GET", "/users"))

			// Test with v1.0
			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "1.0")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			Expect(recorder.Body.Len()).To(BeNumerically(">", 0), "Response body should not be empty")

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			Expect(response).To(HaveKey("users"))
			Expect(response).To(HaveKey("total"))

			users, ok := response["users"].([]interface{})
			Expect(ok).To(BeTrue())
			Expect(len(users)).To(Equal(2))

			user1, ok := users[0].(map[string]interface{})
			Expect(ok).To(BeTrue())
			Expect(user1).NotTo(HaveKey("phone"), "V1 should not have phone field")

			// Test with HEAD version
			req2 := httptest.NewRequest("GET", "/users", nil)
			req2.Header.Set("X-API-Version", "head")
			recorder2 := httptest.NewRecorder()

			router.ServeHTTP(recorder2, req2)

			Expect(recorder2.Code).To(Equal(200))

			var response2 map[string]interface{}
			json.Unmarshal(recorder2.Body.Bytes(), &response2)

			users2, _ := response2["users"].([]interface{})
			user2, _ := users2[0].(map[string]interface{})
			Expect(user2).To(HaveKey("phone"), "HEAD version should have phone field")
		})

		It("should handle POST requests with single version and HEAD", func() {
			v1, _ := NewSemverVersion("1.0")
			v2, _ := NewSemverVersion("2.0")

			change := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "default@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithHeadVersion().
				WithChanges(change).
				WithVersionFormat(VersionFormatSemver).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var user map[string]interface{}
				if err := c.ShouldBindJSON(&user); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				Expect(user).To(HaveKey("email"), "Handler should receive migrated request with email field")

				c.JSON(201, gin.H{
					"id":    1,
					"name":  user["full_name"],
					"email": user["email"],
				})
			}).Accepts(User{}).Returns(User{}).ToHandlerFunc("POST", "/users"))

			// POST request from v1.0 (without email)
			reqBody := map[string]interface{}{
				"full_name": "Jane Doe",
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/users", bytes.NewReader(bodyBytes))
			req.Header.Set("X-API-Version", "1.0")
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()

			router.ServeHTTP(rec, req)

			Expect(rec.Code).To(Equal(201), "POST request should succeed after migration")

			var response map[string]interface{}
			json.Unmarshal(rec.Body.Bytes(), &response)
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("name"))
			Expect(response).NotTo(HaveKey("email"), "v1.0 response should have email removed by migration")
		})
	})

	Describe("Request Transformation Edge Cases", func() {
		It("should transform top-level array request body", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Simple user type for array body test
			type SimpleUser struct {
				ID   int    `json:"id"`
				Name string `json:"name"` // Renamed from "user_name" in older version
			}

			change := NewVersionChangeBuilder(v1, v2).
				ForType(SimpleUser{}).
				RequestToNextVersion().
				RenameField("user_name", "name").
				ResponseToPreviousVersion().
				RenameField("name", "user_name").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/users/bulk", epochInstance.WrapHandler(func(c *gin.Context) {
				var users []map[string]interface{}
				if err := c.ShouldBindJSON(&users); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				// Return the processed users
				c.JSON(200, users)
			}).Accepts([]SimpleUser{}).Returns([]SimpleUser{}).ToHandlerFunc("POST", "/users/bulk"))

			// V1 client sends array with "user_name"
			reqBody := `[
				{"id": 1, "user_name": "John"},
				{"id": 2, "user_name": "Jane"}
			]`
			req := httptest.NewRequest("POST", "/users/bulk", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response []map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Response for V1 should have "user_name" back
			Expect(response).To(HaveLen(2))
			Expect(response[0]).To(HaveKey("user_name"))
			Expect(response[0]).NotTo(HaveKey("name"))
		})

		It("should auto-transform nested objects in requests", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Define change for CreateUserRequest (top-level) and Profile (nested object)
			requestChange := NewVersionChangeBuilder(v1, v2).
				ForType(CreateUserRequest{}).
				RequestToNextVersion().
				RenameField("full_name", "display_name"). // Top-level rename
				Build()

			// Separate change for nested Profile type
			profileChange := NewVersionChangeBuilder(v1, v2).
				ForType(Profile{}).
				RequestToNextVersion().
				RenameField("biography", "bio"). // Nested object rename
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{requestChange, profileChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)
				c.JSON(200, req)
			}).Accepts(CreateUserRequest{}).Returns(CreateUserRequest{}).ToHandlerFunc("POST", "/users"))

			// V1 client sends request with nested profile containing "biography" (older field name)
			reqBody := `{
				"full_name": "John Doe",
				"email": "john@example.com",
				"phone": "+1-555-0100",
				"profile": {
					"biography": "Software Engineer",
					"avatar": "avatar.png"
				}
			}`
			req := httptest.NewRequest("POST", "/users", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Top-level field should be transformed
			Expect(response).To(HaveKey("display_name"))
			Expect(response).NotTo(HaveKey("full_name"))

			// Nested profile.biography should NOW be transformed to bio
			profile := response["profile"].(map[string]interface{})
			Expect(profile).To(HaveKey("bio"), "nested fields should be transformed")
			Expect(profile).NotTo(HaveKey("biography"), "old nested field name should be removed")
			Expect(profile["bio"]).To(Equal("Software Engineer"))
		})

		It("should auto-transform nested arrays in requests", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Define change for User (used as request type) and Role (nested array item)
			userChange := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				RenameField("name", "full_name"). // Top-level rename
				Build()

			// Separate change for nested Role type in roles[] array
			roleChange := NewVersionChangeBuilder(v1, v2).
				ForType(Role{}).
				RequestToNextVersion().
				RenameField("role_name", "name"). // Nested array item rename
				AddField("priority", 0).          // Add default priority
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{userChange, roleChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)
				c.JSON(200, req)
			}).Accepts(User{}).Returns(User{}).ToHandlerFunc("POST", "/users"))

			// V1 client sends request with nested roles[] containing "role_name" (older field name)
			reqBody := `{
				"name": "John Doe",
				"email": "john@example.com",
				"phone": "+1-555-0100",
				"profile": {"bio": "Engineer", "avatar": "avatar.png"},
				"roles": [
					{"role_name": "admin"},
					{"role_name": "developer"}
				],
				"tags": ["go", "rust"]
			}`
			req := httptest.NewRequest("POST", "/users", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Top-level field should be transformed
			Expect(response).To(HaveKey("full_name"))
			Expect(response).NotTo(HaveKey("name"))
			Expect(response["full_name"]).To(Equal("John Doe"))

			// Nested roles[].role_name should NOW be transformed to name
			roles := response["roles"].([]interface{})
			Expect(roles).To(HaveLen(2))

			role0 := roles[0].(map[string]interface{})
			Expect(role0).To(HaveKey("name"), "nested array item fields should be transformed")
			Expect(role0).NotTo(HaveKey("role_name"), "old nested array item field name should be removed")
			Expect(role0["name"]).To(Equal("admin"))
			Expect(role0).To(HaveKey("priority"), "added field should be present")

			role1 := roles[1].(map[string]interface{})
			Expect(role1).To(HaveKey("name"))
			Expect(role1["name"]).To(Equal("developer"))
		})
	})

	Describe("Array of Primitives Handling", func() {
		It("should preserve array of primitives unchanged during transformation", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users/1", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"id":        1,
					"full_name": "John",
					"email":     "john@example.com",
					"phone":     "+1-555-0100",
					"profile":   gin.H{"bio": "Engineer", "avatar": "avatar.png"},
					"roles":     []gin.H{{"name": "admin", "priority": 1}},
					"tags":      []string{"go", "rust", "python"}, // Array of primitives
				})
			}).Returns(User{}).ToHandlerFunc("GET", "/users/1"))

			req := httptest.NewRequest("GET", "/users/1", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Tags (array of primitives) should be preserved
			Expect(response).To(HaveKey("tags"))
			tags := response["tags"].([]interface{})
			Expect(tags).To(HaveLen(3))
			Expect(tags[0]).To(Equal("go"))
			Expect(tags[1]).To(Equal("rust"))
			Expect(tags[2]).To(Equal("python"))

			// Top-level transformation should work
			Expect(response).To(HaveKey("name"))
			Expect(response).NotTo(HaveKey("full_name"))
		})
	})

	Describe("Nested Object and Array Auto-Discovery", func() {
		It("should transform nested arrays using auto-discovered types (single object response)", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Migration for Role type in nested array
			roleChange := NewVersionChangeBuilder(v1, v2).
				ForType(Role{}).
				ResponseToPreviousVersion().
				RenameField("name", "role_name").
				RemoveField("priority").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{roleChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users/1", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"id":        1,
					"full_name": "John",
					"email":     "john@example.com",
					"phone":     "+1-555-0100",
					"profile":   gin.H{"bio": "Engineer", "avatar": "avatar.png"},
					"roles": []gin.H{
						{"name": "admin", "priority": 1},
						{"name": "user", "priority": 2},
					},
					"tags": []string{"go"},
				})
			}).Returns(User{}).ToHandlerFunc("GET", "/users/1"))

			req := httptest.NewRequest("GET", "/users/1", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Roles should be transformed via auto-discovery
			Expect(response).To(HaveKey("roles"))
			roles := response["roles"].([]interface{})
			Expect(roles).To(HaveLen(2))

			role0 := roles[0].(map[string]interface{})
			Expect(role0).To(HaveKey("role_name"))
			Expect(role0).NotTo(HaveKey("name"))
			Expect(role0).NotTo(HaveKey("priority"))
		})

		It("should transform metadata nested object using auto-discovered types", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Migration for ListMetadata type
			metadataChange := NewVersionChangeBuilder(v1, v2).
				ForType(ListMetadata{}).
				ResponseToPreviousVersion().
				RenameField("updated_by", "author").
				Build()

			// Migration for User in array
			userChange := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				ResponseToPreviousVersion().
				RemoveField("phone").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{metadataChange, userChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"users": []gin.H{
						{"id": 1, "full_name": "Alice", "email": "alice@example.com", "phone": "+1-555-0100", "profile": gin.H{}, "roles": []gin.H{}, "tags": []string{}},
					},
					"total": 1,
					"metadata": gin.H{
						"page":       1,
						"per_page":   10,
						"updated_by": "admin",
					},
				})
			}).Returns(UsersListResponse{}).ToHandlerFunc("GET", "/users"))

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Metadata should be transformed via auto-discovery
			metadata := response["metadata"].(map[string]interface{})
			Expect(metadata).To(HaveKey("author"))
			Expect(metadata).NotTo(HaveKey("updated_by"))

			// Users array should also be transformed
			users := response["users"].([]interface{})
			user0 := users[0].(map[string]interface{})
			Expect(user0).NotTo(HaveKey("phone"))
		})

		It("should recursively transform nested arrays inside list response items (users[].roles[])", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Migration for User in array
			userChange := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			// Migration for Role type in deeply nested array (users[].roles[])
			roleChange := NewVersionChangeBuilder(v1, v2).
				ForType(Role{}).
				ResponseToPreviousVersion().
				RenameField("name", "role_name").
				RemoveField("priority").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{userChange, roleChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"users": []gin.H{
						{
							"id":        1,
							"full_name": "Alice",
							"email":     "alice@example.com",
							"phone":     "+1-555-0100",
							"profile":   gin.H{"bio": "Senior Engineer", "avatar": "alice.png"},
							"roles": []gin.H{
								{"name": "admin", "priority": 1},
								{"name": "developer", "priority": 2},
							},
							"tags": []string{"go", "rust"},
						},
						{
							"id":        2,
							"full_name": "Bob",
							"email":     "bob@example.com",
							"phone":     "+1-555-0200",
							"profile":   gin.H{"bio": "Junior Dev", "avatar": "bob.png"},
							"roles": []gin.H{
								{"name": "developer", "priority": 3},
							},
							"tags": []string{"python"},
						},
					},
					"total":    2,
					"metadata": gin.H{"page": 1, "per_page": 10, "updated_by": "system"},
				})
			}).Returns(UsersListResponse{}).ToHandlerFunc("GET", "/users"))

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Verify users array exists
			users := response["users"].([]interface{})
			Expect(users).To(HaveLen(2))

			// Check first user - email should be removed
			user0 := users[0].(map[string]interface{})
			Expect(user0).NotTo(HaveKey("email"))
			Expect(user0).To(HaveKey("full_name"))

			// CRITICAL: Verify nested roles[] array inside users[] is transformed recursively
			roles0 := user0["roles"].([]interface{})
			Expect(roles0).To(HaveLen(2))

			role00 := roles0[0].(map[string]interface{})
			Expect(role00).To(HaveKey("role_name"), "roles[] inside users[] should be transformed to role_name")
			Expect(role00["role_name"]).To(Equal("admin"))
			Expect(role00).NotTo(HaveKey("name"), "old field name should be removed")
			Expect(role00).NotTo(HaveKey("priority"), "priority field should be removed for v1")

			// Check second role in first user
			role01 := roles0[1].(map[string]interface{})
			Expect(role01).To(HaveKey("role_name"))
			Expect(role01["role_name"]).To(Equal("developer"))

			// Check second user's roles
			user1 := users[1].(map[string]interface{})
			roles1 := user1["roles"].([]interface{})
			Expect(roles1).To(HaveLen(1))

			role10 := roles1[0].(map[string]interface{})
			Expect(role10).To(HaveKey("role_name"))
			Expect(role10["role_name"]).To(Equal("developer"))
			Expect(role10).NotTo(HaveKey("priority"))
		})

		It("should recursively transform nested objects inside list response items (users[].profile)", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Migration for Profile type nested in users[]
			profileChange := NewVersionChangeBuilder(v1, v2).
				ForType(Profile{}).
				ResponseToPreviousVersion().
				RenameField("bio", "biography").
				RemoveField("avatar").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{profileChange})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"users": []gin.H{
						{
							"id":        1,
							"full_name": "Alice",
							"email":     "alice@example.com",
							"phone":     "+1-555-0100",
							"profile":   gin.H{"bio": "Senior Engineer", "avatar": "alice.png"},
							"roles":     []gin.H{},
							"tags":      []string{},
						},
						{
							"id":        2,
							"full_name": "Bob",
							"email":     "bob@example.com",
							"phone":     "+1-555-0200",
							"profile":   gin.H{"bio": "Junior Dev", "avatar": "bob.png"},
							"roles":     []gin.H{},
							"tags":      []string{},
						},
					},
					"total":    2,
					"metadata": gin.H{"page": 1, "per_page": 10, "updated_by": "system"},
				})
			}).Returns(UsersListResponse{}).ToHandlerFunc("GET", "/users"))

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			users := response["users"].([]interface{})
			Expect(users).To(HaveLen(2))

			// CRITICAL: Verify nested profile object inside users[] is transformed recursively
			user0 := users[0].(map[string]interface{})
			profile0 := user0["profile"].(map[string]interface{})
			Expect(profile0).To(HaveKey("biography"), "profile.bio should be renamed to biography for v1")
			Expect(profile0["biography"]).To(Equal("Senior Engineer"))
			Expect(profile0).NotTo(HaveKey("bio"), "new field name should not exist in v1")
			Expect(profile0).NotTo(HaveKey("avatar"), "avatar should be removed for v1")

			// Verify second user's profile too
			user1 := users[1].(map[string]interface{})
			profile1 := user1["profile"].(map[string]interface{})
			Expect(profile1).To(HaveKey("biography"))
			Expect(profile1["biography"]).To(Equal("Junior Dev"))
			Expect(profile1).NotTo(HaveKey("bio"))
			Expect(profile1).NotTo(HaveKey("avatar"))
		})
	})

	Describe("Auto-Capture Field Preservation", func() {
		// AutoCaptureRequest is the request type for auto-capture tests
		type AutoCaptureRequest struct {
			Name        string `json:"name"`
			Description string `json:"description"` // This field is removed in v2 but we want to preserve the value
			Metadata    string `json:"metadata"`    // Another field to test multiple captures
		}

		// AutoCaptureResponse is the response type for auto-capture tests
		type AutoCaptureResponse struct {
			ID          int    `json:"id"`
			Name        string `json:"name"`
			Description string `json:"description"` // Added back in response with captured value
			Metadata    string `json:"metadata"`    // Added back in response with captured value
			CreatedAt   string `json:"created_at"`
		}

		It("should preserve field values from request in response when using RemoveField/AddField", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// V1→V2: Remove deprecated fields from request, add them back in response
			// The auto-capture feature should preserve the original values
			change := NewVersionChangeBuilder(v1, v2).
				Description("Remove description and metadata fields").
				ForType(AutoCaptureRequest{}).
				RequestToNextVersion().
				RemoveField("description"). // Captured automatically
				RemoveField("metadata").    // Captured automatically
				ForType(AutoCaptureResponse{}).
				ResponseToPreviousVersion().
				AddField("description", "default description"). // Uses captured value instead
				AddField("metadata", "default metadata").       // Uses captured value instead
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			// Handler receives HEAD version (without description/metadata)
			// and returns HEAD version (without description/metadata)
			router.POST("/items", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				if err := c.ShouldBindJSON(&req); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				// Verify handler does NOT receive the deprecated fields
				Expect(req).NotTo(HaveKey("description"), "Handler should not receive removed field")
				Expect(req).NotTo(HaveKey("metadata"), "Handler should not receive removed field")
				Expect(req).To(HaveKey("name"))

				// Return response without deprecated fields (HEAD version)
				c.JSON(200, gin.H{
					"id":         1,
					"name":       req["name"],
					"created_at": "2024-01-15T10:00:00Z",
				})
			}).Accepts(AutoCaptureRequest{}).Returns(AutoCaptureResponse{}).ToHandlerFunc("POST", "/items"))

			// V1 client sends request WITH deprecated fields
			reqBody := `{
				"name": "Test Item",
				"description": "My custom description",
				"metadata": "Important metadata value"
			}`
			req := httptest.NewRequest("POST", "/items", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// V1 response should have the ORIGINAL values from the request, NOT defaults
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("name"))
			Expect(response).To(HaveKey("description"))
			Expect(response).To(HaveKey("metadata"))
			Expect(response).To(HaveKey("created_at"))

			// These should be the captured values, not the defaults
			Expect(response["description"]).To(Equal("My custom description"),
				"Should use captured value from request, not default")
			Expect(response["metadata"]).To(Equal("Important metadata value"),
				"Should use captured value from request, not default")
		})

		It("should use defaults when request doesn't have the field", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				Description("Handle optional field").
				ForType(AutoCaptureRequest{}).
				RequestToNextVersion().
				RemoveField("description").
				ForType(AutoCaptureResponse{}).
				ResponseToPreviousVersion().
				AddField("description", "default description").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/items", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)

				c.JSON(200, gin.H{
					"id":         1,
					"name":       req["name"],
					"created_at": "2024-01-15T10:00:00Z",
				})
			}).Accepts(AutoCaptureRequest{}).Returns(AutoCaptureResponse{}).ToHandlerFunc("POST", "/items"))

			// V1 client sends request WITHOUT the optional field
			reqBody := `{"name": "Test Item"}`
			req := httptest.NewRequest("POST", "/items", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			// Should use default since field wasn't in request
			Expect(response["description"]).To(Equal("default description"),
				"Should use default when field wasn't in request")
		})

		It("should not override handler-set values with captured values", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				Description("Handler override test").
				ForType(AutoCaptureRequest{}).
				RequestToNextVersion().
				RemoveField("description").
				ForType(AutoCaptureResponse{}).
				ResponseToPreviousVersion().
				AddField("description", "default description").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/items", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)

				// Handler explicitly sets description in response
				c.JSON(200, gin.H{
					"id":          1,
					"name":        req["name"],
					"description": "Handler-set description", // Explicit value
					"created_at":  "2024-01-15T10:00:00Z",
				})
			}).Accepts(AutoCaptureRequest{}).Returns(AutoCaptureResponse{}).ToHandlerFunc("POST", "/items"))

			reqBody := `{
				"name": "Test Item",
				"description": "Request description"
			}`
			req := httptest.NewRequest("POST", "/items", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			// Handler's value should be preserved, not overwritten by captured value
			Expect(response["description"]).To(Equal("Handler-set description"),
				"Handler-set value should take precedence")
		})

		It("should preserve captured values for complex types", func() {
			// Test type with complex field
			type ComplexRequest struct {
				Name     string                 `json:"name"`
				Settings map[string]interface{} `json:"settings"`
			}

			type ComplexResponse struct {
				ID       int                    `json:"id"`
				Name     string                 `json:"name"`
				Settings map[string]interface{} `json:"settings"`
			}

			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				Description("Complex field preservation").
				ForType(ComplexRequest{}).
				RequestToNextVersion().
				RemoveField("settings").
				ForType(ComplexResponse{}).
				ResponseToPreviousVersion().
				AddField("settings", map[string]interface{}{"default": true}).
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/complex", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)

				Expect(req).NotTo(HaveKey("settings"))

				c.JSON(200, gin.H{
					"id":   1,
					"name": req["name"],
				})
			}).Accepts(ComplexRequest{}).Returns(ComplexResponse{}).ToHandlerFunc("POST", "/complex"))

			reqBody := `{
				"name": "Test",
				"settings": {
					"theme": "dark",
					"notifications": true,
					"nested": {"key": "value"}
				}
			}`
			req := httptest.NewRequest("POST", "/complex", strings.NewReader(reqBody))
			req.Header.Set("X-API-Version", "2024-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			// Complex object should be preserved
			settings, ok := response["settings"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "settings should be a map")
			Expect(settings["theme"]).To(Equal("dark"))
			Expect(settings["notifications"]).To(Equal(true))

			nested, ok := settings["nested"].(map[string]interface{})
			Expect(ok).To(BeTrue(), "nested should be a map")
			Expect(nested["key"]).To(Equal("value"))
		})

		It("should be thread-safe with concurrent requests", func() {
			// This test verifies that captured field values don't leak between concurrent requests
			type ConcurrentRequest struct {
				ID          int    `json:"id"`
				Description string `json:"description"`
			}

			type ConcurrentResponse struct {
				ID          int    `json:"id"`
				Description string `json:"description"`
				ProcessedAt string `json:"processed_at"`
			}

			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			change := NewVersionChangeBuilder(v1, v2).
				Description("Concurrent test").
				ForType(ConcurrentRequest{}).
				RequestToNextVersion().
				RemoveField("description").
				ForType(ConcurrentResponse{}).
				ResponseToPreviousVersion().
				AddField("description", "default").
				Build()

			epochInstance, err := setupBasicEpoch([]*Version{v1, v2}, []*VersionChange{change})
			Expect(err).NotTo(HaveOccurred())

			router := setupRouterWithMiddleware(epochInstance)

			router.POST("/concurrent", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)

				// Handler returns the ID it received (without description)
				c.JSON(200, gin.H{
					"id":           req["id"],
					"processed_at": "2024-01-15T10:00:00Z",
				})
			}).Accepts(ConcurrentRequest{}).Returns(ConcurrentResponse{}).ToHandlerFunc("POST", "/concurrent"))

			// Run concurrent requests
			const numRequests = 50
			var wg sync.WaitGroup
			results := make(chan struct {
				requestID   int
				description string
				err         error
			}, numRequests)

			for i := 0; i < numRequests; i++ {
				wg.Add(1)
				go func(id int) {
					defer wg.Done()

					// Each request has a unique description
					description := fmt.Sprintf("Description for request %d", id)
					reqBody := fmt.Sprintf(`{"id": %d, "description": "%s"}`, id, description)

					req := httptest.NewRequest("POST", "/concurrent", strings.NewReader(reqBody))
					req.Header.Set("X-API-Version", "2024-01-01")
					req.Header.Set("Content-Type", "application/json")
					recorder := httptest.NewRecorder()

					router.ServeHTTP(recorder, req)

					if recorder.Code != 200 {
						results <- struct {
							requestID   int
							description string
							err         error
						}{id, "", fmt.Errorf("unexpected status code: %d", recorder.Code)}
						return
					}

					var response map[string]interface{}
					if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
						results <- struct {
							requestID   int
							description string
							err         error
						}{id, "", err}
						return
					}

					results <- struct {
						requestID   int
						description string
						err         error
					}{
						requestID:   int(response["id"].(float64)),
						description: response["description"].(string),
						err:         nil,
					}
				}(i)
			}

			wg.Wait()
			close(results)

			// Verify all requests got their own captured values back
			for result := range results {
				Expect(result.err).NotTo(HaveOccurred(), fmt.Sprintf("Request %d failed", result.requestID))

				expectedDescription := fmt.Sprintf("Description for request %d", result.requestID)
				Expect(result.description).To(Equal(expectedDescription),
					fmt.Sprintf("Request %d got wrong description: expected '%s', got '%s'",
						result.requestID, expectedDescription, result.description))
			}
		})
	})
})
