package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/gin-gonic/gin"
)

// User model evolution:
// v1 (2025-01-01): ID, Name
// v2 (2025-06-01): Added Email, Added Status (only "active" and "inactive")
// v3 (2024-01-01): Renamed Name->FullName, Added Phone, Status gains "pending" and "suspended"
type User struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`                                        // Was "name" before v3 (2024-01-01)
	Email    string `json:"email"`                                            // Added in v2 (2025-06-01)
	Phone    string `json:"phone"`                                            // Added in v3 (2024-01-01)
	Status   string `json:"status" enums:"active,inactive,pending,suspended"` // Added in v2, expanded in v3
}

// Product model evolution:
// v1-v2: ID, Name, Price
// v3: Added Description, Currency
type Product struct {
	ID          int     `json:"id"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description"` // Added in v3 (2024-01-01)
	Currency    string  `json:"currency"`    // Added in v3 (2024-01-01)
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
	// Create date-based versions
	v1, _ := epoch.NewDateVersion("2025-01-01")
	v2, _ := epoch.NewDateVersion("2025-06-01")
	v3, _ := epoch.NewDateVersion("2024-01-01")

	// Build Epoch instance with DECLARATIVE migrations
	epochInstance, err := epoch.NewEpoch().
		WithVersions(v1, v2, v3).
		WithHeadVersion().
		WithChanges(
			// ✨ NEW DECLARATIVE API - Simple field operations are one-liners!
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

	fmt.Println("🚀 Advanced Epoch Example - Declarative API")
	fmt.Println("==============================================")
	fmt.Println("This example demonstrates the NEW declarative API!")
	fmt.Println()
	fmt.Println("✨ What's New:")
	fmt.Println("  • Declarative field operations (AddField, RenameField, etc.)")
	fmt.Println("  • Automatic bidirectional migrations")
	fmt.Println("  • Automatic error message field name transformation")
	fmt.Println("  • 90% less code for common operations!")
	fmt.Println()
	fmt.Println("📅 API Versions:")
	fmt.Println("  • 2025-01-01 (v1): Initial release (users with id, name)")
	fmt.Println("  • 2025-06-01 (v2): Added email and status to users")
	fmt.Println("  • 2024-01-01 (v3): Renamed name->full_name, added phone, expanded status enum, added products fields")
	fmt.Println("  •       head: Latest (all features)")
	fmt.Println()
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
	fmt.Println()
	fmt.Println("💡 Try it:")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8080/users/1")
	fmt.Println("  curl -H 'X-API-Version: 2025-06-01' http://localhost:8080/users/1")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8080/users/1")
	fmt.Println()
	fmt.Println("Server running on http://localhost:8080")
	fmt.Println("Use X-API-Version header to specify version")

	r.Run(":8080")
}

// ============================================================================
// DECLARATIVE MIGRATIONS - Simple & Clean! ✨
// ============================================================================

// v1 -> v2: Add email and status fields to User
// BEFORE: 30+ lines of code with manual AST manipulation
// AFTER: 6 lines of declarative operations!
func createUserV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Add email and status fields to User").
		Schema(User{}).
		AddField("email", "unknown@example.com").
		AddField("status", "active").
		Build()
}

// v2 -> v3: Rename name to full_name, add phone, expand status enum
// BEFORE: 60+ lines of nested conditionals and AST manipulation
// AFTER: 8 lines of declarative operations!
func createUserV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Rename name to full_name, add phone, and expand status enum").
		Schema(User{}).
		RenameField("name", "full_name"). // Automatic bidirectional + error transformation!
		AddField("phone", "").
		MapEnumValues("status", map[string]string{
			"pending":   "active", // Map new values to old equivalents
			"suspended": "inactive",
		}).
		Build()
}

// v2 -> v3: Add description and currency to Product
// BEFORE: 40+ lines of code
// AFTER: 6 lines!
func createProductV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Add description and currency to Product").
		Schema(Product{}).
		AddField("description", "").
		AddField("currency", "USD").
		Build()
}

// ============================================================================
// HTTP Handlers (unchanged from old API)
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
