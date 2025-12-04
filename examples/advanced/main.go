package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/gin-gonic/gin"
)

// ============================================================================
// USER - REQUEST STRUCTS (HEAD version only)
// ============================================================================

// SkillRequest represents a skill in the request (HEAD version)
// v1: skill_name, no level
// v2+: name, level
type SkillRequest struct {
	Name  string `json:"name"`            // v1: "skill_name"
	Level int    `json:"level,omitempty"` // Added in v2
}

// ProfileSettingsRequest represents deeply nested settings in request (HEAD version)
// v1: color_theme
// v2+: theme
type ProfileSettingsRequest struct {
	Theme string `json:"theme,omitempty"` // v1: "color_theme"
}

// ProfileRequest represents a nested object in requests (HEAD version)
// v1: biography, skills[].skill_name
// v2+: bio, skills[].name + level
type ProfileRequest struct {
	Bio      string                  `json:"bio,omitempty"`      // v1: "biography"
	Skills   []SkillRequest          `json:"skills,omitempty"`   // Nested array in request
	Settings *ProfileSettingsRequest `json:"settings,omitempty"` // Deeply nested object
}

// CreateUserRequest - What clients send to create a user (HEAD version)
// v1 (2024-01-01): name, profile.biography, profile.skills[].skill_name
// v2 (2024-06-01): name, email, status, profile.bio, profile.skills[].name + level
// v3 (2025-01-01): full_name, email, phone, status, profile (all fields)
type CreateUserRequest struct {
	FullName string          `json:"full_name" binding:"required,max=100"`
	Email    string          `json:"email" binding:"required,email"`
	Phone    string          `json:"phone,omitempty"`
	Status   string          `json:"status" binding:"required,oneof=active inactive pending suspended"`
	Profile  *ProfileRequest `json:"profile,omitempty"` // Nested object with array and deeply nested settings
}

// UpdateUserRequest - What clients send to update a user (HEAD version)
type UpdateUserRequest struct {
	FullName string          `json:"full_name" binding:"required,max=100"`
	Email    string          `json:"email" binding:"required,email"`
	Phone    string          `json:"phone,omitempty"`
	Status   string          `json:"status" binding:"required,oneof=active inactive pending suspended"`
	Profile  *ProfileRequest `json:"profile,omitempty"` // Nested object with array and deeply nested settings
}

// ============================================================================
// USER - RESPONSE STRUCTS (HEAD version only)
// ============================================================================

// Skill represents an item in the profile.skills[] array (nested array inside nested object)
// v1: skill_name, no level
// v2+: name, level
type Skill struct {
	Name  string `json:"name"`  // v1: "skill_name"
	Level int    `json:"level"` // Added in v2
}

// ProfileSettings represents deeply nested settings (3 levels: user.profile.settings)
// v1: color_theme
// v2+: theme
type ProfileSettings struct {
	Theme string `json:"theme"` // v1: "color_theme"
}

// UserProfile represents a nested object containing an array and another nested object
// Demonstrates: nested object with array (profile.skills[]) AND deeply nested object (profile.settings)
// v1: biography
// v2+: bio
type UserProfile struct {
	Bio      string          `json:"bio"`      // v1: "biography"
	Skills   []Skill         `json:"skills"`   // Array inside nested object
	Settings ProfileSettings `json:"settings"` // 3-level deep nesting
}

// UserResponse - What API returns to clients (HEAD version)
// Migrations handle transforming this to v1/v2/v3 formats
// Now includes Profile for nested transformation demonstrations
type UserResponse struct {
	ID       int          `json:"id,omitempty"`
	FullName string       `json:"full_name"`
	Email    string       `json:"email,omitempty"`
	Phone    string       `json:"phone,omitempty"`
	Status   string       `json:"status,omitempty"`
	Profile  *UserProfile `json:"profile,omitempty"` // Nested object with array and deeply nested settings
}

type UsersListResponse struct {
	Users []UserResponse `json:"users"`
}

// ============================================================================
// USER - INTERNAL STORAGE MODEL
// ============================================================================

type SkillInternal struct {
	Name  string
	Level int
}

type ProfileSettingsInternal struct {
	Theme string
}

type UserProfileInternal struct {
	Bio      string
	Skills   []SkillInternal
	Settings ProfileSettingsInternal
}

type UserInternal struct {
	ID       int
	FullName string
	Email    string
	Phone    string
	Status   string
	Profile  *UserProfileInternal
}

// ============================================================================
// PRODUCT - REQUEST STRUCTS (HEAD version only)
// ============================================================================

// CreateProductRequest - What clients send to create a product (HEAD version)
// v1-v2: name, price
// v3: name, price, description, currency
type CreateProductRequest struct {
	Name        string  `json:"name" binding:"required"`
	Price       float64 `json:"price" binding:"required"`
	Description string  `json:"description,omitempty"`
	Currency    string  `json:"currency,omitempty"`
}

// ============================================================================
// PRODUCT - RESPONSE STRUCTS (HEAD version only)
// ============================================================================

// ProductResponse - What API returns to clients (HEAD version)
type ProductResponse struct {
	ID          int     `json:"id,omitempty"`
	Name        string  `json:"name"`
	Price       float64 `json:"price"`
	Description string  `json:"description,omitempty"`
	Currency    string  `json:"currency,omitempty"`
}

type ProductsListResponse struct {
	Products []ProductResponse `json:"products"`
}

