package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/astronomer/cadwyn-go/cadwyn"
	"github.com/gin-gonic/gin"
)

// User model evolution:
// 2023-01-01: ID, Name
// 2023-06-01: Added Email
// 2024-01-01: Added Phone, renamed Name -> FullName
type User struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"` // Was "name" before 2024-01-01
	Email    string `json:"email"`     // Added in 2023-06-01
	Phone    string `json:"phone"`     // Added in 2024-01-01
}

// Product model evolution:
// 2023-01-01: ID, Name, Price
// 2023-06-01: No changes
// 2024-01-01: Added Description, Currency
type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"` // Added in 2024-01-01
	Currency    string  `json:"currency"`    // Added in 2024-01-01
}

// In-memory storage (for demo purposes)
var (
	users = map[int]User{
		1: {ID: 1, FullName: "Alice Johnson", Email: "alice@example.com", Phone: "+1-555-0100"},
		2: {ID: 2, FullName: "Bob Smith", Email: "bob@example.com", Phone: "+1-555-0200"},
	}
	products = map[int]Product{
		1: {ID: 1, Name: "Laptop", Price: 999.99, Description: "High-performance laptop", Currency: "USD"},
		2: {ID: 2, Name: "Mouse", Price: 29.99, Description: "Wireless mouse", Currency: "USD"},
	}
	nextUserID    = 3
	nextProductID = 3
)

func main() {
	// Create date-based versions
	v1, _ := cadwyn.NewDateVersion("2023-01-01")
	v2, _ := cadwyn.NewDateVersion("2023-06-01")
	v3, _ := cadwyn.NewDateVersion("2024-01-01")

	// Build Cadwyn instance with complex migrations
	cadwynInstance, err := cadwyn.NewCadwyn().
		WithVersions(v1, v2, v3).
		WithHeadVersion().
		WithChanges(
			createUserV1ToV2Migration(v1, v2),
			createUserV2ToV3Migration(v2, v3),
			createProductV2ToV3Migration(v2, v3),
		).
		WithTypes(User{}, Product{}).
		WithVersionLocation(cadwyn.VersionLocationHeader).
		WithVersionParameter("X-API-Version").
		WithVersionFormat(cadwyn.VersionFormatDate).
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to create Cadwyn instance: %v", err))
	}

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Middleware
	r.Use(cadwynInstance.Middleware())
	r.Use(corsMiddleware())

	// User endpoints
	userRoutes := r.Group("/users")
	{
		userRoutes.GET("", cadwynInstance.WrapHandler(listUsers))
		userRoutes.GET("/:id", cadwynInstance.WrapHandler(getUser))
		userRoutes.POST("", cadwynInstance.WrapHandler(createUser))
		userRoutes.PUT("/:id", cadwynInstance.WrapHandler(updateUser))
		userRoutes.DELETE("/:id", cadwynInstance.WrapHandler(deleteUser))
	}

	// Product endpoints
	productRoutes := r.Group("/products")
	{
		productRoutes.GET("", cadwynInstance.WrapHandler(listProducts))
		productRoutes.GET("/:id", cadwynInstance.WrapHandler(getProduct))
		productRoutes.POST("", cadwynInstance.WrapHandler(createProduct))
	}

	// Meta endpoints (unversioned)
	r.GET("/health", healthCheck)
	r.GET("/versions", func(c *gin.Context) {
		versions := cadwynInstance.GetVersions()
		versionStrings := make([]string, len(versions))
		for i, v := range versions {
			versionStrings[i] = v.String()
		}
		c.JSON(http.StatusOK, gin.H{
			"versions":     versionStrings,
			"head_version": cadwynInstance.GetHeadVersion().String(),
		})
	})

	fmt.Println("🚀 Advanced Cadwyn-Go Example Server")
	fmt.Println("=====================================")
	fmt.Println("API Versions:")
	fmt.Println("  • 2023-01-01: Initial release")
	fmt.Println("  • 2023-06-01: Added email to users")
	fmt.Println("  • 2024-01-01: Added phone to users, renamed name -> full_name")
	fmt.Println("  •       head: Latest (all features)")
	fmt.Println("\nEndpoints:")
	fmt.Println("  GET    /users")
	fmt.Println("  GET    /users/:id")
	fmt.Println("  POST   /users")
	fmt.Println("  PUT    /users/:id")
	fmt.Println("  DELETE /users/:id")
	fmt.Println("  GET    /products")
	fmt.Println("  GET    /products/:id")
	fmt.Println("  POST   /products")
	fmt.Println("\nServer running on http://localhost:8080")
	fmt.Println("Use X-API-Version header to specify version")

	r.Run(":8080")
}

// ============================================================================
// User Handlers (all implement HEAD version)
// ============================================================================

func listUsers(c *gin.Context) {
	userList := make([]User, 0, len(users))
	for _, user := range users {
		userList = append(userList, user)
	}
	c.JSON(http.StatusOK, userList)
}

func getUser(c *gin.Context) {
	id := parseID(c)
	user, exists := users[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}
	c.JSON(http.StatusOK, user)
}

func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.ID = nextUserID
	nextUserID++
	users[user.ID] = user

	c.JSON(http.StatusCreated, user)
}

