package main

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/isaacchung/cadwyn-go/cadwyn"
)

// User models for different versions
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email,omitempty"`
	Phone string `json:"phone,omitempty"` // Added in v2.0
}

// In-memory user storage
var users = []User{
	{ID: 1, Name: "John Doe", Email: "john@example.com", Phone: "+1234567890"},
	{ID: 2, Name: "Jane Smith", Email: "jane@example.com", Phone: "+0987654321"},
}
var nextID = 3

func main() {
	gin.SetMode(gin.ReleaseMode)

	// Create versions
	v1, _ := cadwyn.NewVersion("1.0") // No phone field
	v2, _ := cadwyn.NewVersion("2.0") // Added phone field
	head := cadwyn.NewHeadVersion()

	// Create version changes
	v1ToV2Change := createV1ToV2Change(v1, v2)
	v2.Changes = []cadwyn.VersionChangeInterface{v1ToV2Change}

	// Create Cadwyn application
	app, err := cadwyn.NewBuilder().
		WithVersions(v1, v2, head).
		WithGinServer(true).
		WithGinMode("release").
		WithVersionLocation(cadwyn.VersionLocationHeader).
		WithVersionParameter("X-API-Version").
		WithTitle("User API").
		WithDescription("A versioned user management API built with Gin").
		WithSchemaGeneration(true).
		WithChangelog(true).
		WithDebugLogging(true).
		Build()

	if err != nil {
		panic("Failed to create Cadwyn application: " + err.Error())
	}

	// Get the Gin application
	ginApp := app.GetGinApp()
	if ginApp == nil {
		panic("Gin application not enabled")
	}

	// Setup API routes
	setupRoutes(ginApp)

	// Start server
	println("🚀 Cadwyn-Go Gin Server Example")
	println("🌐 Starting server on :8080")
	println("📚 Try these endpoints:")
	println("   • GET  http://localhost:8080/users")
	println("   • GET  http://localhost:8080/users/1")
	println("   • POST http://localhost:8080/users")
	println("   • GET  http://localhost:8080/docs")
	println("   • GET  http://localhost:8080/versions")
	println("   • GET  http://localhost:8080/changelog")
	println("")
	println("🔧 Version Headers:")
	println("   • X-API-Version: 1.0  (no phone field)")
	println("   • X-API-Version: 2.0  (with phone field)")
	println("   • (no header)         (latest version)")

	if err := ginApp.Run(":8080"); err != nil {
		panic("Server failed: " + err.Error())
	}
}

func setupRoutes(app *cadwyn.Application) {
	// GET /users - List all users
	app.GET("/users", func(c *gin.Context) {
		// Get version from context
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
	})

	// GET /users/:id - Get specific user
	app.GET("/users/:id", func(c *gin.Context) {
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
	})

	// POST /users - Create new user
	app.POST("/users", func(c *gin.Context) {
		var newUser User
		if err := c.ShouldBindJSON(&newUser); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid JSON: " + err.Error()})
			return
		}

		// Get version from context
		requestedVersion := cadwyn.GetVersionFromContext(c)
		versionStr := "head"
		if requestedVersion != nil {
			versionStr = requestedVersion.String()
		}

		// Apply version-specific logic
		if versionStr == "1.0" {
			// v1.0 doesn't support phone field
			newUser.Phone = ""
		}

		// Assign ID and add to storage
		newUser.ID = nextID
		nextID++
		users = append(users, newUser)

		// Transform response based on version
		transformedUser := transformUserForVersion(newUser, versionStr)

		c.JSON(http.StatusCreated, transformedUser)
	})
}

// createV1ToV2Change creates the version change that adds phone field
func createV1ToV2Change(from, to *cadwyn.Version) *UserVersionChange {
	requestInstruction := &cadwyn.AlterRequestInstruction{
		Schemas: []interface{}{User{}},
		Transformer: func(requestInfo *cadwyn.RequestInfo) error {
			// This would be used for automatic request migration
			// For now, we handle it manually in the handlers
			return nil
		},
	}

	responseInstruction := &cadwyn.AlterResponseInstruction{
		Schemas: []interface{}{User{}},
		Transformer: func(responseInfo *cadwyn.ResponseInfo) error {
			// This would be used for automatic response migration
			// For now, we handle it manually in the handlers
			return nil
		},
	}

	return &UserVersionChange{
		description:         "v1→v2: Add phone field to User",
		fromVersion:         from,
		toVersion:           to,
		requestInstruction:  requestInstruction,
		responseInstruction: responseInstruction,
	}
}

// UserVersionChange implements version change for User model
type UserVersionChange struct {
	description         string
	fromVersion         *cadwyn.Version
	toVersion           *cadwyn.Version
	requestInstruction  *cadwyn.AlterRequestInstruction
	responseInstruction *cadwyn.AlterResponseInstruction
}

func (uvc *UserVersionChange) Description() string {
	return uvc.description
}

func (uvc *UserVersionChange) FromVersion() *cadwyn.Version {
	return uvc.fromVersion
}

func (uvc *UserVersionChange) ToVersion() *cadwyn.Version {
	return uvc.toVersion
}

func (uvc *UserVersionChange) GetSchemaInstructions() interface{} {
	return []*cadwyn.SchemaInstruction{
		{
			Schema: User{},
			Name:   "phone",
			Type:   "field_added",
			Attributes: map[string]interface{}{
				"type":     "string",
				"required": false,
			},
		},
	}
}

func (uvc *UserVersionChange) GetEnumInstructions() interface{} {
	return []interface{}{}
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
	case "1.0":
		// v1.0 doesn't have phone field
		return gin.H{
			"id":    user.ID,
			"name":  user.Name,
			"email": user.Email,
		}
	case "2.0", "head":
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