// ============================================================================
// PRODUCT - INTERNAL STORAGE MODEL
// ============================================================================

type ProductInternal struct {
	ID          int
	Name        string
	Price       float64
	Description string
	Currency    string
}

// ============================================================================
// ORDER - REQUEST/RESPONSE STRUCTS (HEAD version only)
// ============================================================================

// CreateOrderRequest - What clients send to create an order (HEAD version)
type CreateOrderRequest struct {
	UserID    int `json:"user_id" binding:"required"`
	ProductID int `json:"product_id" binding:"required"`
	Quantity  int `json:"quantity" binding:"required,min=1"`
}

// OrderResponse - What API returns to clients (HEAD version)
type OrderResponse struct {
	ID        int     `json:"id"`
	UserID    int     `json:"user_id"`
	ProductID int     `json:"product_id"`
	Quantity  int     `json:"quantity"`
	Total     float64 `json:"total"`
	CreatedAt string  `json:"created_at"`
}

type OrdersListResponse struct {
	Orders []OrderResponse `json:"orders"`
}

// ============================================================================
// ORDER - INTERNAL STORAGE MODEL
// ============================================================================

type OrderInternal struct {
	ID        int
	UserID    int
	ProductID int
	Quantity  int
	Total     float64
	CreatedAt string
}

// ============================================================================
// EXAMPLE - RESPONSE STRUCTS (HEAD version only)
// ============================================================================

// SubItem represents items in a nested array (examples[].sub_items[]) - 2-level array nesting
// v1: name
// v2+: label
type SubItem struct {
	ID    int    `json:"id"`
	Label string `json:"label"` // v1: "name"
}

// ExamplesPaginated demonstrates nested array transformations (HEAD version)
type ExamplesPaginated struct {
	Examples   []ExampleItemResponse `json:"examples"`
	TotalCount int                   `json:"total_count"`
	Metadata   ExampleMetaResponse   `json:"metadata"`
}

// ExampleItemResponse - What API returns for example items (HEAD version)
// v1: id, name, tags
// v2: id, title (renamed from name), tags, category
// v3: id, display_name (renamed from title), tags, category, priority
// Now includes sub_items[] to demonstrate nested arrays (examples[].sub_items[])
type ExampleItemResponse struct {
	ID          int       `json:"id"`
	DisplayName string    `json:"display_name"`
	Tags        []string  `json:"tags"`
	Category    string    `json:"category,omitempty"`
	Priority    int       `json:"priority,omitempty"`
	SubItems    []SubItem `json:"sub_items,omitempty"` // Nested array inside array items
}

// ExampleMetaResponse demonstrates nested object transformations (HEAD version)
type ExampleMetaResponse struct {
	CreatedBy   string `json:"created_by"`
	LastUpdated string `json:"last_updated"` // Was "updated_at" in v1-v2
}

// ============================================================================
// EXAMPLE - INTERNAL STORAGE MODEL
// ============================================================================

type SubItemInternal struct {
	ID    int
	Label string
}

type ExampleItemInternal struct {
	ID          int
	DisplayName string
	Tags        []string
	Category    string
	Priority    int
	SubItems    []SubItemInternal
}

type ExampleMetaInternal struct {
	CreatedBy   string
	LastUpdated string
}

// ============================================================================
// CONVERSION FUNCTIONS
// ============================================================================

// User conversions
func NewUserResponse(u UserInternal) UserResponse {
	resp := UserResponse{
		ID:       u.ID,
		FullName: u.FullName,
		Email:    u.Email,
		Phone:    u.Phone,
		Status:   u.Status,
	}
	if u.Profile != nil {
		skills := make([]Skill, len(u.Profile.Skills))
		for i, s := range u.Profile.Skills {
			skills[i] = Skill{Name: s.Name, Level: s.Level}
		}
		resp.Profile = &UserProfile{
			Bio:    u.Profile.Bio,
			Skills: skills,
			Settings: ProfileSettings{
				Theme: u.Profile.Settings.Theme,
			},
		}
	}
	return resp
}

func NewUsersListResponse(users []UserInternal) UsersListResponse {
	responses := make([]UserResponse, len(users))
	for i, u := range users {
		responses[i] = NewUserResponse(u)
	}
	return UsersListResponse{Users: responses}
}

// Product conversions
func NewProductResponse(p ProductInternal) ProductResponse {
	return ProductResponse{
		ID:          p.ID,
		Name:        p.Name,
		Price:       p.Price,
		Description: p.Description,
		Currency:    p.Currency,
	}
}

func NewProductsListResponse(products []ProductInternal) ProductsListResponse {
	responses := make([]ProductResponse, len(products))
	for i, p := range products {
		responses[i] = NewProductResponse(p)
	}
	return ProductsListResponse{Products: responses}
}

// Order conversions
func NewOrderResponse(o OrderInternal) OrderResponse {
	return OrderResponse{
		ID:        o.ID,
		UserID:    o.UserID,
		ProductID: o.ProductID,
		Quantity:  o.Quantity,
		Total:     o.Total,
		CreatedAt: o.CreatedAt,
	}
}

func NewOrdersListResponse(orders []OrderInternal) OrdersListResponse {
	responses := make([]OrderResponse, len(orders))
	for i, o := range orders {
		responses[i] = NewOrderResponse(o)
	}
	return OrdersListResponse{Orders: responses}
}

