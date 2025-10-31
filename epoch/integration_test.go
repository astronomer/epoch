package epoch

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"

	"github.com/bytedance/sonic"
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

var _ = Describe("End-to-End Integration Tests", func() {
	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
	})

	Describe("Basic Single-Type Migrations", func() {
		It("should handle simple field addition (V1 to V2 migration)", func() {
			// Setup versions
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// V1→V2: Add email field
			// In V1, User only has: id, full_name, phone
			// In V2, User has: id, full_name, email, phone (HEAD)
			change := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			// Handler works with HEAD version (has email)
			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var user map[string]interface{}
				if err := c.ShouldBindJSON(&user); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				// Verify handler receives V2 format (HEAD) with email
				Expect(user).To(HaveKey("full_name"))
				Expect(user).To(HaveKey("email"))

				// Return HEAD version response
				c.JSON(200, gin.H{
					"id":        1,
					"full_name": user["full_name"],
					"email":     user["email"],
					"phone":     "+1-555-0100",
				})
			}).Accepts(User{}).Returns(User{}).ToHandlerFunc())

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

			// V1 has "name", V2 (HEAD) has "full_name"
			change := NewVersionChangeBuilder(v1, v2).
				Description("Rename name to full_name").
				ForType(User{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var user map[string]interface{}
				c.ShouldBindJSON(&user)

				// Handler expects "full_name" (HEAD version)
				Expect(user).To(HaveKey("full_name"))
				Expect(user).NotTo(HaveKey("name"))

				c.JSON(200, gin.H{
					"id":        1,
					"full_name": user["full_name"],
					"email":     "test@example.com",
					"phone":     "+1-555-0100",
				})
			}).Accepts(User{}).Returns(User{}).ToHandlerFunc())

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

		It("should work with semver versioning format", func() {
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

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.GET("/users/1", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"id":        1,
					"full_name": "Bob",
					"email":     "bob@example.com",
					"phone":     "+1-555-0200",
				})
			}).Returns(User{}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/users/1", nil)
			req.Header.Set("X-API-Version", "1.0.0")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			// V1 should not have email
			Expect(response).NotTo(HaveKey("email"))
		})
	})

	Describe("Array Response Handling", func() {
		It("should handle top-level array responses", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// V1 doesn't have email field
			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			// Handler returns array of users (HEAD version)
			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "full_name": "Alice", "email": "alice@example.com", "phone": "+1-555-0100"},
					{"id": 2, "full_name": "Bob", "email": "bob@example.com", "phone": "+1-555-0200"},
				})
			}).Returns([]User{}).ToHandlerFunc())

			// V1 client request
			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response []map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// All array items should not have email
			Expect(response).To(HaveLen(2))
			Expect(response[0]).NotTo(HaveKey("email"))
			Expect(response[1]).NotTo(HaveKey("email"))
			Expect(response[0]["full_name"]).To(Equal("Alice"))
		})

		It("should handle nested array fields with WithArrayItems", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// V1 doesn't have email field
			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			// Handler returns wrapper with nested array
			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"users": []gin.H{
						{"id": 1, "full_name": "Alice", "email": "alice@example.com", "phone": "+1-555-0100"},
						{"id": 2, "full_name": "Bob", "email": "bob@example.com", "phone": "+1-555-0200"},
					},
					"total": 2,
				})
			}).Returns(UsersListResponse{}).WithArrayItems("users", User{}).ToHandlerFunc())

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

			// Nested array items should not have email in V1
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

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(change1, change2).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.POST("/products", epochInstance.WrapHandler(func(c *gin.Context) {
				var product map[string]interface{}
				c.ShouldBindJSON(&product)

				// Handler expects V3 (HEAD) with currency and description
				Expect(product).To(HaveKey("currency"))
				Expect(product).To(HaveKey("description"))

				c.JSON(200, gin.H{
					"id":          1,
					"name":        product["name"],
					"price":       product["price"],
					"currency":    "USD",
					"description": "High-performance laptop",
				})
			}).Accepts(Product{}).Returns(Product{}).ToHandlerFunc())

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

			// V1 response should have no currency or description
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

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(change1, change2).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.GET("/products/1", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, gin.H{
					"id":          1,
					"name":        "Mouse",
					"price":       29.99,
					"currency":    "USD",
					"description": "Wireless mouse",
				})
			}).Returns(Product{}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/products/1", nil)
			req.Header.Set("X-API-Version", "2024-06-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			// V2 should have currency but NOT description
			Expect(response).To(HaveKey("currency"))
			Expect(response).NotTo(HaveKey("description"))
		})
	})

	Describe("Multiple Types in Single Endpoint", func() {
		It("should handle different request and response types", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Request type migration (CreateUserRequest)
			reqChange := NewVersionChangeBuilder(v1, v2).
				ForType(CreateUserRequest{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				Build()

			// Response type migration (User)
			respChange := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				ResponseToPreviousVersion().
				RemoveField("phone").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(reqChange, respChange).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var req map[string]interface{}
				c.ShouldBindJSON(&req)

				// Request should have email (added by migration)
				Expect(req).To(HaveKey("email"))

				c.JSON(200, gin.H{
					"id":        1,
					"full_name": req["full_name"],
					"email":     req["email"],
					"phone":     "+1-555-0100",
				})
			}).Accepts(CreateUserRequest{}).Returns(User{}).ToHandlerFunc())

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

			// V1 response should not have phone
			Expect(response).NotTo(HaveKey("phone"))
			Expect(response).To(HaveKey("email"))
		})
	})

	Describe("Field Renaming Across Versions", func() {
		It("should transform field names in error messages", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// V1 has "name", V2 (HEAD) has "full_name"
			change := NewVersionChangeBuilder(v1, v2).
				ForType(User{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			// Handler returns error mentioning "full_name"
			router.GET("/users/1", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(400, gin.H{
					"error": "Field 'full_name' is required",
				})
			}).Returns(User{}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/users/1", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(400))

			body := recorder.Body.String()

			// Error message should mention "name" not "full_name" for V1
			Expect(body).To(ContainSubstring("name"))
			Expect(body).NotTo(ContainSubstring("full_name"))
		})
	})

	Describe("Error Response Handling", func() {
		It("should not migrate error responses when MigrateHTTPErrors is false", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Custom change with MigrateHTTPErrors = false
			change := NewVersionChange(
				"Test error handling",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{User{}},
					MigrateHTTPErrors: false,
					Transformer: func(resp *ResponseInfo) error {
						resp.SetField("migrated", true)
						return nil
					},
				},
			)

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.GET("/error", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(400, gin.H{"error": "Bad request"})
			}).Returns(User{}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/error", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(400))

			var response map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &response)

			// Should NOT be migrated
			Expect(response).NotTo(HaveKey("migrated"))
			Expect(response).To(HaveKey("error"))
		})

		It("should migrate error responses when MigrateHTTPErrors is true", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Custom change with MigrateHTTPErrors = true
			change := NewVersionChange(
				"Add version_info field",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{User{}},
					MigrateHTTPErrors: true,
					Transformer: func(resp *ResponseInfo) error {
						// Remove version_info for V1
						resp.DeleteField("version_info")
						return nil
					},
				},
			)

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.GET("/error", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(400, gin.H{
					"error":        "Bad request",
					"version_info": "v2",
				})
			}).Returns(User{}).ToHandlerFunc())

			// Test V2 - should have version_info
			req2 := httptest.NewRequest("GET", "/error", nil)
			req2.Header.Set("X-API-Version", "2024-06-01")
			rec2 := httptest.NewRecorder()

			router.ServeHTTP(rec2, req2)

			Expect(rec2.Code).To(Equal(400))

			var resp2 map[string]interface{}
			json.Unmarshal(rec2.Body.Bytes(), &resp2)
			Expect(resp2).To(HaveKey("version_info"))

			// Test V1 - should NOT have version_info
			req1 := httptest.NewRequest("GET", "/error", nil)
			req1.Header.Set("X-API-Version", "2024-01-01")
			rec1 := httptest.NewRecorder()

			router.ServeHTTP(rec1, req1)

			Expect(rec1.Code).To(Equal(400))

			var resp1 map[string]interface{}
			json.Unmarshal(rec1.Body.Bytes(), &resp1)
			Expect(resp1).NotTo(HaveKey("version_info"))
		})
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

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.GET("/users/:id", epochInstance.WrapHandler(func(c *gin.Context) {
				id := c.Param("id")
				c.JSON(200, gin.H{
					"id":        id,
					"full_name": "Test User",
					"email":     "test@example.com",
					"phone":     "+1-555-0100",
				})
			}).Returns(User{}).ToHandlerFunc())

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

		It("should handle different HTTP methods on same path separately", func() {
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

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(userChange, createChange).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			// GET /users - returns User
			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "full_name": "Alice", "email": "alice@example.com", "phone": "+1-555-0100"},
				})
			}).Returns([]User{}).ToHandlerFunc())

			// POST /users - accepts CreateUserRequest, returns User
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
			}).Accepts(CreateUserRequest{}).Returns(User{}).ToHandlerFunc())

			// Test GET
			getReq := httptest.NewRequest("GET", "/users", nil)
			getReq.Header.Set("X-API-Version", "2024-01-01")
			getRec := httptest.NewRecorder()
			router.ServeHTTP(getRec, getReq)
			Expect(getRec.Code).To(Equal(200))

			// Test POST
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

	Describe("Builder Pattern Integration", func() {
		It("should build complete Epoch instance with all features", func() {
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
		})

		It("should accumulate errors during building", func() {
			_, err := NewEpoch().
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

			instance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(instance.Middleware())

			router.GET("/users", instance.WrapHandler(func(c *gin.Context) {
				// Return JSON with non-alphabetical field order
				c.Data(200, "application/json", []byte(`{"zebra": "first", "alpha": "second", "full_name": "John", "email": "john@example.com", "phone": "555-1234"}`))
			}).Returns(User{}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			responseBody := recorder.Body.String()

			// Verify field order is preserved (zebra comes before alpha)
			zebraPos := strings.Index(responseBody, `"zebra"`)
			alphaPos := strings.Index(responseBody, `"alpha"`)
			namePos := strings.Index(responseBody, `"full_name"`)

			Expect(zebraPos).To(BeNumerically("<", alphaPos))
			Expect(alphaPos).To(BeNumerically("<", namePos))

			// Verify removed fields are not present
			Expect(responseBody).NotTo(ContainSubstring(`"email"`))
			Expect(responseBody).NotTo(ContainSubstring(`"phone"`))
		})

		It("should demonstrate Sonic vs standard JSON order preservation", func() {
			testJSON := `{"zebra": 1, "alpha": 2, "middle": 3}`

			// Test with Sonic (should preserve order)
			sonicNode, err := sonic.Get([]byte(testJSON))
			Expect(err).NotTo(HaveOccurred())
			err = sonicNode.Load()
			Expect(err).NotTo(HaveOccurred())

			sonicResult, err := sonicNode.Raw()
			Expect(err).NotTo(HaveOccurred())

			// Test with standard JSON (will NOT preserve order)
			var stdMap map[string]interface{}
			err = json.Unmarshal([]byte(testJSON), &stdMap)
			Expect(err).NotTo(HaveOccurred())

			stdResult, err := json.Marshal(stdMap)
			Expect(err).NotTo(HaveOccurred())

			sonicStr := string(sonicResult)
			stdStr := string(stdResult)

			// Sonic should preserve original order
			sonicZebraPos := strings.Index(sonicStr, `"zebra"`)
			sonicAlphaPos := strings.Index(sonicStr, `"alpha"`)
			Expect(sonicZebraPos).To(BeNumerically("<", sonicAlphaPos),
				"Sonic should preserve zebra before alpha")

			GinkgoWriter.Printf("Original: %s\n", testJSON)
			GinkgoWriter.Printf("Sonic:    %s\n", sonicStr)
			GinkgoWriter.Printf("Standard: %s\n", stdStr)
		})
	})

	Describe("Cycle Detection Integration", func() {
		It("should prevent circular migrations at build time", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Create circular migration: v1 -> v2 -> v1
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

			// Valid linear chain: v1 -> v2 -> v3
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

			// User migrations
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

			// Product migrations
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

			router := gin.New()
			router.Use(instance.Middleware())

			// User endpoint (returns HEAD version)
			router.GET("/users", instance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "full_name": "Alice Johnson", "email": "alice@example.com", "phone": "+1-555-0100"},
					{"id": 2, "full_name": "Bob Smith", "email": "bob@example.com", "phone": "+1-555-0200"},
				})
			}).Returns([]User{}).ToHandlerFunc())

			// Product endpoint
			router.GET("/products", instance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "name": "Laptop", "price": 999.99, "currency": "USD", "description": "High-performance"},
					{"id": 2, "name": "Mouse", "price": 29.99, "currency": "USD", "description": "Wireless"},
				})
			}).Returns([]Product{}).ToHandlerFunc())

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
})
