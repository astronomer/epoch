package main

import (
	"net/http"

	"github.com/astronomer/cadwyn-go/cadwyn"
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
	// Create a simple versioned API with Cadwyn
	// v1.0.0: User has ID and Name
	// v2.0.0: User adds Email field
	cadwynInstance, err := cadwyn.NewCadwyn().
		WithSemverVersions("1.0.0", "2.0.0").
		WithHeadVersion().
		WithChanges(createV1ToV2Change()).
		Build()

	if err != nil {
		panic(err)
	}

	// Setup Gin with Cadwyn middleware
	r := gin.Default()

	// Add Cadwyn version detection middleware
	r.Use(cadwynInstance.Middleware())

	// Define routes with version-aware handlers
	r.GET("/users/:id", cadwynInstance.WrapHandler(getUser))
	r.POST("/users", cadwynInstance.WrapHandler(createUser))

	// Run server
	r.Run(":8080")
}

// createV1ToV2Change defines the migration between v1.0.0 and v2.0.0
func createV1ToV2Change() *cadwyn.VersionChange {
	v1, _ := cadwyn.NewSemverVersion("1.0.0")
	v2, _ := cadwyn.NewSemverVersion("2.0.0")

	return cadwyn.NewVersionChange(
		"Add email field to User",
		v1,
		v2,
		// Request migration: v1 -> v2 (add email if missing)
		&cadwyn.AlterRequestInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(req *cadwyn.RequestInfo) error {
				if userMap, ok := req.Body.(map[string]interface{}); ok {
					if _, hasEmail := userMap["email"]; !hasEmail {
						userMap["email"] = "default@example.com"
					}
				}
				return nil
			},
		},
		// Response migration: v2 -> v1 (remove email)
		&cadwyn.AlterResponseInstruction{
			Schemas: []interface{}{User{}},
			Transformer: func(resp *cadwyn.ResponseInfo) error {
				if userMap, ok := resp.Body.(map[string]interface{}); ok {
					delete(userMap, "email")
				}
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