// Example conversions
func NewExampleItemResponse(e ExampleItemInternal) ExampleItemResponse {
	subItems := make([]SubItem, len(e.SubItems))
	for i, s := range e.SubItems {
		subItems[i] = SubItem{ID: s.ID, Label: s.Label}
	}
	return ExampleItemResponse{
		ID:          e.ID,
		DisplayName: e.DisplayName,
		Tags:        e.Tags,
		Category:    e.Category,
		Priority:    e.Priority,
		SubItems:    subItems,
	}
}

func NewExamplesPaginated(items []ExampleItemInternal, meta ExampleMetaInternal) ExamplesPaginated {
	responses := make([]ExampleItemResponse, len(items))
	for i, item := range items {
		responses[i] = NewExampleItemResponse(item)
	}
	return ExamplesPaginated{
		Examples:   responses,
		TotalCount: len(items),
		Metadata: ExampleMetaResponse{
			CreatedBy:   meta.CreatedBy,
			LastUpdated: meta.LastUpdated,
		},
	}
}

// ============================================================================
// IN-MEMORY STORAGE (for demo purposes)
// ============================================================================

var (
	users = map[int]UserInternal{
		1: {
			ID: 1, FullName: "Alice Johnson", Email: "alice@example.com", Phone: "+1-555-0100", Status: "active",
			Profile: &UserProfileInternal{
				Bio: "Senior software engineer with 10 years of experience",
				Skills: []SkillInternal{
					{Name: "Go", Level: 5},
					{Name: "Python", Level: 4},
					{Name: "Kubernetes", Level: 3},
				},
				Settings: ProfileSettingsInternal{Theme: "dark"},
			},
		},
		2: {
			ID: 2, FullName: "Bob Smith", Email: "bob@example.com", Phone: "+1-555-0200", Status: "active",
			Profile: &UserProfileInternal{
				Bio: "Full-stack developer and DevOps enthusiast",
				Skills: []SkillInternal{
					{Name: "JavaScript", Level: 5},
					{Name: "React", Level: 4},
				},
				Settings: ProfileSettingsInternal{Theme: "light"},
			},
		},
	}
	products = map[int]ProductInternal{
		1: {ID: 1, Name: "Laptop", Price: 999.99, Description: "High-performance laptop", Currency: "USD"},
		2: {ID: 2, Name: "Mouse", Price: 29.99, Description: "Wireless mouse", Currency: "USD"},
	}
	orders = map[int]OrderInternal{
		1: {ID: 1, UserID: 1, ProductID: 1, Quantity: 1, Total: 999.99, CreatedAt: time.Now().Format(time.RFC3339)},
	}
	// Sample data for nested array transformations (including 2-level nested arrays)
	examplesData = []ExampleItemInternal{
		{
			ID: 1, DisplayName: "First Example", Tags: []string{"demo", "test"}, Category: "tutorial", Priority: 1,
			SubItems: []SubItemInternal{
				{ID: 101, Label: "Step 1: Setup"},
				{ID: 102, Label: "Step 2: Configure"},
			},
		},
		{
			ID: 2, DisplayName: "Second Example", Tags: []string{"advanced", "api"}, Category: "documentation", Priority: 2,
			SubItems: []SubItemInternal{
				{ID: 201, Label: "API Overview"},
				{ID: 202, Label: "Authentication"},
				{ID: 203, Label: "Endpoints"},
			},
		},
		{
			ID: 3, DisplayName: "Third Example", Tags: []string{"nested", "arrays"}, Category: "testing", Priority: 3,
			SubItems: []SubItemInternal{
				{ID: 301, Label: "Unit Tests"},
			},
		},
	}
	examplesMeta = ExampleMetaInternal{
		CreatedBy:   "system",
		LastUpdated: time.Now().Format(time.RFC3339),
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
			// User v1->v2: Top-level fields + nested type transformations (separate for each type)
			createUserV1ToV2Migration(v1, v2),
			createProfileV1ToV2Migration(v1, v2),
			createSkillV1ToV2Migration(v1, v2),
			createProfileSettingsV1ToV2Migration(v1, v2),
			// User v2->v3
			createUserV2ToV3Migration(v2, v3),
			// Product v2->v3
			createProductV2ToV3Migration(v2, v3),
			// Example migrations
			createExampleV1ToV2Migration(v1, v2),
			createExampleV2ToV3Migration(v2, v3),
		).
		WithTypes(
			// User types (including nested types for profile, skills, settings)
			CreateUserRequest{}, UpdateUserRequest{}, UserResponse{}, UsersListResponse{},
			UserProfile{}, Skill{}, ProfileSettings{},
			// Request nested types (for request nested transformations)
			ProfileRequest{}, SkillRequest{}, ProfileSettingsRequest{},
			// Product types
			CreateProductRequest{}, ProductResponse{}, ProductsListResponse{},
			// Order types
			CreateOrderRequest{}, OrderResponse{}, OrdersListResponse{},
			// Example types (including nested SubItem for 2-level array nesting)
			ExamplesPaginated{}, ExampleItemResponse{}, ExampleMetaResponse{}, SubItem{},
		).
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

	// User endpoints with type registration
	// Nested types are automatically discovered from struct definitions
	userRoutes := r.Group("/users")
	{
		userRoutes.GET("", epochInstance.WrapHandler(listUsers).
			Returns(UsersListResponse{}).
			ToHandlerFunc("GET", "/users"))
		userRoutes.GET("/:id", epochInstance.WrapHandler(getUser).
			Returns(UserResponse{}).
			ToHandlerFunc("GET", "/users/:id"))
		userRoutes.POST("", epochInstance.WrapHandler(createUser).
			Accepts(CreateUserRequest{}).
			Returns(UserResponse{}).
			ToHandlerFunc("POST", "/users"))
		userRoutes.PUT("/:id", epochInstance.WrapHandler(updateUser).
			Accepts(UpdateUserRequest{}).
			Returns(UserResponse{}).
			ToHandlerFunc("PUT", "/users/:id"))
		userRoutes.DELETE("/:id", epochInstance.WrapHandler(deleteUser).
			ToHandlerFunc("DELETE", "/users/:id"))
	}

	// Product endpoints with type registration
	productRoutes := r.Group("/products")
	{
		productRoutes.GET("", epochInstance.WrapHandler(listProducts).
			Returns(ProductsListResponse{}).
			ToHandlerFunc("GET", "/products"))
		productRoutes.GET("/:id", epochInstance.WrapHandler(getProduct).
			Returns(ProductResponse{}).
			ToHandlerFunc("GET", "/products/:id"))
		productRoutes.POST("", epochInstance.WrapHandler(createProduct).
			Accepts(CreateProductRequest{}).
			Returns(ProductResponse{}).
			ToHandlerFunc("POST", "/products"))
	}

	// Order endpoints with type registration
	orderRoutes := r.Group("/orders")
	{
		orderRoutes.GET("", epochInstance.WrapHandler(listOrders).
			Returns(OrdersListResponse{}).
			ToHandlerFunc("GET", "/orders"))
		orderRoutes.POST("", epochInstance.WrapHandler(createOrder).
			Accepts(CreateOrderRequest{}).
			Returns(OrderResponse{}).
			ToHandlerFunc("POST", "/orders"))
	}

	// Examples endpoint with nested array AND nested object type registration
	// Nested types (examples[] array, metadata object) are automatically discovered
	r.GET("/examples", epochInstance.WrapHandler(listExamples).
		Returns(ExamplesPaginated{}).
		ToHandlerFunc("GET", "/examples"))

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

	fmt.Println("🚀 Advanced Epoch Example - Nested Transformation Demo")
	fmt.Println("==========================================================")
	fmt.Println("📅 API Versions:")
	fmt.Println("  • 2024-01-01 (v1): Initial release")
	fmt.Println("      - Users: id, name (no email/status/phone)")
	fmt.Println("      - Profile: biography, skills[].skill_name (no level), settings.color_theme")
	fmt.Println("      - Examples: name, sub_items[].name")
	fmt.Println("  • 2024-06-01 (v2): Added fields and renamed nested fields")
	fmt.Println("      - Users: + email, status; profile.bio, skills[].name + level, settings.theme")
	fmt.Println("      - Examples: title (renamed), + category, sub_items[].label")
	fmt.Println("  • 2025-01-01 (v3): Further renames")
	fmt.Println("      - Users: full_name (renamed), + phone")
	fmt.Println("      - Examples: display_name (renamed), + priority")
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
	fmt.Println("  GET    /examples       - List examples")
	fmt.Println("")
	fmt.Println("💡 Comprehensive Test Commands:")
	fmt.Println("")
	fmt.Println("🔍 1. VERSION DETECTION & METADATA")
	fmt.Println("  curl http://localhost:8090/versions")
	fmt.Println("  # Expected: {\"head_version\":\"head\",\"versions\":[\"2024-01-01\",\"2024-06-01\",\"2025-01-01\",\"head\"]}")
	fmt.Println("")
	fmt.Println("👤 2. USER RESPONSE MIGRATIONS (Field Transformations)")
	fmt.Println("  # V1 (2024-01-01): id + name + nested profile with v1 field names")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8090/users/1")
	fmt.Println("  # Expected: {\"id\":1,\"name\":\"Alice Johnson\",\"profile\":{\"biography\":\"...\",")
	fmt.Println("  #           \"skills\":[{\"skill_name\":\"Go\"},...],\"settings\":{\"color_theme\":\"dark\"}}}")
	fmt.Println("")
	fmt.Println("  # V2 (2024-06-01): id + name + email + status + nested profile with v2 field names")
	fmt.Println("  curl -H 'X-API-Version: 2024-06-01' http://localhost:8090/users/1")
	fmt.Println("  # Expected: {\"id\":1,\"name\":\"Alice Johnson\",\"email\":\"...\",\"status\":\"active\",")
	fmt.Println("  #           \"profile\":{\"bio\":\"...\",\"skills\":[{\"name\":\"Go\",\"level\":5},...],\"settings\":{\"theme\":\"dark\"}}}")
	fmt.Println("")
	fmt.Println("  # V3 (2025-01-01): All fields with full_name + nested profile")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8090/users/1")
	fmt.Println("  # Expected: {\"id\":1,\"full_name\":\"Alice Johnson\",\"email\":\"...\",\"phone\":\"...\",\"status\":\"active\",")
	fmt.Println("  #           \"profile\":{\"bio\":\"...\",\"skills\":[{\"name\":\"Go\",\"level\":5},...],\"settings\":{\"theme\":\"dark\"}}}")
	fmt.Println("")
	fmt.Println("📝 3. REQUEST MIGRATIONS (Field Transformations)")
	fmt.Println("  # V2 POST: Use 'name' field (migrated to 'full_name' internally)")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-06-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"name\":\"Test User\",\"email\":\"test@example.com\",\"status\":\"active\"}' \\")
	fmt.Println("    http://localhost:8090/users")
	fmt.Println("  # Expected: {\"id\":N,\"name\":\"Test User\",\"email\":\"test@example.com\",\"status\":\"active\"}")
	fmt.Println("")
	fmt.Println("📦 4. PRODUCT MIGRATIONS (AddField Operations)")
	fmt.Println("  # V1: Only basic fields")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8090/products/1")
	fmt.Println("  # Expected: {\"id\":1,\"name\":\"Laptop\",\"price\":999.99}")
	fmt.Println("")
	fmt.Println("  # V3: With added description + currency fields")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8090/products/1")
	fmt.Println("  # Expected: {\"id\":1,\"name\":\"Laptop\",\"price\":999.99,\"description\":\"High-performance laptop\",\"currency\":\"USD\"}")
	fmt.Println("")
	fmt.Println("⚠️  5. ERROR MESSAGE FIELD NAME TRANSFORMATION")
	fmt.Println("  Validation errors show field names matching the client's API version")
	fmt.Println("")
	fmt.Println("  # V1 API - Missing required 'name' field")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-01-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{}' http://localhost:8090/users")
	fmt.Println("  # Expected: Error mentions 'Name' field (v1 field name)")
	fmt.Println("")
	fmt.Println("  # V2 API - Missing required 'name' field")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-06-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{}' http://localhost:8090/users")
	fmt.Println("  # Expected: Error mentions 'Name' field (v2 still uses 'name', not 'full_name')")
	fmt.Println("")
	fmt.Println("  # V3 API - Missing required 'full_name' field")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2025-01-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{}' http://localhost:8090/users")
	fmt.Println("  # Expected: Error mentions 'FullName' field (v3 HEAD version)")
	fmt.Println("")
	fmt.Println("📊 6. LIST ENDPOINTS (Array Transformations)")
	fmt.Println("  # V1 user list: Each user has id + name + nested profile (v1 field names)")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8090/users")
	fmt.Println("  # Expected: {\"users\":[{\"id\":1,\"name\":\"Alice Johnson\",\"profile\":{...}},...]}")
	fmt.Println("")
	fmt.Println("  # V3 user list: Each user has all fields including full_name + nested profile")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8090/users")
	fmt.Println("  # Expected: {\"users\":[{\"id\":1,\"full_name\":\"Alice Johnson\",...,\"profile\":{...}},...]}]")
	fmt.Println("")
	fmt.Println("🎯 7. ADVANCED SCENARIOS")
	fmt.Println("  # Default version (no header): Uses HEAD version")
	fmt.Println("  curl http://localhost:8090/users/1")
	fmt.Println("")
	fmt.Println("  # Health check (unversioned endpoint)")
	fmt.Println("  curl http://localhost:8090/health")
	fmt.Println("")
	fmt.Println("🔧 8. NESTED ARRAY TRANSFORMATIONS (examples[])")
	fmt.Println("  # V1: Examples with 'name' field, sub_items with 'name'")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8090/examples")
	fmt.Println("  # Expected: {\"examples\":[{\"id\":1,\"name\":\"First Example\",\"tags\":[...],")
	fmt.Println("  #           \"sub_items\":[{\"id\":101,\"name\":\"Step 1: Setup\"},...]},...]}")
	fmt.Println("")
	fmt.Println("  # V2: Examples with 'title' + category, sub_items with 'label'")
	fmt.Println("  curl -H 'X-API-Version: 2024-06-01' http://localhost:8090/examples")
	fmt.Println("  # Expected: {\"examples\":[{\"id\":1,\"title\":\"First Example\",\"category\":\"tutorial\",")
	fmt.Println("  #           \"sub_items\":[{\"id\":101,\"label\":\"Step 1: Setup\"},...]},...]}")
	fmt.Println("")
	fmt.Println("  # V3/HEAD: Examples with 'display_name' + priority, sub_items with 'label'")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8090/examples")
	fmt.Println("  # Expected: {\"examples\":[{\"id\":1,\"display_name\":\"First Example\",\"priority\":1,")
	fmt.Println("  #           \"sub_items\":[{\"id\":101,\"label\":\"Step 1: Setup\"},...]},...]}")
	fmt.Println("")
	fmt.Println("🏗️  9. DEEPLY NESTED OBJECTS (user.profile.settings - 3 levels)")
	fmt.Println("  # V1: profile.biography, profile.skills[].skill_name (no level), profile.settings.color_theme")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8090/users/1 | jq '.profile'")
	fmt.Println("  # Expected: {\"biography\":\"...\",\"skills\":[{\"skill_name\":\"Go\"},...],\"settings\":{\"color_theme\":\"dark\"}}")
	fmt.Println("")
	fmt.Println("  # V2+: profile.bio, profile.skills[].name + level, profile.settings.theme")
	fmt.Println("  curl -H 'X-API-Version: 2024-06-01' http://localhost:8090/users/1 | jq '.profile'")
	fmt.Println("  # Expected: {\"bio\":\"...\",\"skills\":[{\"name\":\"Go\",\"level\":5},...],\"settings\":{\"theme\":\"dark\"}}")
	fmt.Println("")
	fmt.Println("🔄 10. ARRAYS INSIDE NESTED OBJECTS (user.profile.skills[])")
	fmt.Println("  # V1: Skills array inside profile with 'skill_name', no 'level'")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8090/users/1 | jq '.profile.skills'")
	fmt.Println("  # Expected: [{\"skill_name\":\"Go\"},{\"skill_name\":\"Python\"},{\"skill_name\":\"Kubernetes\"}]")
	fmt.Println("")
	fmt.Println("  # V2+/HEAD: Skills array with 'name' and 'level'")
	fmt.Println("  curl -H 'X-API-Version: 2025-01-01' http://localhost:8090/users/1 | jq '.profile.skills'")
	fmt.Println("  # Expected: [{\"name\":\"Go\",\"level\":5},{\"name\":\"Python\",\"level\":4},{\"name\":\"Kubernetes\",\"level\":3}]")
	fmt.Println("")
	fmt.Println("📚 11. 2-LEVEL NESTED ARRAYS (examples[].sub_items[])")
	fmt.Println("  # V1: sub_items[].name (transformed from label)")
	fmt.Println("  curl -H 'X-API-Version: 2024-01-01' http://localhost:8090/examples | jq '.examples[0].sub_items'")
	fmt.Println("  # Expected: [{\"id\":101,\"name\":\"Step 1: Setup\"},{\"id\":102,\"name\":\"Step 2: Configure\"}]")
	fmt.Println("")
	fmt.Println("  # V2+: sub_items[].label (HEAD field name)")
	fmt.Println("  curl -H 'X-API-Version: 2024-06-01' http://localhost:8090/examples | jq '.examples[0].sub_items'")
	fmt.Println("  # Expected: [{\"id\":101,\"label\":\"Step 1: Setup\"},{\"id\":102,\"label\":\"Step 2: Configure\"}]")
	fmt.Println("")
	fmt.Println("📥 12. REQUEST NESTED OBJECT TRANSFORMATIONS (profile.biography -> profile.bio)")
	fmt.Println("  # V1: POST with nested profile using 'biography' (old field name)")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-01-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"name\":\"New User\",\"profile\":{\"biography\":\"A great developer\"}}' \\")
	fmt.Println("    http://localhost:8090/users")
	fmt.Println("  # Request transformation: biography -> bio (stored internally as bio)")
	fmt.Println("  # Response transformation: bio -> biography (returned to V1 client)")
	fmt.Println("  # Expected response: {\"id\":N,\"name\":\"New User\",\"profile\":{\"biography\":\"A great developer\",...}}")
	fmt.Println("")
	fmt.Println("📥 13. REQUEST NESTED ARRAY TRANSFORMATIONS (profile.skills[].skill_name -> name)")
	fmt.Println("  # V1: POST with nested skills array using 'skill_name' (old field name)")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-01-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"name\":\"Skilled User\",\"profile\":{\"biography\":\"Expert\",\"skills\":[{\"skill_name\":\"Go\"},{\"skill_name\":\"Python\"}]}}' \\")
	fmt.Println("    http://localhost:8090/users")
	fmt.Println("  # Request transformation: skill_name -> name, level added with default 1")
	fmt.Println("  # Response transformation: name -> skill_name, level removed")
	fmt.Println("  # Expected response: {...,\"profile\":{\"biography\":\"Expert\",\"skills\":[{\"skill_name\":\"Go\"},{\"skill_name\":\"Python\"}],...}}")
	fmt.Println("")
	fmt.Println("📥 14. REQUEST DEEPLY NESTED TRANSFORMATIONS (profile.settings.color_theme -> theme)")
	fmt.Println("  # V1: POST with deeply nested settings using 'color_theme' (old field name)")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-01-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"name\":\"Theme User\",\"profile\":{\"biography\":\"Designer\",\"settings\":{\"color_theme\":\"dark\"}}}' \\")
	fmt.Println("    http://localhost:8090/users")
	fmt.Println("  # Request transformation: color_theme -> theme (stored as 'theme')")
	fmt.Println("  # Response transformation: theme -> color_theme (returned to V1 client)")
	fmt.Println("  # Expected response: {...,\"profile\":{\"biography\":\"...\",\"settings\":{\"color_theme\":\"dark\"},...}}")
	fmt.Println("")
	fmt.Println("📥 15. COMPLETE REQUEST NESTED TRANSFORMATION TEST")
	fmt.Println("  # V1: POST with ALL nested structures (object, array, deeply nested)")
	fmt.Println("  curl -X POST -H 'X-API-Version: 2024-01-01' -H 'Content-Type: application/json' \\")
	fmt.Println("    -d '{\"name\":\"Full User\",\"profile\":{\"biography\":\"Full stack dev\",\"skills\":[{\"skill_name\":\"JavaScript\"},{\"skill_name\":\"React\"}],\"settings\":{\"color_theme\":\"light\"}}}' \\")
	fmt.Println("    http://localhost:8090/users")
	fmt.Println("  # Request transformations applied:")
	fmt.Println("  #   - name stays as name (V1 field name)")
	fmt.Println("  #   - biography -> bio")
	fmt.Println("  #   - skill_name -> name, level added (default: 1)")
	fmt.Println("  #   - color_theme -> theme")
	fmt.Println("  # Response transformations (back to V1):")
	fmt.Println("  #   - bio -> biography")
	fmt.Println("  #   - name -> skill_name, level removed")
	fmt.Println("  #   - theme -> color_theme")
	fmt.Println("  # Expected: {\"id\":N,\"name\":\"Full User\",\"profile\":{\"biography\":\"Full stack dev\",")
	fmt.Println("  #           \"skills\":[{\"skill_name\":\"JavaScript\"},{\"skill_name\":\"React\"}],")
	fmt.Println("  #           \"settings\":{\"color_theme\":\"light\"}}}")
	fmt.Println("")
	fmt.Println("🌐 Server listening on http://localhost:8090")
	fmt.Println("   Use X-API-Version header to specify version")
	fmt.Println("")

	r.Run(":8090")
}

