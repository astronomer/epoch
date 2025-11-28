package epoch

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test models - HEAD version only (what controllers actually work with)
// Migrations describe how older versions differ from HEAD

// User - HEAD version has all fields
type User struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"` // V2 renamed from "name"
	Email    string `json:"email"`     // Added in V2
	Phone    string `json:"phone"`     // Added in V3
}

// Product - HEAD version
type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`    // Added in V2
	Description string  `json:"description"` // Added in V3
}

// CreateUserRequest - HEAD version
type CreateUserRequest struct {
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// UsersListResponse - wrapper with nested array
type UsersListResponse struct {
	Users []User `json:"users"`
	Total int    `json:"total"`
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

		It("should handle nested array fields with WithArrayItems", func() {
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
			}).Returns(UsersListResponse{}).WithArrayItems("users", User{}).ToHandlerFunc("GET", "/users"))

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
				WithArrayItems("users", User{}).
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
})
