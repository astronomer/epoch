package epoch

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
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

			// v1 -> v2: Add email field to User
			change := NewVersionChange(
				"Add email field to User",
				v1,
				v2,
				// Forward migration: v1 request -> v2 (add email)
				&AlterRequestInstruction{
					Schemas: []interface{}{UserV1{}},
					Transformer: func(req *RequestInfo) error {
						if !req.HasField("email") {
							req.SetField("email", "unknown@example.com")
						}
						return nil
					},
				},
				// Backward migration: v2 response -> v1 (remove email)
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						resp.DeleteField("email")
						return nil
					},
				},
			)

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
			}))

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

			change := NewVersionChange(
				"Add email field to User",
				v1,
				v2,
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						// Handle single user (if body is an object)
						if resp.Body != nil && resp.Body.TypeSafe() == ast.V_OBJECT {
							resp.DeleteField("email")
						}
						// Handle array of users
						return resp.TransformArrayField("", func(userNode *ast.Node) error {
							return DeleteNodeField(userNode, "email")
						})
					},
				},
			)

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
			}))

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

			// Change 1->2: Add email field
			change1 := NewVersionChange(
				"Add email field to User",
				v1, v2,
				&AlterRequestInstruction{
					Schemas: []interface{}{UserV1{}},
					Transformer: func(req *RequestInfo) error {
						if !req.HasField("email") {
							req.SetField("email", "unknown@example.com")
						}
						return nil
					},
				},
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						resp.DeleteField("email")
						return nil
					},
				},
			)

			// Change 2->3: Add phone, rename name -> full_name
			change2 := NewVersionChange(
				"Add phone field and rename name to full_name",
				v2, v3,
				&AlterRequestInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(req *RequestInfo) error {
						// Rename name -> full_name
						if req.HasField("name") {
							nameNode := req.GetField("name")
							if nameNode != nil {
								nameStr, _ := nameNode.String()
								req.SetField("full_name", nameStr)
								req.DeleteField("name")
							}
						}
						// Add phone if missing
						if !req.HasField("phone") {
							req.SetField("phone", "")
						}
						return nil
					},
				},
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV3{}},
					Transformer: func(resp *ResponseInfo) error {
						// Rename full_name -> name
						if fullNameNode := resp.GetField("full_name"); fullNameNode != nil && fullNameNode.Exists() {
							fullNameStr, _ := fullNameNode.String()
							resp.SetField("name", fullNameStr)
							resp.DeleteField("full_name")
						}
						// Remove phone
						resp.DeleteField("phone")
						return nil
					},
				},
			)

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
			}))

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

			// v2 -> v3: Add description and currency to Product
			change := NewVersionChange(
				"Add description and currency fields to Product",
				v2, v3,
				&AlterRequestInstruction{
					Schemas: []interface{}{ProductV1{}},
					Transformer: func(req *RequestInfo) error {
						if !req.HasField("description") {
							req.SetField("description", "")
						}
						if !req.HasField("currency") {
							req.SetField("currency", "USD")
						}
						return nil
					},
				},
				&AlterResponseInstruction{
					Schemas: []interface{}{ProductV3{}},
					Transformer: func(resp *ResponseInfo) error {
						transformProduct := func(productNode *ast.Node) error {
							productNode.Unset("description")
							productNode.Unset("currency")
							return nil
						}

						// Handle single product
						if resp.Body != nil {
							transformProduct(resp.Body)
						}

						// Handle array of products
						return resp.TransformArrayField("", transformProduct)
					},
				},
			)

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
			}))

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

			change := NewVersionChange(
				"Transform users",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{UserV1{}},
					MigrateHTTPErrors: false, // Don't migrate HTTP errors
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
			}))

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

			change := NewVersionChange(
				"Transform all responses",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{UserV1{}},
					MigrateHTTPErrors: true, // Migrate HTTP errors
					Transformer: func(resp *ResponseInfo) error {
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
			}))

			req := httptest.NewRequest("GET", "/error", nil)
			req.Header.Set("X-API-Version", "2025-01-01")
			recorder := httptest.NewRecorder()

			router.ServeHTTP(recorder, req)

			Expect(recorder.Code).To(Equal(400))

			var response map[string]interface{}
			err = json.Unmarshal(recorder.Body.Bytes(), &response)
			Expect(err).NotTo(HaveOccurred())

			// Should have been migrated
			Expect(response).To(HaveKey("migrated"))
			Expect(response["migrated"]).To(BeTrue())
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
			Expect(instance.GetSchemaGenerator()).NotTo(BeNil())
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

	Describe("Schema Generation Integration", func() {
		It("should generate struct code for specific version", func() {
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2024-01-01")

			instance, err := NewEpoch().
				WithVersions(v1, v2).
				WithTypes(UserV1{}).
				WithVersionFormat(VersionFormatDate).
				Build()

			Expect(err).NotTo(HaveOccurred())

			code, err := instance.GenerateStructForVersion(UserV1{}, "2025-01-01")
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(ContainSubstring("UserV1"))
			Expect(code).To(ContainSubstring("struct"))
		})

		It("should handle pointer types in schema generation", func() {
			v1, _ := NewDateVersion("2025-01-01")

			instance, err := NewEpoch().
				WithVersions(v1).
				WithTypes(&UserV1{}).
				WithVersionFormat(VersionFormatDate).
				Build()

			Expect(err).NotTo(HaveOccurred())

			user := &UserV1{}
			code, err := instance.GenerateStructForVersion(user, "2025-01-01")
			Expect(err).NotTo(HaveOccurred())
			Expect(code).NotTo(BeEmpty())
		})
	})

	Describe("Complete Real-World API Scenario", func() {
		It("should handle full API with multiple endpoints and date-based versions", func() {
			// Setup date-based versions (similar to examples/advanced)
			v1, _ := NewDateVersion("2025-01-01")
			v2, _ := NewDateVersion("2025-06-01")
			v3, _ := NewDateVersion("2024-01-01")

			// v1 -> v2: Add email to users
			userChange1 := NewVersionChange(
				"Add email field to User",
				v1, v2,
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						// Handle single user (if body is an object)
						if resp.Body != nil && resp.Body.Type() == ast.V_OBJECT {
							resp.DeleteField("email")
						}
						// Handle array of users
						return resp.TransformArrayField("", func(userNode *ast.Node) error {
							return DeleteNodeField(userNode, "email")
						})
					},
				},
			)

			// v2 -> v3: Add phone, rename name -> full_name for users
			userChange2 := NewVersionChange(
				"Add phone and rename name to full_name",
				v2, v3,
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV3{}},
					Transformer: func(resp *ResponseInfo) error {
						transformUser := func(userNode *ast.Node) error {
							// Rename full_name -> name using helper function
							if err := RenameNodeField(userNode, "full_name", "name"); err != nil {
								return err
							}
							// Remove phone field using helper function
							return DeleteNodeField(userNode, "phone")
						}

						// Handle single user (if body is an object)
						if resp.Body != nil && resp.Body.Type() == ast.V_OBJECT {
							transformUser(resp.Body)
						}

						// Handle array of users
						return resp.TransformArrayField("", transformUser)
					},
				},
			)

			// v2 -> v3: Add currency and description to products
			productChange := NewVersionChange(
				"Add currency and description to Product",
				v2, v3,
				&AlterResponseInstruction{
					Schemas: []interface{}{ProductV3{}},
					Transformer: func(resp *ResponseInfo) error {
						transformProduct := func(productNode *ast.Node) error {
							productNode.Unset("currency")
							productNode.Unset("description")
							return nil
						}

						// Handle single product
						if resp.Body != nil {
							transformProduct(resp.Body)
						}

						// Handle array of products
						return resp.TransformArrayField("", transformProduct)
					},
				},
			)

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
			}))

			router.GET("/products", instance.WrapHandler(func(c *gin.Context) {
				c.JSON(200, []gin.H{
					{"id": 1, "name": "Laptop", "price": 999.99, "currency": "USD", "description": "High-performance"},
					{"id": 2, "name": "Mouse", "price": 29.99, "currency": "USD", "description": "Wireless"},
				})
			}))

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
					Schemas: []interface{}{UserV1{}},
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
			router.GET("/user", instance.WrapHandler(func(c *gin.Context) {
				// Return JSON with non-alphabetical field order
				c.Data(200, "application/json", []byte(`{"zebra": "first", "alpha": "second", "name": "John", "email": "john@example.com", "phone": "555-1234"}`))
			}))

			// Test with v1 - should preserve original order of remaining fields
			req := httptest.NewRequest("GET", "/user", nil)
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
			}))

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
})
