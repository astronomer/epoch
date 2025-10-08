package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/gin-gonic/gin"
)

// User model evolution:
// 2023-01-01: ID, Name
// 2023-06-01: Added Email, Added Status (only "active" and "inactive")
// 2024-01-01: Added Phone, renamed Name -> FullName, Status gains "pending" and "suspended"
type User struct {
	ID       int    `json:"id"`
	FullName string `json:"full_name"`                                        // Was "name" before 2024-01-01
	Email    string `json:"email"`                                            // Added in 2023-06-01
	Phone    string `json:"phone"`                                            // Added in 2024-01-01
	Status   string `json:"status" enums:"active,inactive,pending,suspended"` // Added in 2023-06-01, expanded in 2024-01-01
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

// Order model for demonstrating path-based migrations
// 2024-01-01: Added Order model
type Order struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total"`
	CreatedAt string  `json:"created_at"`
}

// Valid user status values (using enums tag on User.Status field)
// In v1: Status field doesn't exist
// In v2: Status field exists with "active" and "inactive"
// In v3+: Status field has all four values
const (
	StatusActive    = "active"
	StatusInactive  = "inactive"
	StatusPending   = "pending"   // Added in v3 (2024-01-01)
	StatusSuspended = "suspended" // Added in v3 (2024-01-01)
)

