package main

import (
	"fmt"
	"net/http"

	"github.com/astronomer/epoch/epoch"
	"github.com/gin-gonic/gin"
)

// User represents our data model
// This is the HEAD version (latest) with all fields
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name" binding:"required"`
	Email string `json:"email" binding:"required"` // Added in v2.0.0
}

func main() {
	// Create a simple versioned API with Epoch
	// v1.0.0: User has ID and Name
	// v2.0.0: User adds Email field (with automatic bidirectional migration!)

	fmt.Println("🚀 Starting Epoch Basic Example")
	fmt.Println("")
	fmt.Println("📦 API Versions:")
	fmt.Println("  • v1.0.0: User has id, name")
	fmt.Println("  • v2.0.0: User has id, name, email")
	fmt.Println("  • HEAD: Latest version (v2.0.0)")
	fmt.Println("")

	// Build Epoch instance with automatic cycle detection
	epochInstance, err := epoch.NewEpoch().
		WithSemverVersions("1.0.0", "2.0.0").
		WithHeadVersion().
		WithChanges(createV1ToV2Change()).
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to build Epoch: %v", err))
	}

	fmt.Println("✅ Epoch configured successfully (with cycle detection)")
	fmt.Println("")

	// Setup Gin with Epoch middleware
	r := gin.Default()

	// Add Epoch version detection middleware
	r.Use(epochInstance.Middleware())

	// Define routes with version-aware handlers
	r.GET("/users/:id", epochInstance.WrapHandler(getUser))
	r.POST("/users", epochInstance.WrapHandler(createUser))

	// Print usage instructions
	fmt.Println("💡 Try these commands:")
	fmt.Println("")
	fmt.Println("# Get user (v1.0.0 - no email in response)")
	fmt.Println("curl http://localhost:8080/users/1 -H 'X-API-Version: 1.0.0'")
	fmt.Println("")
	fmt.Println("# Get user (v2.0.0 - includes email)")
	fmt.Println("curl http://localhost:8080/users/1 -H 'X-API-Version: 2.0.0'")
	fmt.Println("")
	fmt.Println("# Create user (v1.0.0 - no email required)")
	fmt.Println("curl -X POST http://localhost:8080/users -H 'X-API-Version: 1.0.0' -H 'Content-Type: application/json' -d '{\"name\":\"Jane\"}'")
	fmt.Println("")
	fmt.Println("# Create user (v2.0.0 - email required)")
	fmt.Println("curl -X POST http://localhost:8080/users -H 'X-API-Version: 2.0.0' -H 'Content-Type: application/json' -d '{\"name\":\"Jane\",\"email\":\"jane@example.com\"}'")
	fmt.Println("")
	fmt.Println("🌐 Server listening on http://localhost:8080")
	fmt.Println("")

	// Run server
	r.Run(":8080")
}

// createV1ToV2Change defines the migration between v1.0.0 and v2.0.0
// This uses the NEW declarative API which automatically generates:
//  1. Request migration (v1 → v2): adds "email" field if missing
//  2. Response migration (v2 → v1): removes "email" field
//  3. Error transformation: updates field names in error messages
func createV1ToV2Change() *epoch.VersionChange {
	v1, _ := epoch.NewSemverVersion("1.0.0")
	v2, _ := epoch.NewSemverVersion("2.0.0")

	return epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email field to User").
		// PATH-BASED ROUTING: Explicitly specify which endpoints this migration affects
		ForPath("/users", "/users/:id").
		// ✨ One line → Automatic bidirectional migration!
		AddField("email", "default@example.com").
		Build()
}

// getUser returns a user (HEAD version)
func getUser(c *gin.Context) {
	// Always implement your handler for the HEAD (latest) version
	user := User{
		ID:    1,
		Name:  "John Doe",
		Email: "john@example.com",
	}
	c.JSON(http.StatusOK, user)
}

// createUser creates a user (HEAD version)
func createUser(c *gin.Context) {
	var user User
	if err := c.ShouldBindJSON(&user); err != nil {
		// Error messages will automatically transform field names for each version!
		// For v1.0.0: "Email" field names won't appear (field was added in v2.0.0)
		// For v2.0.0: Shows "Email" as expected
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In a real app, save to database here
	user.ID = 1

	c.JSON(http.StatusCreated, user)
}
