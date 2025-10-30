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

// Test models for integration tests
// User model evolution (similar to examples/advanced/main.go):
// 2025-01-01: ID, Name
// 2025-06-01: Added Email
// 2024-01-01: Added Phone, renamed Name -> FullName
type UserV1 struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type UserV2 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserV3 struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
}

// Product model evolution:
// 2025-01-01: ID, Name, Price
// 2025-06-01: No changes
// 2024-01-01: Added Description, Currency
type ProductV1 struct {
	ID    int     `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

type ProductV3 struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Currency    string  `json:"currency"`
	Description string  `json:"description,omitempty"`
}

var _ = Describe("End-to-End Integration Tests", func() {
	BeforeEach(func() {
		gin.SetMode(gin.TestMode)
	})

	Describe("Complete API Versioning Flow", func() {
		It("should handle v1 to v2 migration (add email field)", func() {
			// Setup date-based versions (similar to examples/advanced)
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")

			// v1 -> v2: Add email field to User using type-based API
			change := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			// Build Cadwyn instance
			cadwynInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithTypes(UserV1{}, UserV2{}).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			// Setup Gin router
			router := gin.New()
			router.Use(cadwynInstance.Middleware())

			// Handler that works with v2 (latest) format
			router.POST("/users", cadwynInstance.WrapHandler(func(c *gin.Context) {
				var user map[string]interface{}
				if err := c.ShouldBindJSON(&user); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				// Verify we received v2 format with email
				Expect(user).To(HaveKey("name"))
				Expect(user).To(HaveKey("email"))

				// Return v2 format response
				c.JSON(200, gin.H{
					"id":    1,
					"name":  user["name"],
					"email": user["email"],
				})
			}).ToHandlerFunc())

			// Test with v1 client (no email)
			reqBody := map[string]interface{}{
				"name": "John Doe",
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/users", bytes.NewReader(bodyBytes))
			req.Header.Set("X-API-Version", "2025-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Response should be converted back to v1 format (no email)
			Expect(response).To(HaveKey("name"))
			Expect(response).To(HaveKey("id"))
			Expect(response).NotTo(HaveKey("email"))
			Expect(response["name"]).To(Equal("John Doe"))
		})

		It("should handle array responses with migrations", func() {
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")

			// Use type-based API for array migrations
			change := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			cadwynInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithTypes(UserV1{}, UserV2{}).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(cadwynInstance.Middleware())

			router.GET("/users", cadwynInstance.WrapHandler(func(c *gin.Context) {
				// Return array of v2 users with email
				c.JSON(200, []gin.H{
					{"id": 1, "name": "Alice", "email": "alice@example.com"},
					{"id": 2, "name": "Bob", "email": "bob@example.com"},
				})
			}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2025-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response []map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// All users should not have email field
			Expect(response).To(HaveLen(2))
			Expect(response[0]).NotTo(HaveKey("email"))
			Expect(response[1]).NotTo(HaveKey("email"))
			Expect(response[0]["name"]).To(Equal("Alice"))
			Expect(response[1]["name"]).To(Equal("Bob"))
		})
	})

	Describe("Multi-Version Chain Migration", func() {
		It("should migrate through three versions with field renaming", func() {
			// Setup three date-based versions
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")
			v3, _ := NewDateVersion("2024-01-01")

			// Change 1->2: Add email field using type-based API
			change1 := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			// Change 2->3: Add phone, rename name -> full_name using type-based API
			change2 := NewVersionChangeBuilder(v2, v3).
				Description("Add phone field and rename name to full_name").
				ForType(UserV3{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				AddField("phone", "").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				RemoveField("phone").
				Build()

			// Build Cadwyn instance
			cadwynInstance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(change1, change2).
				WithTypes(UserV1{}, UserV2{}, UserV3{}).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			// Setup router
			router := gin.New()
			router.Use(cadwynInstance.Middleware())

			router.POST("/users", cadwynInstance.WrapHandler(func(c *gin.Context) {
				var user map[string]interface{}
				if err := c.ShouldBindJSON(&user); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				// Handler expects v3 format (latest) with full_name, email, phone
				Expect(user).To(HaveKey("full_name"))
				Expect(user).To(HaveKey("email"))
				Expect(user).To(HaveKey("phone"))
				Expect(user).NotTo(HaveKey("name"))

				c.JSON(200, gin.H{
					"id":        1,
					"full_name": user["full_name"],
					"email":     user["email"],
					"phone":     user["phone"],
				})
			}).ToHandlerFunc())

			// Test with v1 client (only has name field)
			reqBody := map[string]interface{}{
				"name": "Alice Johnson",
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/users", bytes.NewReader(bodyBytes))
			req.Header.Set("X-API-Version", "2025-01-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Response should be v1 format (only id and name)
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("name"))
			Expect(response).NotTo(HaveKey("full_name"))
			Expect(response).NotTo(HaveKey("email"))
			Expect(response).NotTo(HaveKey("phone"))
			Expect(response["name"]).To(Equal("Alice Johnson"))
		})

		It("should handle Product migrations with currency and description", func() {
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")
			v3, _ := NewDateVersion("2024-01-01")

			// v2 -> v3: Add description and currency to Product using type-based API
			change := NewVersionChangeBuilder(v2, v3).
				Description("Add description and currency fields to Product").
				ForType(ProductV3{}).
				RequestToNextVersion().
				AddField("description", "").
				AddField("currency", "USD").
				ResponseToPreviousVersion().
				RemoveField("description").
				RemoveField("currency").
				Build()

			cadwynInstance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(change).
				WithTypes(ProductV1{}, ProductV3{}).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(cadwynInstance.Middleware())

			router.POST("/products", cadwynInstance.WrapHandler(func(c *gin.Context) {
				var product map[string]interface{}
				if err := c.ShouldBindJSON(&product); err != nil {
					c.JSON(400, gin.H{"error": err.Error()})
					return
				}

				// Handler expects v3 format (latest)
				Expect(product).To(HaveKey("currency"))
				Expect(product).To(HaveKey("description"))

				c.JSON(200, gin.H{
					"id":          1,
					"name":        product["name"],
					"price":       product["price"],
					"currency":    product["currency"],
					"description": "High-performance laptop",
				})
			}).ToHandlerFunc())

			// Test with v2 client (no currency/description)
			reqBody := map[string]interface{}{
				"name":  "Laptop",
				"price": 999.99,
			}
			bodyBytes, _ := json.Marshal(reqBody)

			req := httptest.NewRequest("POST", "/products", bytes.NewReader(bodyBytes))
			req.Header.Set("X-API-Version", "2025-06-01")
			req.Header.Set("Content-Type", "application/json")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Response should be v2 format (no currency, no description)
			Expect(response).To(HaveKey("id"))
			Expect(response).To(HaveKey("name"))
			Expect(response).To(HaveKey("price"))
			Expect(response).NotTo(HaveKey("currency"))
			Expect(response).NotTo(HaveKey("description"))
		})
	})

	Describe("Error Response Handling", func() {
		It("should not migrate error responses when MigrateHTTPErrors is false", func() {
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")

			// NOTE: Schema-based API uses MigrateHTTPErrors=true by default
			// This test uses custom transformer to test the MigrateHTTPErrors=false case
			change := NewVersionChange(
				"Transform users",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{}, // Global (no specific schema)
					MigrateHTTPErrors: false,           // Don't migrate HTTP errors
					Transformer: func(resp *ResponseInfo) error {
						// This should not be called for error responses
						if resp.Body != nil {
							resp.SetField("migrated", true)
						}
						return nil
					},
				},
			)

			cadwynInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(cadwynInstance.Middleware())

			router.GET("/error", cadwynInstance.WrapHandler(func(c *gin.Context) {
				c.JSON(400, gin.H{"error": "Bad request"})
			}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/error", nil)
			req.Header.Set("X-API-Version", "2025-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(400))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Should not have been migrated
			Expect(response).NotTo(HaveKey("migrated"))
			Expect(response).To(HaveKey("error"))
		})

		It("should migrate error responses when MigrateHTTPErrors is true", func() {
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")

			// Test that type-based API works with error responses (MigrateHTTPErrors=true by default)
			change := NewVersionChange(
				"Transform error responses",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{}, // Global (applies to all responses)
					MigrateHTTPErrors: true,
					Transformer: func(resp *ResponseInfo) error {
						// Remove version_info field for v1 clients
						if resp.Body != nil && resp.HasField("version_info") {
							resp.DeleteField("version_info")
						}
						return nil
					},
				},
			)

			cadwynInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(cadwynInstance.Middleware())

			router.GET("/error", cadwynInstance.WrapHandler(func(c *gin.Context) {
				// Handler returns HEAD/v2 format with version_info field
				c.JSON(400, gin.H{"error": "Bad request", "version_info": "v2"})
			}).ToHandlerFunc())

			// Test with v2 - should have version_info field (no migration needed)
			req2 := httptest.NewRequest("GET", "/error", nil)
			req2.Header.Set("X-API-Version", "2025-06-01")
			recorder2 := httptest.NewRecorder()

			router.ServeHTTP(recorder2, req2)

			Expect(recorder2.Code).To(Equal(400))

			var response2 map[string]interface{}
			err = json.Unmarshal(recorder2.Body.Bytes(), &response2)
			Expect(err).NotTo(HaveOccurred())

			// Should have version_info field (no migration needed)
			Expect(response2).To(HaveKey("version_info"))
			Expect(response2["version_info"]).To(Equal("v2"))
			Expect(response2).To(HaveKey("error"))

			// Test with v1 - should NOT have version_info field (removed by backward migration)
			req1 := httptest.NewRequest("GET", "/error", nil)
			req1.Header.Set("X-API-Version", "2025-01-01")
			recorder1 := httptest.NewRecorder()

			router.ServeHTTP(recorder1, req1)

			Expect(recorder1.Code).To(Equal(400))

			var response1 map[string]interface{}
			err = json.Unmarshal(recorder1.Body.Bytes(), &response1)
			Expect(err).NotTo(HaveOccurred())

			// Should NOT have version_info field (removed by backward migration)
			Expect(response1).NotTo(HaveKey("version_info"))
			Expect(response1).To(HaveKey("error"))
		})
	})

	Describe("Builder Pattern Integration", func() {
		It("should build complete Cadwyn instance with all features using date versioning", func() {
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2024-01-01")

			change := NewVersionChange("Test change", v1, v2)

			instance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithTypes(UserV1{}, UserV2{}).
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

			instance2, err2 := QuickStart("2025-01-01", "2024-01-01")
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

	Describe("Complete Real-World API Scenario", func() {
		It("should handle full API with multiple endpoints and date-based versions", func() {
			// Setup date-based versions (similar to examples/advanced)
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")
			v3, _ := NewDateVersion("2024-01-01")

			// v1 -> v2: Add email to users using type-based API
			userChange1 := NewVersionChangeBuilder(v1, v2).
				Description("Add email field to User").
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "unknown@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			// v2 -> v3: Add phone, rename name -> full_name for users using type-based API
			userChange2 := NewVersionChangeBuilder(v2, v3).
				Description("Add phone and rename name to full_name").
				ForType(UserV3{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				AddField("phone", "").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				RemoveField("phone").
				Build()

			// v2 -> v3: Add currency and description to products using type-based API
			productChange := NewVersionChangeBuilder(v2, v3).
				Description("Add currency and description to Product").
				ForType(ProductV3{}).
				RequestToNextVersion().
				AddField("currency", "USD").
				AddField("description", "").
				ResponseToPreviousVersion().
				RemoveField("currency").
				RemoveField("description").
				Build()

			instance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithHeadVersion().
				WithChanges(userChange1, userChange2, productChange).
				WithTypes(UserV1{}, UserV2{}, UserV3{}, ProductV1{}, ProductV3{}).
				WithVersionFormat(VersionFormatDate).
				WithDefaultVersion(v2).
				Build()

			Expect(err).NotTo(HaveOccurred())

			// Setup router with multiple endpoints
			router := gin.New()
			router.Use(instance.Middleware())

			// User endpoints (return HEAD version format)
			router.GET("/users", instance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "full_name": "Alice Johnson", "email": "alice@example.com", "phone": "+1-555-0100"},
					{"id": 2, "full_name": "Bob Smith", "email": "bob@example.com", "phone": "+1-555-0200"},
				})
			}).ToHandlerFunc())

			router.GET("/products", instance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "name": "Laptop", "price": 999.99, "currency": "USD", "description": "High-performance"},
					{"id": 2, "name": "Mouse", "price": 29.99, "currency": "USD", "description": "Wireless"},
				})
			}).ToHandlerFunc())

			// Test users with v1 (2025-01-01) - only id and name
			req1 := httptest.NewRequest("GET", "/users", nil)
			req1.Header.Set("X-API-Version", "2025-01-01")
			rec1 := httptest.NewRecorder()
			router.ServeHTTP(rec1, req1)
			Expect(rec1.Code).To(Equal(200))

			var users1 []map[string]interface{}
			json.Unmarshal(rec1.Body.Bytes(), &users1)
			Expect(users1[0]).To(HaveKey("name"))
			Expect(users1[0]).NotTo(HaveKey("email"))
			Expect(users1[0]).NotTo(HaveKey("phone"))
			Expect(users1[0]).NotTo(HaveKey("full_name"))

			// Test users with v2 (2025-06-01) - has name and email, no phone or full_name
			req2 := httptest.NewRequest("GET", "/users", nil)
			req2.Header.Set("X-API-Version", "2025-06-01")
			rec2 := httptest.NewRecorder()
			router.ServeHTTP(rec2, req2)
			Expect(rec2.Code).To(Equal(200))

			var users2 []map[string]interface{}
			json.Unmarshal(rec2.Body.Bytes(), &users2)
			Expect(users2[0]).To(HaveKey("name"))
			Expect(users2[0]).To(HaveKey("email")) // v2 has email
			Expect(users2[0]).NotTo(HaveKey("phone"))
			Expect(users2[0]).NotTo(HaveKey("full_name"))

			// Test products with v2 (2025-06-01) - no currency/description
			req3 := httptest.NewRequest("GET", "/products", nil)
			req3.Header.Set("X-API-Version", "2025-06-01")
			rec3 := httptest.NewRecorder()
			router.ServeHTTP(rec3, req3)
			Expect(rec3.Code).To(Equal(200))

			var products2 []map[string]interface{}
			json.Unmarshal(rec3.Body.Bytes(), &products2)
			Expect(products2[0]).To(HaveKey("name"))
			Expect(products2[0]).To(HaveKey("price"))
			Expect(products2[0]).NotTo(HaveKey("currency"))
			Expect(products2[0]).NotTo(HaveKey("description"))

			// Test with HEAD version - all fields present
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

	Describe("JSON Field Order Preservation", func() {
		It("should preserve field order in API responses", func() {
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2024-01-01")

			// Migration that adds fields - they should appear at the end
			change := NewVersionChange(
				"Add fields to response",
				v1, v2,
				&AlterResponseInstruction{
					Schemas: []interface{}{}, // Global (applies to all responses)
					Transformer: func(resp *ResponseInfo) error {
						// Remove fields for v1 (going backwards)
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

			// Handler returns v2 format with specific field order
			router.GET("/users", instance.WrapHandler(func(c *gin.Context) {
				// Return JSON with non-alphabetical field order
				c.Data(200, "application/json", []byte(`{"zebra": "first", "alpha": "second", "name": "John", "email": "john@example.com", "phone": "555-1234"}`))
			}).ToHandlerFunc())

			// Test with v1 - should preserve original order of remaining fields
			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2025-01-01")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			responseBody := recorder.Body.String()

			// Verify field order is preserved (zebra comes before alpha)
			zebraPos := strings.Index(responseBody, `"zebra"`)
			alphaPos := strings.Index(responseBody, `"alpha"`)
			namePos := strings.Index(responseBody, `"name"`)

			Expect(zebraPos).To(BeNumerically("<", alphaPos))
			Expect(alphaPos).To(BeNumerically("<", namePos))

			// Verify removed fields are not present
			Expect(responseBody).NotTo(ContainSubstring(`"email"`))
			Expect(responseBody).NotTo(ContainSubstring(`"phone"`))
		})

		It("should demonstrate Sonic vs standard JSON order preservation", func() {
			// Test data with intentionally non-alphabetical order
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

			// This demonstrates why we use Sonic for order preservation
			GinkgoWriter.Printf("Original: %s\n", testJSON)
			GinkgoWriter.Printf("Sonic:    %s\n", sonicStr)
			GinkgoWriter.Printf("Standard: %s\n", stdStr)
		})

		It("should preserve order in nested objects and arrays", func() {
			v1, _ := NewDateVersion("2025-01-01")

			instance, err := NewEpoch().
				WithVersions(v1).
				WithVersionFormat(VersionFormatDate).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(instance.Middleware())

			router.GET("/complex", instance.WrapHandler(func(c *gin.Context) {
				// Complex nested structure with non-alphabetical order
				complexJSON := `{
					"users": [
						{
							"z_last_name": "Smith",
							"a_first_name": "John",
							"profile": {
								"zebra_field": "first",
								"alpha_field": "second"
							}
						}
					],
					"zebra_metadata": {"version": "1.0"}
				}`
				c.Data(200, "application/json", []byte(complexJSON))
			}).ToHandlerFunc())

			req := httptest.NewRequest("GET", "/complex", nil)
			req.Header.Set("X-API-Version", "2025-01-01")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(200))
			responseBody := recorder.Body.String()

			// Verify nested object field order is preserved
			zLastNamePos := strings.Index(responseBody, `"z_last_name"`)
			aFirstNamePos := strings.Index(responseBody, `"a_first_name"`)
			Expect(zLastNamePos).To(BeNumerically("<", aFirstNamePos))

			// Verify deeply nested field order is preserved
			zebraFieldPos := strings.Index(responseBody, `"zebra_field"`)
			alphaFieldPos := strings.Index(responseBody, `"alpha_field"`)
			Expect(zebraFieldPos).To(BeNumerically("<", alphaFieldPos))
		})
	})

	Describe("Cycle Detection Integration", func() {
		It("should prevent circular migrations at build time", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Create circular migration: v1 -> v2 -> v1
			change1 := NewVersionChangeBuilder(v1, v2).
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "test@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			change2 := NewVersionChangeBuilder(v2, v1).
				ForType(UserV1{}).
				RequestToNextVersion().
				RemoveField("email").
				ResponseToPreviousVersion().
				AddField("email", "test@example.com").
				Build()

			// This should fail with cycle detection error
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
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "test@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			change2 := NewVersionChangeBuilder(v2, v3).
				ForType(UserV3{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			// Should succeed
			epochInstance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(change1, change2).
				WithTypes(UserV1{}, UserV2{}, UserV3{}).
				Build()

			Expect(err).NotTo(HaveOccurred())
			Expect(epochInstance).NotTo(BeNil())
		})
	})

	Describe("Declarative API Integration", func() {
		It("should handle all operation types in one migration", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Migration with operation types using type-based API
			change := NewVersionChangeBuilder(v1, v2).
				Description("Test type-based operations").
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "default@example.com"). // AddField
				RemoveField("temp_field").                // RemoveField
				RenameField("name", "full_name").         // RenameField
				ResponseToPreviousVersion().
				RemoveField("email").
				AddField("temp_field", "").
				RenameField("full_name", "name").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithTypes(UserV1{}, UserV2{}).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.POST("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				var body map[string]interface{}
				c.ShouldBindJSON(&body)
				c.JSON(200, body)
			}).ToHandlerFunc())

			// Test request migration (v1 -> v2)
			// Send v1 request, it gets migrated to v2, handler echoes it, then migrated back to v1
			reqBody := `{"name":"John","temp_field":"old","status":"active"}`
			req := httptest.NewRequest("POST", "/users", bytes.NewBufferString(reqBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			var resp map[string]interface{}
			json.Unmarshal(recorder.Body.Bytes(), &resp)

			// Verify transformations (response is migrated back to v1)
			Expect(resp["name"]).To(Equal("John")) // RenameField applied (full_name -> name on response)
			_, hasEmail := resp["email"]
			Expect(hasEmail).To(BeFalse()) // AddField removed on response migration
			_, hasTempField := resp["temp_field"]
			Expect(hasTempField).To(BeFalse()) // RemoveField applied on request
		})

		It("should transform error messages with field renames", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")

			// Test error message field name transformation with type-based API
			change := NewVersionChangeBuilder(v1, v2).
				Description("Test error message field name transformation").
				ForType(UserV2{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2).
				WithChanges(change).
				WithTypes(UserV1{}, UserV2{}).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.GET("/users", epochInstance.WrapHandler(func(c *gin.Context) {
				// Simulate an error mentioning "full_name"
				c.JSON(400, gin.H{"error": "Field 'full_name' is required"})
			}).ToHandlerFunc())

			// Request with v1 - error should mention "name" not "full_name"
			req := httptest.NewRequest("GET", "/users", nil)
			req.Header.Set("X-API-Version", "2024-01-01")
			recorder := httptest.NewRecorder()
			router.ServeHTTP(recorder, req)

			body := recorder.Body.String()
			Expect(body).To(ContainSubstring("name"))
			Expect(body).NotTo(ContainSubstring("full_name"))
		})
	})

	Describe("Complex Chained Migrations Integration", func() {
		It("should handle migrations across multiple versions", func() {
			v1, _ := NewDateVersion("2024-01-01")
			v2, _ := NewDateVersion("2024-06-01")
			v3, _ := NewDateVersion("2025-01-01")

			// v1 -> v2: Add email using type-based API
			change1 := NewVersionChangeBuilder(v1, v2).
				Description("Add email field").
				ForType(UserV2{}).
				RequestToNextVersion().
				AddField("email", "default@example.com").
				ResponseToPreviousVersion().
				RemoveField("email").
				Build()

			// v2 -> v3: Rename name -> full_name using type-based API
			change2 := NewVersionChangeBuilder(v2, v3).
				Description("Rename name to full_name").
				ForType(UserV3{}).
				RequestToNextVersion().
				RenameField("name", "full_name").
				ResponseToPreviousVersion().
				RenameField("full_name", "name").
				Build()

			epochInstance, err := NewEpoch().
				WithVersions(v1, v2, v3).
				WithChanges(change1, change2).
				WithTypes(UserV1{}, UserV2{}, UserV3{}).
				Build()
			Expect(err).NotTo(HaveOccurred())

			router := gin.New()
			router.Use(epochInstance.Middleware())

			router.GET("/users/:id", epochInstance.WrapHandler(func(c *gin.Context) {
				// HEAD version (v3) response
				c.JSON(200, gin.H{
					"id":        1,
					"full_name": "John Doe",
					"email":     "john@example.com",
				})
			}).ToHandlerFunc())

			// Test v1 (should apply BOTH response migrations in reverse)
			req1 := httptest.NewRequest("GET", "/users/1", nil)
			req1.Header.Set("X-API-Version", "2024-01-01")
			recorder1 := httptest.NewRecorder()
			router.ServeHTTP(recorder1, req1)

			var v1Resp map[string]interface{}
			json.Unmarshal(recorder1.Body.Bytes(), &v1Resp)
			Expect(v1Resp["id"]).To(Equal(float64(1)))
			Expect(v1Resp["name"]).To(Equal("John Doe")) // full_name -> name (chained)
			_, hasEmail := v1Resp["email"]
			Expect(hasEmail).To(BeFalse()) // email removed
			_, hasFullName := v1Resp["full_name"]
			Expect(hasFullName).To(BeFalse()) // Doesn't exist in v1

			// Test v2 (should only apply one response migration: full_name -> name)
			req2 := httptest.NewRequest("GET", "/users/1", nil)
			req2.Header.Set("X-API-Version", "2024-06-01")
			recorder2 := httptest.NewRecorder()
			router.ServeHTTP(recorder2, req2)

			var v2Resp map[string]interface{}
			json.Unmarshal(recorder2.Body.Bytes(), &v2Resp)
			Expect(v2Resp["id"]).To(Equal(float64(1)))
			Expect(v2Resp["name"]).To(Equal("John Doe"))          // full_name -> name
			Expect(v2Resp["email"]).To(Equal("john@example.com")) // email kept
			_, hasFullName2 := v2Resp["full_name"]
			Expect(hasFullName2).To(BeFalse()) // Doesn't exist in v2
		})
	})
})
