package main

import (
	"net/http"

	"github.com/astronomer/epoch/epoch"
	"github.com/gin-gonic/gin"
)

// User represents our data model
// This is the HEAD version (latest) with all fields
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"` // Added in v2.0.0
}

func main() {
	// Create a simple versioned API with Epoch
	// v1.0.0: User has ID and Name
	// v2.0.0: User adds Email field
	epochInstance, err := epoch.NewEpoch().
		WithSemverVersions("1.0.0", "2.0.0").
		WithHeadVersion().
		WithChanges(createV1ToV2Change()).
		Build()

	if err != nil {
		panic(err)
	}

	// Setup Gin with Epoch middleware
	r := gin.Default()

	// Add Epoch version detection middleware
	r.Use(epochInstance.Middleware())

	// Define routes with version-aware handlers
	r.GET("/users/:id", epochInstance.WrapHandler(getUser))
	r.POST("/users", epochInstance.WrapHandler(createUser))

	// Run server
	r.Run(":8080")
}

// createV1ToV2Change defines the migration between v1.0.0 and v2.0.0
func createV1ToV2Change() *epoch.VersionChange {
	v1, _ := epoch.NewSemverVersion("1.0.0")
	v2, _ := epoch.NewSemverVersion("2.0.0")

	return epoch.NewVersionChange(
		"Add email field to User",
		v1,
		v2,
		// Request migration: v1 -> v2 (add email if missing)
		&epoch.AlterRequestInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(req *epoch.RequestInfo) error {
				if !req.HasField("email") {
					req.SetField("email", "default@example.com")
				}
				return nil
			},
		},
		// Response migration: v2 -> v1 (remove email)
		&epoch.AlterResponseInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(resp *epoch.ResponseInfo) error {
				resp.DeleteField("email")
				return nil
			},
		},
	)
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
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// In a real app, save to database here
	user.ID = 1

	c.JSON(http.StatusCreated, user)
}

/*
Try it out:

# v2.0.0 request (includes email)
curl -X POST http://localhost:8080/users \
  -H "X-API-Version: 2.0.0" \
  -H "Content-Type: application/json" \
  -d '{"name":"Jane","email":"jane@example.com"}'

# v1.0.0 request (no email in response)
curl http://localhost:8080/users/1 \
  -H "X-API-Version: 1.0.0"

# HEAD request (latest, includes email)
curl http://localhost:8080/users/1 \
  -H "X-API-Version: head"
*/