// ============================================================================
// DECLARATIVE MIGRATIONS - Simple & Clean! ✨
// ============================================================================

// NOTE: Field operations apply to ALL types in ForType(). To avoid unintended
// field renames across different types, we create SEPARATE version changes
// for each type group with distinct field names.

// createUserV1ToV2Migration defines migrations for TOP-LEVEL user fields only
// Nested types (Profile, Skill, Settings) have their own separate migrations
func createUserV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Add email and status fields for v1->v2").
		// Only target top-level user types (NOT nested types - they have separate migrations)
		ForType(UserResponse{}, CreateUserRequest{}, UpdateUserRequest{}).
		// Requests: Client→HEAD (add defaults for old clients)
		RequestToNextVersion().
		AddField("email", "unknown@example.com"). // Add email with default for v1 clients
		AddField("status", "active").             // Add status with default for v1 clients
		RemoveField("temp_field").                // Remove deprecated field
		// Responses: HEAD→Client (remove new fields for old clients)
		ResponseToPreviousVersion().
		RemoveField("email").  // Remove email from responses for v1 clients
		RemoveField("status"). // Remove status from responses for v1 clients
		Build()
}

// createProfileV1ToV2Migration handles nested Profile object transformations
func createProfileV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Transform profile.biography -> profile.bio").
		// Target ONLY Profile types (response and request)
		ForType(UserProfile{}, ProfileRequest{}).
		// Requests: biography -> bio
		RequestToNextVersion().
		RenameField("biography", "bio").
		// Responses: bio -> biography
		ResponseToPreviousVersion().
		RenameField("bio", "biography").
		Build()
}