// In-memory storage (for demo purposes)
var (
	users = map[int]User{
		1: {ID: 1, FullName: "Alice Johnson", Email: "alice@example.com", Phone: "+1-555-0100", Status: StatusActive},
		2: {ID: 2, FullName: "Bob Smith", Email: "bob@example.com", Phone: "+1-555-0200", Status: StatusActive},
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
	v1, _ := epoch.NewDateVersion("2023-01-01")
	v2, _ := epoch.NewDateVersion("2023-06-01")
	v3, _ := epoch.NewDateVersion("2024-01-01")

	// Build Epoch instance with ALL types of migrations
	epochInstance, err := epoch.NewEpoch().
		WithVersions(v1, v2, v3).
		WithHeadVersion().
		WithChanges(
			// DEMONSTRATION 1: Schema-based request/response migrations
			createUserV1ToV2Migration(v1, v2),
			createUserV2ToV3Migration(v2, v3),
			createProductV2ToV3Migration(v2, v3),

			// DEMONSTRATION 2: Path-based migrations
			createPathBasedOrderMigration(v2, v3),

			// DEMONSTRATION 3: Enum instructions
			createEnumMigration(v2, v3),

			// DEMONSTRATION 4: Schema instructions
			createSchemaMigration(v2, v3),

			// DEMONSTRATION 5: Endpoint instructions
			createEndpointMigration(v2, v3),

			// DEMONSTRATION 6: Error response migrations
			createErrorMigration(v1, v2),
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

	// Order endpoints (demonstrating path-based migrations)
	orderRoutes := r.Group("/orders")
	{
		orderRoutes.GET("", epochInstance.WrapHandler(listOrders))
		orderRoutes.POST("", epochInstance.WrapHandler(createOrder))
	}

	// Error endpoint (demonstrating error migrations)
	r.GET("/error", epochInstance.WrapHandler(errorEndpoint))

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

	fmt.Println("🚀 Advanced Epoch Example Server - ALL Migration Types")
	fmt.Println("===========================================================")
	fmt.Println("This example demonstrates ALL possible migration types:")
	fmt.Println()
	fmt.Println("📋 Migration Types Demonstrated:")
	fmt.Println("  1. Schema-based Request Migrations")
	fmt.Println("  2. Schema-based Response Migrations")
	fmt.Println("  3. Path-based Request Migrations")
	fmt.Println("  4. Path-based Response Migrations")
	fmt.Println("  5. Schema Instructions (field changes)")
	fmt.Println("  6. Enum Instructions (enum member changes)")
	fmt.Println("  7. Endpoint Instructions (endpoint availability)")
	fmt.Println("  8. Error Response Migrations (MigrateHTTPErrors)")
	fmt.Println()
	fmt.Println("📅 API Versions:")
	fmt.Println("  • 2023-01-01: Initial release (users with id, name)")
	fmt.Println("  • 2023-06-01: Added email and status to users")
	fmt.Println("  • 2024-01-01: Added phone, renamed name->full_name, expanded status enum, orders endpoint")
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
	fmt.Println("  GET    /orders         - List all orders (v3+)")
	fmt.Println("  POST   /orders         - Create order (v3+)")
	fmt.Println("  GET    /error          - Test error migrations")
	fmt.Println()
	fmt.Println("Server running on http://localhost:8080")
	fmt.Println("Use X-API-Version header to specify version")

	r.Run(":8080")
}

// ============================================================================
// DEMONSTRATION 1: Schema-based Request/Response Migrations
// ============================================================================

// v1 -> v2: Add email and status fields to User (SCHEMA-BASED)
func createUserV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Add email and status fields to User",
		from,
		to,
		// Forward migration: v1 request -> v2 (add email and status)
		&epoch.AlterRequestInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(req *epoch.RequestInfo) error {
				if userMap, ok := req.Body.(map[string]interface{}); ok {
					if _, hasEmail := userMap["email"]; !hasEmail {
						userMap["email"] = "unknown@example.com"
					}
					if _, hasStatus := userMap["status"]; !hasStatus {
						userMap["status"] = StatusActive
					}
				}
				return nil
			},
		},
		// Backward migration: v2 response -> v1 (remove email and status)
		&epoch.AlterResponseInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(resp *epoch.ResponseInfo) error {
				transformUser := func(userMap map[string]interface{}) {
					delete(userMap, "email")
					delete(userMap, "status")
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

// v2 -> v3: Add phone, rename name -> full_name (SCHEMA-BASED)
func createUserV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Add phone field and rename name to full_name",
		from,
		to,
		// Forward migration: v2 request -> v3
		&epoch.AlterRequestInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(req *epoch.RequestInfo) error {
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
		&epoch.AlterResponseInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(resp *epoch.ResponseInfo) error {
				transformUser := func(userMap map[string]interface{}) {
					// Rename full_name -> name
					if fullName, hasFullName := userMap["full_name"]; hasFullName {
						userMap["name"] = fullName
						delete(userMap, "full_name")
					}
					// Remove phone (status stays - it exists in v2)
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

// v2 -> v3: Add description and currency to Product (SCHEMA-BASED)
func createProductV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Add description and currency fields to Product",
		from,
		to,
		// Forward migration: v2 request -> v3
		&epoch.AlterRequestInstruction{
			Schemas: []interface{}{Product{}},
			Transformer: func(req *epoch.RequestInfo) error {
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
		&epoch.AlterResponseInstruction{
			Schemas: []interface{}{Product{}},
			Transformer: func(resp *epoch.ResponseInfo) error {
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
// DEMONSTRATION 2: Path-based Migrations
// ============================================================================

// v2 -> v3: Add Order endpoint with PATH-BASED migrations
func createPathBasedOrderMigration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Add Order endpoints with path-based migrations",
		from,
		to,
		// PATH-BASED Request Migration for /orders POST
		&epoch.AlterRequestInstruction{
			Path:    "/orders",
			Methods: []string{"POST"},
			Transformer: func(req *epoch.RequestInfo) error {
				// Add created_at timestamp to all order creation requests
				if orderMap, ok := req.Body.(map[string]interface{}); ok {
					if _, hasCreatedAt := orderMap["created_at"]; !hasCreatedAt {
						orderMap["created_at"] = time.Now().Format(time.RFC3339)
					}
					// Calculate total if not provided
					if _, hasTotal := orderMap["total"]; !hasTotal {
						if quantity, ok := orderMap["quantity"].(float64); ok {
							// In a real app, you'd look up the product price
							orderMap["total"] = quantity * 99.99
						}
					}
				}
				return nil
			},
		},
		// PATH-BASED Response Migration for /orders GET
		&epoch.AlterResponseInstruction{
			Path:    "/orders",
			Methods: []string{"GET"},
			Transformer: func(resp *epoch.ResponseInfo) error {
				// For older versions, simplify order responses
				if orderList, ok := resp.Body.([]interface{}); ok {
					for _, item := range orderList {
						if orderMap, ok := item.(map[string]interface{}); ok {
							// Remove created_at for older versions
							delete(orderMap, "created_at")
						}
					}
				}
				return nil
			},
			MigrateHTTPErrors: false, // Don't migrate errors for this path
		},
	)
}

// ============================================================================
// DEMONSTRATION 3: Enum Instructions
// ============================================================================

// v2 -> v3: Add new enum members to User.status
func createEnumMigration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Add 'pending' and 'suspended' status values to User.status enum",
		from,
		to,
		// Enum instruction showing that v3 has additional members
		// The User.Status field has enums:"active,inactive,pending,suspended" tag
		&epoch.EnumInstruction{
			Enum: "User.Status", // Reference the field by name
			Type: "had_members",
			Members: map[string]interface{}{
				"pending":   "pending",   // Added in v3
				"suspended": "suspended", // Added in v3
			},
			IsHidden: false,
		},
		// Handle the enum values in responses
		&epoch.AlterResponseInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(resp *epoch.ResponseInfo) error {
				normalizeStatus := func(userMap map[string]interface{}) {
					if status, hasStatus := userMap["status"]; hasStatus {
						// Map new statuses to old ones for backward compatibility
						statusStr, ok := status.(string)
						if !ok {
							return
						}
						switch statusStr {
						case "pending", "suspended":
							// Map to "inactive" for older versions
							userMap["status"] = "inactive"
						}
					}
				}

				if userMap, ok := resp.Body.(map[string]interface{}); ok {
					normalizeStatus(userMap)
				} else if userList, ok := resp.Body.([]interface{}); ok {
					for _, item := range userList {
						if userMap, ok := item.(map[string]interface{}); ok {
							normalizeStatus(userMap)
						}
					}
				}
				return nil
			},
		},
	)
}

// ============================================================================
// DEMONSTRATION 4: Schema Instructions
// ============================================================================

// v2 -> v3: Schema instruction describing field changes
func createSchemaMigration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Schema changes: User gained phone, status, and full_name fields",
		from,
		to,
		// Schema instruction documenting what changed
		&epoch.SchemaInstruction{
			Schema: User{},
			Name:   "User",
			Type:   "had",
			Attributes: map[string]interface{}{
				"phone": map[string]interface{}{
					"type":        "string",
					"added_in":    "2024-01-01",
					"description": "User phone number",
				},
				"status": map[string]interface{}{
					"type":        "string",
					"added_in":    "2023-06-01",
					"description": "User status enum",
					"enums":       "active,inactive,pending,suspended",
					"note":        "Field added in v2 with active/inactive; pending/suspended added in v3",
				},
				"full_name": map[string]interface{}{
					"type":         "string",
					"renamed_from": "name",
					"changed_in":   "2024-01-01",
				},
			},
			IsHidden: false,
		},
	)
}

// ============================================================================
// DEMONSTRATION 5: Endpoint Instructions
// ============================================================================

// v2 -> v3: Endpoint instruction showing new endpoints
func createEndpointMigration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Add /orders endpoints",
		from,
		to,
		// Endpoint instruction showing /orders didn't exist in v2
		&epoch.EndpointInstruction{
			Path:     "/orders",
			Methods:  []string{"GET", "POST"},
			FuncName: "listOrders, createOrder",
			Type:     "didnt_exist",
			Attributes: map[string]interface{}{
				"description": "Order management endpoints",
				"added_in":    "2024-01-01",
			},
			IsHidden: false,
		},
	)
}

// ============================================================================
// DEMONSTRATION 6: Error Response Migrations
// ============================================================================

// v1 -> v2: Demonstrate MigrateHTTPErrors flag
func createErrorMigration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChange(
		"Change error response format",
		from,
		to,
		// Migrate error responses (MigrateHTTPErrors: true)
		&epoch.AlterResponseInstruction{
			Schemas:           []interface{}{User{}},
			MigrateHTTPErrors: true, // This will also transform error responses!
			Transformer: func(resp *epoch.ResponseInfo) error {
				// v2+ has structured error format, v1 has simple error format
				if resp.StatusCode >= 400 {
					if bodyMap, ok := resp.Body.(map[string]interface{}); ok {
						// Convert v2 structured error to v1 simple error
						if errMsg, hasError := bodyMap["error"]; hasError {
							if errDetails, ok := errMsg.(map[string]interface{}); ok {
								// v2 has {error: {message: "...", code: "..."}}
								// v1 has {error: "..."}
								if msg, hasMsg := errDetails["message"]; hasMsg {
									bodyMap["error"] = msg
								}
							}
						}
					}
				}
				return nil
			},
		},
	)
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
		c.JSON(http.StatusNotFound, gin.H{
			"error": map[string]interface{}{
				"message": "user not found",
				"code":    "USER_NOT_FOUND",
			},
		})
		return
	}
	c.JSON(http.StatusOK, user)
}

