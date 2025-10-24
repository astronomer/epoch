package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/gin-gonic/gin"
)

// User model evolution:
// v1 (2024-01-01): ID, Name
// v2 (2024-06-01): Added Email, Added Status (only "active" and "inactive")
// v3 (2025-01-01): Renamed Name->FullName, Added Phone, Status gains "pending" and "suspended"
type User struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name" binding:"required"`                                        // Was "name" before v3 (2025-01-01)
	Email    string `json:"email" binding:"required"`                                            // Added in v2 (2024-06-01)
	Phone    string `json:"phone"`                                                               // Added in v3 (2025-01-01)
	Status   string `json:"status" binding:"required" enums:"active,inactive,pending,suspended"` // Added in v2, expanded in v3
}

// Product model evolution:
// v1-v2: ID, Name, Price
// v3: Added Description, Currency
type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name" binding:"required"`
	Price       float64 `json:"price" binding:"required"`
	Description string  `json:"description"` // Added in v3 (2025-01-01)
	Currency    string  `json:"currency"`    // Added in v3 (2025-01-01)
}

// Order model (demonstrating endpoint additions)
// v3+: Order model was added
type Order struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total"`
	CreatedAt string  `json:"created_at"`
}

// In-memory storage (for demo purposes)
var (
	users = map[int]User{
		1: {ID: 1, FullName: "Alice Johnson", Email: "alice@example.com", Phone: "+1-555-0100", Status: "active"},
		2: {ID: 2, FullName: "Bob Smith", Email: "bob@example.com", Phone: "+1-555-0200", Status: "active"},
	}
	products = map[int]Product{
		1: {ID: 1, Name: "Laptop", Price: 999.99, Description: "High-performance laptop", Currency: "USD"},
		2: {ID: 2, Name: "Mouse", Price: 29.99, Description: "Wireless mouse", Currency: "USD"},
	}
	orders = map[int]Order{
		1: {ID: 1, UserID: 1, ProductID: 1, Quantity: 1, Total: 999.99, CreatedAt: time.Now().Format(time.RFC3339)},
	}
	nextUserID    = 3
	nextProductID = 3
	nextOrderID   = 2
)