// createSkillV1ToV2Migration handles nested Skill array item transformations
func createSkillV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Transform skills[].skill_name -> skills[].name, add level").
		// Target ONLY Skill types (response and request)
		ForType(Skill{}, SkillRequest{}).
		// Requests: skill_name -> name, add level
		RequestToNextVersion().
		RenameField("skill_name", "name").
		AddField("level", 1).
		// Responses: name -> skill_name, remove level
		ResponseToPreviousVersion().
		RenameField("name", "skill_name").
		RemoveField("level").
		Build()
}

// createProfileSettingsV1ToV2Migration handles deeply nested Settings transformations
func createProfileSettingsV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Transform settings.color_theme -> settings.theme").
		// Target ONLY ProfileSettings types (response and request)
		ForType(ProfileSettings{}, ProfileSettingsRequest{}).
		// Requests: color_theme -> theme
		RequestToNextVersion().
		RenameField("color_theme", "theme").
		// Responses: theme -> color_theme
		ResponseToPreviousVersion().
		RenameField("theme", "color_theme").
		Build()
}

// createUserV2ToV3Migration defines the migration from v2 to v3
func createUserV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Rename name to full_name, add phone").
		// Only target top-level user types
		ForType(UserResponse{}, CreateUserRequest{}, UpdateUserRequest{}).
		// Requests: Client→HEAD (rename old field name to new, add defaults)
		RequestToNextVersion().
		RenameField("name", "full_name"). // Rename from old to new field name
		AddField("phone", "").            // Add phone field with default
		// Responses: HEAD→Client (rename new field name back to old, remove new fields)
		ResponseToPreviousVersion().
		RenameField("full_name", "name"). // Rename back to old field name
		RemoveField("phone").             // Remove phone from responses for v2 clients
		Build()
}