func updateUser(c *gin.Context) {
	id := parseID(c)
	if _, exists := users[id]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.ID = id
	users[id] = user
	c.JSON(http.StatusOK, user)
}

func deleteUser(c *gin.Context) {
	id := parseID(c)
	if _, exists := users[id]; !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "user not found"})
		return
	}

	delete(users, id)
	c.JSON(http.StatusNoContent, nil)
}

// ============================================================================
// Product Handlers (all implement HEAD version)
// ============================================================================

func listProducts(c *gin.Context) {
	productList := make([]Product, 0, len(products))
	for _, product := range products {
		productList = append(productList, product)
	}
	c.JSON(http.StatusOK, productList)
}

func getProduct(c *gin.Context) {
	id := parseID(c)
	product, exists := products[id]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}
	c.JSON(http.StatusOK, product)
}

func createProduct(c *gin.Context) {
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product.ID = nextProductID
	nextProductID++
	products[product.ID] = product

	c.JSON(http.StatusCreated, product)
}

// ============================================================================
// Version Migrations
// ============================================================================

// v1 -> v2: Add email field to User
func createUserV1ToV2Migration(from, to *cadwyn.Version) *cadwyn.VersionChange {
	return cadwyn.NewVersionChange(
		"Add email field to User",
		from,
		to,
		// Forward migration: v1 request -> v2 (add email)
		&cadwyn.AlterRequestInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(req *cadwyn.RequestInfo) error {
				if userMap, ok := req.Body.(map[string]interface{}); ok {
					if _, hasEmail := userMap["email"]; !hasEmail {
						userMap["email"] = "unknown@example.com"
					}
				}
				return nil
			},
		},
		// Backward migration: v2 response -> v1 (remove email)
		&cadwyn.AlterResponseInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(resp *cadwyn.ResponseInfo) error {
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
}

// v2 -> v3: Add phone, rename name -> full_name
func createUserV2ToV3Migration(from, to *cadwyn.Version) *cadwyn.VersionChange {
	return cadwyn.NewVersionChange(
		"Add phone field and rename name to full_name",
		from,
		to,
		// Forward migration: v2 request -> v3
		&cadwyn.AlterRequestInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(req *cadwyn.RequestInfo) error {
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
		// Backward migration: v3 response -> v2
		&cadwyn.AlterResponseInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(resp *cadwyn.ResponseInfo) error {
				transformUser := func(userMap map[string]interface{}) {
					// Rename full_name -> name
					if fullName, hasFullName := userMap["full_name"]; hasFullName {
						userMap["name"] = fullName
						delete(userMap, "full_name")
					}
					// Remove phone
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
}

// v2 -> v3: Add description and currency to Product
func createProductV2ToV3Migration(from, to *cadwyn.Version) *cadwyn.VersionChange {
	return cadwyn.NewVersionChange(
		"Add description and currency fields to Product",
		from,
		to,
		// Forward migration: v2 request -> v3
		&cadwyn.AlterRequestInstruction{
			Schemas: []interface{}{Product{}},
			Transformer: func(req *cadwyn.RequestInfo) error {
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
		// Backward migration: v3 response -> v2
		&cadwyn.AlterResponseInstruction{
			Schemas: []interface{}{Product{}},
			Transformer: func(resp *cadwyn.ResponseInfo) error {
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
}

// ============================================================================
// Utility Functions
// ============================================================================

func parseID(c *gin.Context) int {
	var id int
	fmt.Sscanf(c.Param("id"), "%d", &id)
	return id
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Version")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

/*
Try it out:

# Get user with v1 (2023-01-01) - no email or phone
curl http://localhost:8080/users/1 \
  -H "X-API-Version: 2023-01-01"
# Response: {"id":1,"name":"Alice Johnson"}

# Get user with v2 (2023-06-01) - has email, no phone
curl http://localhost:8080/users/1 \
  -H "X-API-Version: 2023-06-01"
# Response: {"id":1,"name":"Alice Johnson","email":"alice@example.com"}

# Get user with v3 (2024-01-01) - has email, phone, and full_name
curl http://localhost:8080/users/1 \
  -H "X-API-Version: 2024-01-01"
# Response: {"id":1,"full_name":"Alice Johnson","email":"alice@example.com","phone":"+1-555-0100"}

# Get user with HEAD (latest)
curl http://localhost:8080/users/1 \
  -H "X-API-Version: head"

# Create user with v1 (old field name)
curl -X POST http://localhost:8080/users \
  -H "X-API-Version: 2023-01-01" \
  -H "Content-Type: application/json" \
  -d '{"name":"Charlie Brown"}'

# Create user with v3 (new field name)
curl -X POST http://localhost:8080/users \
  -H "X-API-Version: 2024-01-01" \
  -H "Content-Type: application/json" \
  -d '{"full_name":"Diana Prince","email":"diana@example.com","phone":"+1-555-0300"}'

# List all users (see migrations in action for arrays)
curl http://localhost:8080/users \
  -H "X-API-Version: 2023-01-01"

# Get products with v2 (no description/currency)
curl http://localhost:8080/products/1 \
  -H "X-API-Version: 2023-06-01"

# Get products with v3 (has description/currency)
curl http://localhost:8080/products/1 \
  -H "X-API-Version: 2024-01-01"

# Check available versions
curl http://localhost:8080/versions
*/