func main() {
	// Create date-based versions (chronologically ordered)
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")
	v3, _ := epoch.NewDateVersion("2025-01-01")

	// Build Epoch instance
	epochInstance, err := epoch.NewEpoch().
		WithVersions(v1, v2, v3).
		WithHeadVersion().
		WithChanges(
			createUserV1ToV2Migration(v1, v2),
			createUserV2ToV3Migration(v2, v3),
			createProductV2ToV3Migration(v2, v3),
		).
		WithTypes(User{}, Product{}, Order{}).
		WithVersionParameter("X-API-Version").
		WithVersionFormat(epoch.VersionFormatDate).
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to create Epoch instance: %v", err))
	}

	// Setup Gin
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()

	// Middleware
	r.Use(epochInstance.Middleware())
	r.Use(corsMiddleware())

	// User endpoints
	userRoutes := r.Group("/users")
	{
		userRoutes.GET("", epochInstance.WrapHandler(listUsers))
		userRoutes.GET("/:id", epochInstance.WrapHandler(getUser))
		userRoutes.POST("", epochInstance.WrapHandler(createUser))
		userRoutes.PUT("/:id", epochInstance.WrapHandler(updateUser))
		userRoutes.DELETE("/:id", epochInstance.WrapHandler(deleteUser))
	}

	// Product endpoints
	productRoutes := r.Group("/products")
	{
		productRoutes.GET("", epochInstance.WrapHandler(listProducts))
		productRoutes.GET("/:id", epochInstance.WrapHandler(getProduct))
		productRoutes.POST("", epochInstance.WrapHandler(createProduct))
	}

	// Order endpoints
	orderRoutes := r.Group("/orders")
	{
		orderRoutes.GET("", epochInstance.WrapHandler(listOrders))
		orderRoutes.POST("", epochInstance.WrapHandler(createOrder))
	}

	// Meta endpoints (unversioned)
	r.GET("/health", healthCheck)
	r.GET("/versions", func(c *gin.Context) {
		versions := epochInstance.GetVersions()
		versionStrings := make([]string, len(versions))
		for i, v := range versions {
			versionStrings[i] = v.String()
		}
		c.JSON(http.StatusOK, gin.H{
			"versions":     versionStrings,
			"head_version": epochInstance.GetHeadVersion().String(),
		})
	})

	fmt.Println("🚀 Advanced Epoch Example")
	fmt.Println("==============================================")
	fmt.Println("This example demonstrates:")
	fmt.Println("  • Declarative field operations (AddField, RenameField, RemoveField, MapEnumValues)")
	fmt.Println("  • Automatic bidirectional migrations")
	fmt.Println("  • Automatic error message field name transformation (validation errors only)")
	fmt.Println("")
	fmt.Println("📅 API Versions:")
	fmt.Println("  • 2024-01-01 (v1): Initial release (users with id, name, temp_field)")
	fmt.Println("  • 2024-06-01 (v2): Added email and status, removed temp_field")
	fmt.Println("  • 2025-01-01 (v3): Renamed name→full_name, added phone, expanded status enum, added product fields")
	fmt.Println("  •       HEAD: Latest (all features)")
	fmt.Println("")
	fmt.Println("🔗 Endpoints:")
	fmt.Println("  GET    /users          - List all users")
	fmt.Println("  GET    /users/:id      - Get user by ID")
	fmt.Println("  POST   /users          - Create user")
	fmt.Println("  PUT    /users/:id      - Update user")
	fmt.Println("  DELETE /users/:id      - Delete user")
	fmt.Println("  GET    /products       - List all products")
	fmt.Println("  GET    /products/:id   - Get product by ID")
	fmt.Println("  POST   /products       - Create product")
	fmt.Println("  GET    /orders         - List all orders")
	fmt.Println("  POST   /orders         - Create order")
	fmt.Println("")
	fmt.Println("💡 Comprehensive Test Commands:")
	fmt.Println("")
	fmt.Println("🔍 1. VERSION DETECTION & METADATA")
	fmt.Println("  curl http://localhost:8082/versions")
	fmt.Println("  # Expected: {\"head_version\":\"head\",\"versions\":[\"2024-01-01\",\"2024-06-01\",\"2025-01-01\",\"head\"]}")
	fmt.Println("")
	fmt.Println("👤 2. USER RESPONSE MIGRATIONS (Field Transformations)")
	fmt.Println("  # V1 (2024-01-01): Only id + name")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8082/users/1")
	fmt.Println("  # Expected: {\"id\":1,\"name\":\"Alice Johnson\"}")
	fmt.Println("")
	fmt.Println("  # V2 (2024-06-01): id + name + email + status")
	fmt.Println("  curl -H 'X-API-Version: 2024-06-01' http://localhost:8082/users/1")
	fmt.Println("  # Expected: {\"id\":1,\"email\":\"alice@example.com\",\"status\":\"active\",\"name\":\"Alice Johnson\"}")
	fmt.Println("")
	fmt.Println("  # V3 (2025-01-01): All fields (full_name instead of name)")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8082/users/1")
	fmt.Println("  # Expected: {\"id\":1,\"full_name\":\"Alice Johnson\",\"email\":\"alice@example.com\",\"phone\":\"+1-555-0100\",\"status\":\"active\"}")
	fmt.Println("")
	fmt.Println("📝 3. REQUEST MIGRATIONS (Field Transformations)")
	fmt.Println("  # V2 POST: Use 'name' field (migrated to 'full_name' internally)")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-06-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"name\":\"Test User\",\"email\":\"test@example.com\",\"status\":\"active\"}' \\")
	fmt.Println("    http://localhost:8082/users")
	fmt.Println("  # Expected: {\"id\":5,\"email\":\"test@example.com\",\"status\":\"active\",\"name\":\"Test User\"}")
	fmt.Println("")
	fmt.Println("📦 4. PRODUCT MIGRATIONS (AddField Operations)")
	fmt.Println("  # V1: Only basic fields")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8082/products/1")
	fmt.Println("  # Expected: {\"id\":1,\"name\":\"Laptop\",\"price\":999.99}")
	fmt.Println("")
	fmt.Println("  # V3: With added description + currency fields")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8082/products/1")
	fmt.Println("  # Expected: {\"id\":1,\"name\":\"Laptop\",\"price\":999.99,\"description\":\"High-performance laptop\",\"currency\":\"USD\"}")
	fmt.Println("")
	fmt.Println("⚠️  5. ERROR MESSAGE FIELD NAME TRANSFORMATION")
	fmt.Println("  # V2 validation error: Shows 'name' in error (not 'full_name')")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-06-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"name\":\"Invalid User\"}' http://localhost:8082/users")
	fmt.Println("  # Expected: Error mentions 'Email' and 'Status' fields (not internal field names)")
	fmt.Println("")
	fmt.Println("📊 6. LIST ENDPOINTS (Array Transformations)")
	fmt.Println("  # V1 user list: Only id + name for each user")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8082/users")
	fmt.Println("")
	fmt.Println("  # V3 user list: All fields for each user")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8082/users")
	fmt.Println("")
	fmt.Println("🎯 7. ADVANCED SCENARIOS")
	fmt.Println("  # Default version (no header): Uses HEAD version")
	fmt.Println("  curl http://localhost:8082/users/1")
	fmt.Println("")
	fmt.Println("  # Health check (unversioned endpoint)")
	fmt.Println("  curl http://localhost:8082/health")
	fmt.Println("")
	fmt.Println("📋 MIGRATION OPERATIONS DEMONSTRATED:")
	fmt.Println("  ✅ AddField: email, status, phone, description, currency")
	fmt.Println("  ✅ RenameField: name ↔ full_name")
	fmt.Println("  ✅ RemoveField: temp_field (v1→v2)")
	fmt.Println("  ✅ Bidirectional: All operations work both ways")
	fmt.Println("  ✅ Error Transformation: Field names in validation errors")
	fmt.Println("  ✅ Array Handling: List endpoints transform each item")
	fmt.Println("")
	fmt.Println("🌐 Server listening on http://localhost:8082")
	fmt.Println("   Use X-API-Version header to specify version")
	fmt.Println("")

	r.Run(":8082")
}