func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"message": err.Error(),
				"code":    "INVALID_REQUEST",
			},
		})
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
		c.JSON(http.StatusNotFound, gin.H{
			"error": map[string]interface{}{
				"message": "user not found",
				"code":    "USER_NOT_FOUND",
			},
		})
		return
	}

	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"message": err.Error(),
				"code":    "INVALID_REQUEST",
			},
		})
		return
	}

	user.ID = id
	users[id] = user
	c.JSON(http.StatusOK, user)
}

func deleteUser(c *gin.Context) {
	id := parseID(c)
	if _, exists := users[id]; !exists {
		c.JSON(http.StatusNotFound, gin.H{
			"error": map[string]interface{}{
				"message": "user not found",
				"code":    "USER_NOT_FOUND",
			},
		})
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
		c.JSON(http.StatusNotFound, gin.H{
			"error": map[string]interface{}{
				"message": "product not found",
				"code":    "PRODUCT_NOT_FOUND",
			},
		})
		return
	}
	c.JSON(http.StatusOK, product)
}

func createProduct(c *gin.Context) {
	var product Product
	if err := c.ShouldBindJSON(&product); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"message": err.Error(),
				"code":    "INVALID_REQUEST",
			},
		})
		return
	}

	product.ID = nextProductID
	nextProductID++
	products[product.ID] = product

	c.JSON(http.StatusCreated, product)
}