// createProductV2ToV3Migration defines the migration from v2 to v3 for products
// This uses the NEW flow-based API with only 2 directions
func createProductV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Add description and currency to Product").
		// TYPE-BASED ROUTING: Target all Product-related request/response types
		ForType(CreateProductRequest{}, ProductResponse{}, ProductsListResponse{}).
		// Requests: Client→HEAD (add defaults for old clients)
		RequestToNextVersion().
		AddField("description", ""). // Add description field with default
		AddField("currency", "USD"). // Add currency field with default
		// Responses: HEAD→Client (remove new fields for old clients)
		ResponseToPreviousVersion().
		RemoveField("description"). // Remove description from responses for v2 clients
		RemoveField("currency").    // Remove currency from responses for v2 clients
		Build()
}

// createExampleV1ToV2Migration
// This migration affects the ExampleItem structs inside the Examples array
// Also demonstrates 2-level nested array transformation (examples[].sub_items[])
func createExampleV1ToV2Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Rename name to title in nested array items, add category field, transform sub_items").
		// TYPE-BASED ROUTING: Target the container, nested item types, and sub-item types
		ForType(ExamplesPaginated{}, ExampleItemResponse{}, SubItem{}).
		// Responses: HEAD→Client (rename new field back to old, remove new fields)
		ResponseToPreviousVersion().
		RenameField("title", "name"). // Rename back to old field name in nested items
		RemoveField("category").      // Remove category from nested items for v1 clients
		RenameField("label", "name"). // sub_items[].label -> sub_items[].name for v1 clients
		Build()
}