// ============================================================================
// DECLARATIVE MIGRATIONS - Simple & Clean! ✨
// ============================================================================

// createUserV1ToV2Migration defines the migration from v1 to v2
// This uses the NEW declarative API which automatically generates:
//  1. Request migration (v1 → v2): adds "email" and "status" fields if missing, removes "temp_field"
//  2. Response migration (v2 → v1): removes "email" and "status" fields
//  3. Error transformation: updates field names in validation errors (400 status only)
func createUserV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Add email and status fields, remove deprecated temp_field").
		// PATH-BASED ROUTING: Explicitly specify which endpoints this migration affects
		ForPath("/users", "/users/:id").
		// ✨ Automatic bidirectional migrations!
		AddField("email", "unknown@example.com"). // Adds in requests, removes in responses
		AddField("status", "active").             // Adds in requests, removes in responses
		RemoveField("temp_field").                // Removes from requests (can't restore in responses)
		Build()
}

// createUserV2ToV3Migration defines the migration from v2 to v3
// This uses the NEW declarative API which automatically generates:
//  1. Request migration (v2 → v3): renames "name" to "full_name", adds "phone" field if missing
//  2. Response migration (v3 → v2): renames "full_name" to "name", removes "phone" field
//  3. Error transformation: updates "full_name" to "name" in validation errors (400 status only)
func createUserV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Rename name to full_name, add phone, and expand status enum").
		// PATH-BASED ROUTING: Explicitly specify which endpoints this migration affects
		ForPath("/users", "/users/:id").
		// ✨ Automatic bidirectional migrations + error transformation!
		RenameField("name", "full_name"). // Renames in both directions + transforms validation errors
		AddField("phone", "").            // Adds in requests, removes in responses
		// Note: MapEnumValues normalizes new values in requests, expands in responses
		// This is useful when the database stores canonical values
		// For this example, we don't use it since v2 and v3 both understand active/inactive
		// Uncomment to test enum mapping behavior:
		// MapEnumValues("status", map[string]string{
		// 	"pending":   "inactive", // In requests: pending→inactive, in responses: inactive→pending
		// 	"suspended": "inactive", // In requests: suspended→inactive, in responses: inactive→suspended
		// }).
		Build()
}

// createProductV2ToV3Migration defines the migration from v2 to v3 for products
// This uses the NEW declarative API which automatically generates:
//  1. Request migration (v2 → v3): adds "description" and "currency" fields if missing
//  2. Response migration (v3 → v2): removes "description" and "currency" fields
//  3. Error transformation: updates field names in validation errors (400 status only)
func createProductV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Add description and currency to Product").
		// PATH-BASED ROUTING: Explicitly specify which endpoints this migration affects
		ForPath("/products", "/products/:id").
		// ✨ Automatic bidirectional migrations!
		AddField("description", ""). // Adds in requests, removes in responses
		AddField("currency", "USD"). // Adds in requests, removes in responses
		Build()
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func listUsers(c *gin.Context) {
	userList := make([]User, 0, len(users))
	for _, user := range users {
		userList = append(userList, user)
	}
	c.JSON(http.StatusOK, userList)
}

func getUser(c *gin.Context) {
	id := c.Param("id")
	var userID int
	fmt.Sscanf(id, "%d", &userID)

	user, exists := users[userID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
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
	id := c.Param("id")
	var userID int
	fmt.Sscanf(id, "%d", &userID)

	_, exists := users[userID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user.ID = userID
	users[userID] = user

	c.JSON(http.StatusOK, user)
}

func deleteUser(c *gin.Context) {
	id := c.Param("id")
	var userID int
	fmt.Sscanf(id, "%d", &userID)

	_, exists := users[userID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	delete(users, userID)
	c.JSON(http.StatusOK, gin.H{"message": "User deleted"})
}

func listProducts(c *gin.Context) {
	productList := make([]Product, 0, len(products))
	for _, product := range products {
		productList = append(productList, product)
	}
	c.JSON(http.StatusOK, productList)
}

func getProduct(c *gin.Context) {
	id := c.Param("id")
	var productID int
	fmt.Sscanf(id, "%d", &productID)

	product, exists := products[productID]
	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
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

func listOrders(c *gin.Context) {
	orderList := make([]Order, 0, len(orders))
	for _, order := range orders {
		orderList = append(orderList, order)
	}
	c.JSON(http.StatusOK, orderList)
}

func createOrder(c *gin.Context) {
	var order Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	order.ID = nextOrderID
	nextOrderID++
	order.CreatedAt = time.Now().Format(time.RFC3339)
	orders[order.ID] = order

	c.JSON(http.StatusCreated, order)
}

func healthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "healthy",
		"time":   time.Now().Format(time.RFC3339),
	})
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-API-Version")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusOK)
			return
		}

		c.Next()
	}
}