// ============================================================================
// Order Handlers (demonstrating path-based migrations)
// ============================================================================

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
		c.JSON(http.StatusBadRequest, gin.H{
			"error": map[string]interface{}{
				"message": err.Error(),
				"code":    "INVALID_REQUEST",
			},
		})
		return
	}

	order.ID = nextOrderID
	nextOrderID++
	orders[order.ID] = order

	c.JSON(http.StatusCreated, order)
}

// ============================================================================
// Error Endpoint (demonstrating error migrations)
// ============================================================================

func errorEndpoint(c *gin.Context) {
	c.JSON(http.StatusBadRequest, gin.H{
		"error": map[string]interface{}{
			"message": "This is a test error",
			"code":    "TEST_ERROR",
		},
	})
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
🎯 Try ALL Migration Types:

# ========================================
# 1. SCHEMA-BASED MIGRATIONS
# ========================================

# Get user with v1 (2023-01-01) - no email, phone, or status
curl http://localhost:8080/users/1 \
  -H "X-API-Version: 2023-01-01"
# Response: {"id":1,"name":"Alice Johnson"}

# Get user with v2 (2023-06-01) - has email and status, no phone
curl http://localhost:8080/users/1 \
  -H "X-API-Version: 2023-06-01"
# Response: {"id":1,"name":"Alice Johnson","email":"alice@example.com","status":"active"}

# Get user with v3 (2024-01-01) - has email, phone, full_name, and all status values
curl http://localhost:8080/users/1 \
  -H "X-API-Version: 2024-01-01"
# Response: {"id":1,"full_name":"Alice Johnson","email":"alice@example.com","phone":"+1-555-0100","status":"active"}

# ========================================
# 2. PATH-BASED MIGRATIONS
# ========================================

# Create order with v3 (path-based migration adds created_at)
curl -X POST http://localhost:8080/orders \
  -H "X-API-Version: 2024-01-01" \
  -H "Content-Type: application/json" \
  -d '{"user_id":1,"product_id":1,"quantity":2}'
# Response includes created_at timestamp

# List orders with v2 (should have all fields - path migrations need route binding)
curl http://localhost:8080/orders \
  -H "X-API-Version: 2023-06-01"
# Note: Path-based migrations require route binding via BindRouteToRequestMigrations/BindRouteToResponseMigrations
# This example shows the structure; implement route binding in production for path-specific transformations
# Schema-based migrations work globally; path-based migrations need explicit route registration

# ========================================
# 3. ENUM MIGRATIONS
# ========================================

# Create user with new status value (v3+)
curl -X POST http://localhost:8080/users \
  -H "X-API-Version: 2024-01-01" \
  -H "Content-Type: application/json" \
  -d '{"full_name":"Charlie","email":"charlie@example.com","phone":"+1-555-0300","status":"pending"}'

# Get that user with v2 - status "pending" should be mapped to "inactive" by enum migration
curl http://localhost:8080/users/3 \
  -H "X-API-Version: 2023-06-01"
# Response: {"id":3,"name":"Charlie","email":"charlie@example.com","status":"inactive"}

# ========================================
# 4. ERROR RESPONSE MIGRATIONS
# ========================================

# Get error with v2 (structured error format)
curl http://localhost:8080/error \
  -H "X-API-Version: 2023-06-01"
# Response: {"error":{"message":"This is a test error","code":"TEST_ERROR"}}

# Get error with v1 (simple error format - MigrateHTTPErrors in action!)
curl http://localhost:8080/error \
  -H "X-API-Version: 2023-01-01"
# Response: {"error":"This is a test error"}

# ========================================
# 5. ENDPOINT INSTRUCTIONS
# ========================================

# Try accessing /orders with v2 (endpoint didn't exist)
curl http://localhost:8080/orders \
  -H "X-API-Version: 2023-06-01"
# Works! Endpoint instruction documents it, but middleware allows access

# ========================================
# 6. COMBINED EXAMPLES
# ========================================

# Create user with v1 format, auto-adds email/phone/status
curl -X POST http://localhost:8080/users \
  -H "X-API-Version: 2023-01-01" \
  -H "Content-Type: application/json" \
  -d '{"name":"Diana Prince"}'

# List all users to see migrations in action on arrays
curl http://localhost:8080/users

# Check available versions
curl http://localhost:8080/versions
*/
