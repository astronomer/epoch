package main

import (
	"net/http"
	"strconv"

	"github.com/astronomer/cadwyn-go/cadwyn"
	"github.com/gin-gonic/gin"
)

// User models for different versions
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"` // Added in v2.0
}

type CreateUserRequest struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone,omitempty"` // Added in v2.0
}

type UserResponse struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Phone string `json:"phone,omitempty"` // Added in v2.0
}

// In-memory user storage
var users = []User{
	{ID: 1, Name: "John Doe", Email: "john@example.com", Phone: "+1234567890"},
	{ID: 2, Name: "Jane Smith", Email: "jane@example.com", Phone: "+0987654321"},
}
var nextID = 3

func main() {
	// Create your own Gin engine with your preferred settings
	r := gin.Default()

	// Add your own middleware
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// Create versions
	v1, _ := cadwyn.NewDateVersion("2023-01-01") // No phone field
	v2, _ := cadwyn.NewDateVersion("2024-01-01") // Added phone field
	head := cadwyn.NewHeadVersion()

	// Create version changes
	v1ToV2Change := createV1ToV2Change(v1, v2)

	// Create Cadwyn with just the versioning logic
	cadwynInstance, err := cadwyn.NewCadwyn().
		WithVersions(v1, v2, head).
		WithChanges(v1ToV2Change).
		WithTypes(CreateUserRequest{}, UserResponse{}).
		WithVersionLocation(cadwyn.VersionLocationHeader).
		WithVersionParameter("X-API-Version").
		Build()

	if err != nil {
		panic("Failed to create Cadwyn: " + err.Error())
	}

	// Apply Cadwyn's version detection middleware
	r.Use(cadwynInstance.Middleware())

	// Setup API routes - you choose which ones need versioning
	setupRoutes(r, cadwynInstance)

	// Add utility endpoints that don't need versioning
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "healthy"})
	})

	r.GET("/versions", func(c *gin.Context) {
		versions := cadwynInstance.GetVersions()
		versionStrings := make([]string, len(versions))
		for i, v := range versions {
			versionStrings[i] = v.String()
		}
		c.JSON(http.StatusOK, gin.H{"versions": versionStrings})
	})

	// Start server
	println("🚀 Cadwyn-Go Simple Example")
	println("🌐 Starting server on :8080")
	println("📚 Try these endpoints:")
	println("   • GET  http://localhost:8080/users")
	println("   • GET  http://localhost:8080/users/1")
	println("   • POST http://localhost:8080/users")
	println("   • GET  http://localhost:8080/health      (no versioning)")
	println("   • GET  http://localhost:8080/versions    (no versioning)")
	println("")
	println("🔧 Version Headers:")
	println("   • X-API-Version: 2023-01-01  (no phone field)")
	println("   • X-API-Version: 2024-01-01  (with phone field)")
	println("   • (no header)                (latest version)")

	if err := r.Run(":8080"); err != nil {
		panic("Server failed: " + err.Error())
	}
}

func setupRoutes(r *gin.Engine, cadwyn *cadwyn.Cadwyn) {
	// API routes that need versioning - wrap them with cadwyn.WrapHandler
	r.GET("/users", cadwyn.WrapHandler(getUsersHandler))
	r.GET("/users/:id", cadwyn.WrapHandler(getUserHandler))
	r.POST("/users", cadwyn.WrapHandler(createUserHandler))
}

// Handlers that use versioning
func getUsersHandler(c *gin.Context) {
	// Get version from context (set by Cadwyn middleware)
	requestedVersion := cadwyn.GetVersionFromContext(c)
	versionStr := "head"
	if requestedVersion != nil {
		versionStr = requestedVersion.String()
	}

	// Transform users based on version
	transformedUsers := transformUsersForVersion(users, versionStr)

	c.JSON(http.StatusOK, gin.H{
		"users":   transformedUsers,
		"version": versionStr,
		"total":   len(transformedUsers),
	})
}

func getUserHandler(c *gin.Context) {
	// Extract ID from path parameter
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid user ID"})
		return
	}

	// Find user
	var user *User
	for i := range users {
		if users[i].ID == id {
			user = &users[i]
			break
		}
	}

	if user == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Get version from context
	requestedVersion := cadwyn.GetVersionFromContext(c)
	versionStr := "head"
	if requestedVersion != nil {
		versionStr = requestedVersion.String()
	}

	// Transform user based on version
	transformedUser := transformUserForVersion(*user, versionStr)

	c.JSON(http.StatusOK, transformedUser)
}

func createUserHandler(c *gin.Context) {
	var request CreateUserRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
		return
	}

	// Get version from context
	requestedVersion := cadwyn.GetVersionFromContext(c)
	versionStr := "head"
	if requestedVersion != nil {
		versionStr = requestedVersion.String()
	}

	// Create new user
	newUser := User{
		ID:    nextID,
		Name:  request.Name,
		Email: request.Email,
		Phone: request.Phone,
	}

	// Apply version-specific logic
	if versionStr == "2023-01-01" {
		// v1.0 doesn't support phone field
		newUser.Phone = ""
	}

	nextID++
	users = append(users, newUser)

	// Transform response based on version
	transformedUser := transformUserForVersion(newUser, versionStr)

	c.JSON(http.StatusCreated, transformedUser)
}

// createV1ToV2Change creates the version change that adds phone field
func createV1ToV2Change(from, to *cadwyn.Version) *cadwyn.VersionChange {
	return cadwyn.NewVersionChange(
		"Add phone field to User",
		from,
		to,
		&cadwyn.AlterRequestInstruction{
			Schemas: []interface{}{CreateUserRequest{}},
			Transformer: func(requestInfo *cadwyn.RequestInfo) error {
				// Request migration would happen here
				return nil
			},
		},
		&cadwyn.AlterResponseInstruction{
			Schemas: []interface{}{UserResponse{}},
			Transformer: func(responseInfo *cadwyn.ResponseInfo) error {
				// Response migration would happen here
				return nil
			},
		},
		&cadwyn.SchemaInstruction{
			Schema: UserResponse{},
			Name:   "phone",
			Type:   "field_added",
			Attributes: map[string]interface{}{
				"type":     "string",
				"required": false,
			},
		},
	)
}

// Version-specific transformation functions
func transformUsersForVersion(users []User, versionStr string) []interface{} {
	result := make([]interface{}, len(users))
	for i, user := range users {
		result[i] = transformUserForVersion(user, versionStr)
	}
	return result
}

func transformUserForVersion(user User, versionStr string) interface{} {
	switch versionStr {
	case "2023-01-01":
		// v1.0 doesn't have phone field
		return gin.H{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		}
	case "2024-01-01", "head":
		// v2.0 and head have all fields
		return gin.H{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
			"phone": user.Phone,
		}
	default:
		// Unknown version, return full object
		return user
	}
}