// createExampleV2ToV3Migration
func createExampleV2ToV3Migration(from, to *epoch.Version) *epoch.VersionChange {
	return epoch.NewVersionChangeBuilder(from, to).
		Description("Rename title to display_name in nested array items, add priority field, rename updated_at to last_updated in metadata").
		// TYPE-BASED ROUTING: Target the container, nested item response, metadata response, and sub-item types
		ForType(ExamplesPaginated{}, ExampleItemResponse{}, ExampleMetaResponse{}, SubItem{}).
		// Responses: HEAD→Client (rename new fields back to old, remove new fields)
		ResponseToPreviousVersion().
		RenameField("display_name", "title").      // Rename back to old field name in nested items
		RemoveField("priority").                   // Remove priority from nested items for v2 clients
		RenameField("last_updated", "updated_at"). // Rename back in nested metadata object
		Build()
}

// ============================================================================
// HTTP Handlers
// ============================================================================

func listUsers(c *gin.Context) {
	// Convert internal storage to list of internal models
	userList := make([]UserInternal, 0, len(users))
	for _, user := range users {
		userList = append(userList, user)
	}
	// Always return HEAD version response
	// Epoch middleware will transform it to the client's requested version
	c.JSON(http.StatusOK, NewUsersListResponse(userList))
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
	// Always return HEAD version response
	// Epoch middleware handles the transformation
	c.JSON(http.StatusOK, NewUserResponse(user))
}

