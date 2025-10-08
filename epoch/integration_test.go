package epoch

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Test models for integration tests
// User model evolution (similar to examples/advanced/main.go):
// 2023-01-01: ID, Name
// 2023-06-01: Added Email
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
// 2023-01-01: ID, Name, Price
// 2023-06-01: No changes
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
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2023-06-01")

			// v1 -> v2: Add email field to User
			change := NewVersionChange(
				"Add email field to User",
				v1,
				v2,
				// Forward migration: v1 request -> v2 (add email)
				&AlterRequestInstruction{
					Schemas: []interface{}{UserV1{}},
					Transformer: func(req *RequestInfo) error {
						if userMap, ok := req.Body.(map[string]interface{}); ok {
							if _, hasEmail := userMap["email"]; !hasEmail {
								userMap["email"] = "unknown@example.com"
							}
						}
						return nil
					},
				},
				// Backward migration: v2 response -> v1 (remove email)
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						if userMap, ok := resp.Body.(map[string]interface{}); ok {
							delete(userMap, "email")
						}
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
			req.Header.Set("X-API-Version", "2023-01-01")
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
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2023-06-01")

			change := NewVersionChange(
				"Add email field to User",
				v1,
				v2,
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						// Handle array of users
						if userList, ok := resp.Body.([]interface{}); ok {
							for _, item := range userList {
								if userMap, ok := item.(map[string]interface{}); ok {
									delete(userMap, "email")
								}
							}
						}
						return nil
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
			req.Header.Set("X-API-Version", "2023-01-01")
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
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2023-06-01")
			v3, _ := NewDateVersion("2024-01-01")

			// Change 1->2: Add email field
			change1 := NewVersionChange(
				"Add email field to User",
				v1, v2,
				&AlterRequestInstruction{
					Schemas: []interface{}{UserV1{}},
					Transformer: func(req *RequestInfo) error {
						if userMap, ok := req.Body.(map[string]interface{}); ok {
							if _, hasEmail := userMap["email"]; !hasEmail {
								userMap["email"] = "unknown@example.com"
							}
						}
						return nil
					},
				},
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						if userMap, ok := resp.Body.(map[string]interface{}); ok {
							delete(userMap, "email")
						}
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
						if userMap, ok := req.Body.(map[string]interface{}); ok {
							// Rename name -> full_name
							if name, hasName := userMap["name"]; hasName {
								userMap["full_name"] = name
								delete(userMap, "name")
							}
							// Add phone if missing
							if _, hasPhone := userMap["phone"]; !hasPhone {
								userMap["phone"] = ""
							}
						}
						return nil
					},
				},
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV3{}},
					Transformer: func(resp *ResponseInfo) error {
						if userMap, ok := resp.Body.(map[string]interface{}); ok {
							// Rename full_name -> name
							if fullName, hasFullName := userMap["full_name"]; hasFullName {
								userMap["name"] = fullName
								delete(userMap, "full_name")
							}
							// Remove phone
							delete(userMap, "phone")
						}
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
			req.Header.Set("X-API-Version", "2023-01-01")
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
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2023-06-01")
			v3, _ := NewDateVersion("2024-01-01")

			// v2 -> v3: Add description and currency to Product
			change := NewVersionChange(
				"Add description and currency fields to Product",
				v2, v3,
				&AlterRequestInstruction{
					Schemas: []interface{}{ProductV1{}},
					Transformer: func(req *RequestInfo) error {
						if productMap, ok := req.Body.(map[string]interface{}); ok {
							if _, hasDesc := productMap["description"]; !hasDesc {
								productMap["description"] = ""
							}
							if _, hasCurrency := productMap["currency"]; !hasCurrency {
								productMap["currency"] = "USD"
							}
						}
						return nil
					},
				},
				&AlterResponseInstruction{
					Schemas: []interface{}{ProductV3{}},
					Transformer: func(resp *ResponseInfo) error {
						transformProduct := func(productMap map[string]interface{}) {
							delete(productMap, "description")
							delete(productMap, "currency")
						}

						if productMap, ok := resp.Body.(map[string]interface{}); ok {
							transformProduct(productMap)
						} else if productList, ok := resp.Body.([]interface{}); ok {
							for _, item := range productList {
								if productMap, ok := item.(map[string]interface{}); ok {
									transformProduct(productMap)
								}
							}
						}
						return nil
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
			req.Header.Set("X-API-Version", "2023-06-01")
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
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2023-06-01")

			change := NewVersionChange(
				"Transform users",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{UserV1{}},
					MigrateHTTPErrors: false, // Don't migrate HTTP errors
					Transformer: func(resp *ResponseInfo) error {
						// This should not be called for error responses
						if userMap, ok := resp.Body.(map[string]interface{}); ok {
							userMap["migrated"] = true
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
			req.Header.Set("X-API-Version", "2023-01-01")
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
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2023-06-01")

			change := NewVersionChange(
				"Transform all responses",
				v1, v2,
				&AlterResponseInstruction{
					Schemas:           []interface{}{UserV1{}},
					MigrateHTTPErrors: true, // Migrate HTTP errors
					Transformer: func(resp *ResponseInfo) error {
						if bodyMap, ok := resp.Body.(map[string]interface{}); ok {
							bodyMap["migrated"] = true
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
			req.Header.Set("X-API-Version", "2023-01-01")
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
			v1, _ := NewDateVersion("2023-01-01")
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

			instance2, err2 := QuickStart("2023-01-01", "2024-01-01")
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
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2024-01-01")

			instance, err := NewEpoch().
				WithVersions(v1, v2).
				WithTypes(UserV1{}).
				WithVersionFormat(VersionFormatDate).
				Build()

			Expect(err).NotTo(HaveOccurred())

			code, err := instance.GenerateStructForVersion(UserV1{}, "2023-01-01")
			Expect(err).NotTo(HaveOccurred())
			Expect(code).To(ContainSubstring("UserV1"))
			Expect(code).To(ContainSubstring("struct"))
		})

		It("should handle pointer types in schema generation", func() {
			v1, _ := NewDateVersion("2023-01-01")

			instance, err := NewEpoch().
				WithVersions(v1).
				WithTypes(&UserV1{}).
				WithVersionFormat(VersionFormatDate).
				Build()

			Expect(err).NotTo(HaveOccurred())

			user := &UserV1{}
			code, err := instance.GenerateStructForVersion(user, "2023-01-01")
			Expect(err).NotTo(HaveOccurred())
			Expect(code).NotTo(BeEmpty())
		})
	})

	Describe("Complete Real-World API Scenario", func() {
		It("should handle full API with multiple endpoints and date-based versions", func() {
			// Setup date-based versions (similar to examples/advanced)
			v1, _ := NewDateVersion("2023-01-01")
			v2, _ := NewDateVersion("2023-06-01")
			v3, _ := NewDateVersion("2024-01-01")

			// v1 -> v2: Add email to users
			userChange1 := NewVersionChange(
				"Add email field to User",
				v1, v2,
				&AlterResponseInstruction{
					Schemas: []interface{}{UserV2{}},
					Transformer: func(resp *ResponseInfo) error {
						if userMap, ok := resp.Body.(map[string]interface{}); ok {
							delete(userMap, "email")
						} else if userList, ok := resp.Body.([]interface{}); ok {
							for _, item := range userList {
								if userMap, ok := item.(map[string]interface{}); ok {
									delete(userMap, "email")
								}
							}
						}
						return nil
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
						transformUser := func(userMap map[string]interface{}) {
							if fullName, hasFullName := userMap["full_name"]; hasFullName {
								userMap["name"] = fullName
								delete(userMap, "full_name")
							}
							delete(userMap, "phone")
						}

						if userMap, ok := resp.Body.(map[string]interface{}); ok {
							transformUser(userMap)
						} else if userList, ok := resp.Body.([]interface{}); ok {
							for _, item := range userList {
								if userMap, ok := item.(map[string]interface{}); ok {
									transformUser(userMap)
								}
							}
						}
						return nil
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
						transformProduct := func(productMap map[string]interface{}) {
							delete(productMap, "currency")
							delete(productMap, "description")
						}

						if productMap, ok := resp.Body.(map[string]interface{}); ok {
							transformProduct(productMap)
						} else if productList, ok := resp.Body.([]interface{}); ok {
							for _, item := range productList {
								if productMap, ok := item.(map[string]interface{}); ok {
									transformProduct(productMap)
								}
							}
						}
						return nil
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

			// Test users with v1 (2023-01-01) - only id and name
			req1 := httptest.NewRequest("GET", "/users", nil)
			req1.Header.Set("X-API-Version", "2023-01-01")
			rec1 := httptest.NewRecorder()
			router.ServeHTTP(rec1, req1)
			Expect(rec1.Code).To(Equal(200))

			var users1 []map[string]interface{}
			json.Unmarshal(rec1.Body.Bytes(), &users1)
			Expect(users1[0]).To(HaveKey("name"))
			Expect(users1[0]).NotTo(HaveKey("email"))
			Expect(users1[0]).NotTo(HaveKey("phone"))
			Expect(users1[0]).NotTo(HaveKey("full_name"))

			// Test users with v2 (2023-06-01) - has name and email, no phone or full_name
			req2 := httptest.NewRequest("GET", "/users", nil)
			req2.Header.Set("X-API-Version", "2023-06-01")
			rec2 := httptest.NewRecorder()
			router.ServeHTTP(rec2, req2)
			Expect(rec2.Code).To(Equal(200))

			var users2 []map[string]interface{}
			json.Unmarshal(rec2.Body.Bytes(), &users2)
			Expect(users2[0]).To(HaveKey("name"))
			Expect(users2[0]).To(HaveKey("email")) // v2 has email
			Expect(users2[0]).NotTo(HaveKey("phone"))
			Expect(users2[0]).NotTo(HaveKey("full_name"))

			// Test products with v2 (2023-06-01) - no currency/description
			req3 := httptest.NewRequest("GET", "/products", nil)
			req3.Header.Set("X-API-Version", "2023-06-01")
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
})