func createUser(c *gin.Context) {
	// Always bind to HEAD version request struct
	var req CreateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to internal model (including nested profile if provided)
	internal := UserInternal{
		ID:       nextUserID,
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Status:   req.Status,
	}

	// Handle nested profile from request (demonstrates request nested transformation)
	if req.Profile != nil {
		skills := make([]SkillInternal, len(req.Profile.Skills))
		for i, s := range req.Profile.Skills {
			skills[i] = SkillInternal{Name: s.Name, Level: s.Level}
		}
		internal.Profile = &UserProfileInternal{
			Bio:    req.Profile.Bio,
			Skills: skills,
		}
		if req.Profile.Settings != nil {
			internal.Profile.Settings = ProfileSettingsInternal{Theme: req.Profile.Settings.Theme}
		}
	}

	nextUserID++
	users[internal.ID] = internal

	// Always return HEAD version response struct
	// Epoch middleware will transform it to the client's requested version
	c.JSON(http.StatusCreated, NewUserResponse(internal))
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

	// Always bind to HEAD version request struct
	var req UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to internal model (including nested profile if provided)
	internal := UserInternal{
		ID:       userID,
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Status:   req.Status,
	}

	// Handle nested profile from request (demonstrates request nested transformation)
	if req.Profile != nil {
		skills := make([]SkillInternal, len(req.Profile.Skills))
		for i, s := range req.Profile.Skills {
			skills[i] = SkillInternal{Name: s.Name, Level: s.Level}
		}
		internal.Profile = &UserProfileInternal{
			Bio:    req.Profile.Bio,
			Skills: skills,
		}
		if req.Profile.Settings != nil {
			internal.Profile.Settings = ProfileSettingsInternal{Theme: req.Profile.Settings.Theme}
		}
	}

	users[userID] = internal

	// Always return HEAD version response
	c.JSON(http.StatusOK, NewUserResponse(internal))
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
	// Convert internal storage to list of internal models
	productList := make([]ProductInternal, 0, len(products))
	for _, product := range products {
		productList = append(productList, product)
	}
	// Always return HEAD version response
	c.JSON(http.StatusOK, NewProductsListResponse(productList))
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
	// Always return HEAD version response
	c.JSON(http.StatusOK, NewProductResponse(product))
}

func createProduct(c *gin.Context) {
	// Always bind to HEAD version request struct
	var req CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Convert to internal model
	internal := ProductInternal{
		ID:          nextProductID,
		Name:        req.Name,
		Price:       req.Price,
		Description: req.Description,
		Currency:    req.Currency,
	}
	nextProductID++
	products[internal.ID] = internal

	// Always return HEAD version response
	c.JSON(http.StatusCreated, NewProductResponse(internal))
}

func listOrders(c *gin.Context) {
	// Convert internal storage to list of internal models
	orderList := make([]OrderInternal, 0, len(orders))
	for _, order := range orders {
		orderList = append(orderList, order)
	}
	// Always return HEAD version response
	c.JSON(http.StatusOK, NewOrdersListResponse(orderList))
}

func createOrder(c *gin.Context) {
	// Always bind to HEAD version request struct
	var req CreateOrderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Look up product to calculate total
	product, exists := products[req.ProductID]
	if !exists {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product not found"})
		return
	}

	// Convert to internal model
	internal := OrderInternal{
		ID:        nextOrderID,
		UserID:    req.UserID,
		ProductID: req.ProductID,
		Quantity:  req.Quantity,
		Total:     product.Price * float64(req.Quantity),
		CreatedAt: time.Now().Format(time.RFC3339),
	}
	nextOrderID++
	orders[internal.ID] = internal

	// Always return HEAD version response
	c.JSON(http.StatusCreated, NewOrderResponse(internal))
}

func listExamples(c *gin.Context) {
	// Always return HEAD version response
	c.JSON(http.StatusOK, NewExamplesPaginated(examplesData, examplesMeta))
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
