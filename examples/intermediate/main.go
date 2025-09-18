package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/isaacchung/cadwyn-go/pkg/cadwyn"
	"github.com/isaacchung/cadwyn-go/pkg/migration"
)

// User models for different API versions

// UserV1 represents a user in API version 1.0 (2023-01-01)
type UserV1 struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// UserV2 represents a user in API version 2.0 (2023-06-01) - added email
type UserV2 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserV3 represents a user in API version 3.0 (2024-01-01) - added timestamps and status
type UserV3 struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Version changes to handle migrations between versions

// V1ToV2Change handles migration from v1 to v2 (adds email field)
type V1ToV2Change struct {
	*migration.BaseVersionChange
}

func NewV1ToV2Change() *V1ToV2Change {
	return &V1ToV2Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"Added email field to User model",
			cadwyn.DateVersion("2023-01-01"),
			cadwyn.DateVersion("2023-06-01"),
		),
	}
}

func (c *V1ToV2Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	// Convert UserV1 to UserV2 by adding default email
	if userMap, ok := data.(map[string]interface{}); ok {
		if _, hasEmail := userMap["email"]; !hasEmail {
			userMap["email"] = ""
		}
	}
	return data, nil
}

func (c *V1ToV2Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	// Convert UserV2 to UserV1 by removing email field
	if userMap, ok := data.(map[string]interface{}); ok {
		delete(userMap, "email")
		return userMap, nil
	}

	// Handle slice of users
	if users, ok := data.([]interface{}); ok {
		for _, user := range users {
			if userMap, ok := user.(map[string]interface{}); ok {
				delete(userMap, "email")
			}
		}
	}

	return data, nil
}

// V2ToV3Change handles migration from v2 to v3 (adds status and timestamps)
type V2ToV3Change struct {
	*migration.BaseVersionChange
}

func NewV2ToV3Change() *V2ToV3Change {
	return &V2ToV3Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"Added status and timestamp fields to User model",
			cadwyn.DateVersion("2023-06-01"),
			cadwyn.DateVersion("2024-01-01"),
		),
	}
}

func (c *V2ToV3Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	// Convert UserV2 to UserV3 by adding default status and timestamps
	if userMap, ok := data.(map[string]interface{}); ok {
		if _, hasStatus := userMap["status"]; !hasStatus {
			userMap["status"] = "active"
		}
		if _, hasCreatedAt := userMap["created_at"]; !hasCreatedAt {
			userMap["created_at"] = time.Now().Format(time.RFC3339)
		}
	}
	return data, nil
}

func (c *V2ToV3Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	// Convert UserV3 to UserV2 by removing status and timestamp fields
	if userMap, ok := data.(map[string]interface{}); ok {
		delete(userMap, "status")
		delete(userMap, "created_at")
		return userMap, nil
	}

	// Handle slice of users
	if users, ok := data.([]interface{}); ok {
		for _, user := range users {
			if userMap, ok := user.(map[string]interface{}); ok {
				delete(userMap, "status")
				delete(userMap, "created_at")
			}
		}
	}

	return data, nil
}

// In-memory data store (for demonstration)
var users = []UserV3{
	{
		ID:        1,
		Name:      "Alice Johnson",
		Email:     "alice@example.com",
		Status:    "active",
		CreatedAt: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
	},
	{
		ID:        2,
		Name:      "Bob Smith",
		Email:     "bob@example.com",
		Status:    "inactive",
		CreatedAt: time.Date(2023, 3, 22, 9, 15, 0, 0, time.UTC),
	},
	{
		ID:        3,
		Name:      "Carol Davis",
		Email:     "carol@example.com",
		Status:    "active",
		CreatedAt: time.Date(2023, 5, 8, 11, 0, 0, 0, time.UTC),
	},
}

var nextUserID = 4

// HTTP handlers

func handleGetUsers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Filter by status if provided
	status := r.URL.Query().Get("status")
	filteredUsers := users

	if status != "" {
		filteredUsers = []UserV3{}
		for _, user := range users {
			if user.Status == status {
				filteredUsers = append(filteredUsers, user)
			}
		}
	}

	json.NewEncoder(w).Encode(filteredUsers)
}

func handleGetUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path
	path := strings.TrimPrefix(r.URL.Path, "/users/")
	userID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Find user
	for _, user := range users {
		if user.ID == userID {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(user)
			return
		}
	}

	http.Error(w, "User not found", http.StatusNotFound)
}

func handleCreateUser(w http.ResponseWriter, r *http.Request) {
	var newUser UserV3
	if err := json.NewDecoder(r.Body).Decode(&newUser); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Set defaults for new user
	newUser.ID = nextUserID
	nextUserID++

	if newUser.Status == "" {
		newUser.Status = "active"
	}

	now := time.Now()
	if newUser.CreatedAt.IsZero() {
		newUser.CreatedAt = now
	}

	users = append(users, newUser)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(newUser)
}

func handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path
	path := strings.TrimPrefix(r.URL.Path, "/users/")
	userID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Find and update user
	for i, user := range users {
		if user.ID == userID {
			var updateData UserV3
			if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
				http.Error(w, "Invalid JSON", http.StatusBadRequest)
				return
			}

			// Update fields
			if updateData.Name != "" {
				users[i].Name = updateData.Name
			}
			if updateData.Email != "" {
				users[i].Email = updateData.Email
			}
			if updateData.Status != "" {
				users[i].Status = updateData.Status
			}

			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(users[i])
			return
		}
	}

	http.Error(w, "User not found", http.StatusNotFound)
}

func handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	// Extract user ID from path
	path := strings.TrimPrefix(r.URL.Path, "/users/")
	userID, err := strconv.Atoi(path)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	// Find and delete user
	for i, user := range users {
		if user.ID == userID {
			users = append(users[:i], users[i+1:]...)
			w.WriteHeader(http.StatusNoContent)
			return
		}
	}

	http.Error(w, "User not found", http.StatusNotFound)
}

// API info handler
func handleAPIInfo(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"name":        "Cadwyn Go Example API",
		"description": "A comprehensive example of API versioning with Cadwyn-Go",
		"versions": []string{
			"2023-01-01 (v1.0) - Basic user management",
			"2023-06-01 (v2.0) - Added email field",
			"2024-01-01 (v3.0) - Added status and timestamps",
		},
		"endpoints": []string{
			"GET /api/info",
			"GET /users",
			"GET /users/{id}",
			"POST /users",
			"PUT /users/{id}",
			"DELETE /users/{id}",
		},
		"version_detection": "Header: x-api-version",
		"example_requests": []string{
			"curl -H 'x-api-version: 2023-01-01' http://localhost:8080/users",
			"curl -H 'x-api-version: 2023-06-01' http://localhost:8080/users",
			"curl -H 'x-api-version: 2024-01-01' http://localhost:8080/users",
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(info)
}

func main() {
	fmt.Println("🏗️  Cadwyn-Go Intermediate Example")
	fmt.Println("Production-ready API server with versioning")

	// Create Cadwyn application with fluent builder API
	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01", "2024-01-01").
		WithVersionChanges(
			NewV1ToV2Change(),
			NewV2ToV3Change(),
		).
		WithSchemaAnalysis().
		WithDebugLogging().
		Build()

	if err != nil {
		log.Fatal("Failed to create Cadwyn app:", err)
	}

	// Print application info
	fmt.Printf("📅 Configured versions: %d\n", len(app.GetVersions()))
	for _, v := range app.GetVersions() {
		fmt.Printf("   - %s\n", v.String())
	}

	fmt.Printf("🔄 Version changes: %d\n", len(app.GetVersionChanges()))
	for _, change := range app.GetVersionChanges() {
		fmt.Printf("   - %s\n", change.Description())
	}

	// Register routes using the router
	router := app.Router()

	// API info endpoint
	router.GET("/api/info", handleAPIInfo)

	// User endpoints
	router.GET("/users", handleGetUsers)
	router.GET("/users/", handleGetUser) // Simplified path matching
	router.POST("/users", handleCreateUser)
	router.PUT("/users/", handleUpdateUser)    // Simplified path matching
	router.DELETE("/users/", handleDeleteUser) // Simplified path matching

	// Print route information
	fmt.Println("\n📍 Registered routes:")
	for _, route := range app.GetRouteInfo() {
		fmt.Printf("   %s %s (versions: %v)\n", route.Method, route.Pattern, route.Versions)
	}

	// Create HTTP server
	server := &http.Server{
		Addr:    ":8080",
		Handler: app, // Cadwyn implements http.Handler
	}

	// Print usage instructions
	fmt.Println("\n🌐 Server starting on http://localhost:8080")
	fmt.Println("\n📖 Try these example requests:")
	fmt.Println("   # Get API info")
	fmt.Println("   curl http://localhost:8080/api/info")
	fmt.Println()
	fmt.Println("   # Get users in different API versions")
	fmt.Println("   curl -H 'x-api-version: 2023-01-01' http://localhost:8080/users")
	fmt.Println("   curl -H 'x-api-version: 2023-06-01' http://localhost:8080/users")
	fmt.Println("   curl -H 'x-api-version: 2024-01-01' http://localhost:8080/users")
	fmt.Println()
	fmt.Println("   # Create a new user (will be migrated based on API version)")
	fmt.Println("   curl -X POST -H 'Content-Type: application/json' -H 'x-api-version: 2023-01-01' \\")
	fmt.Println("        -d '{\"name\":\"John Doe\"}' http://localhost:8080/users")
	fmt.Println()
	fmt.Println("   # Get specific user")
	fmt.Println("   curl -H 'x-api-version: 2023-06-01' http://localhost:8080/users/1")
	fmt.Println()
	fmt.Println("   # Filter users by status (v3.0 feature)")
	fmt.Println("   curl -H 'x-api-version: 2024-01-01' 'http://localhost:8080/users?status=active'")
	fmt.Println()
	fmt.Println("🔧 Version detection:")
	fmt.Println("   - Header: x-api-version: YYYY-MM-DD")
	fmt.Println("   - Default: Latest version (2024-01-01)")
	fmt.Println("   - Automatic request/response migration")
	fmt.Println()

	// Start server
	log.Printf("Server listening on %s", server.Addr)
	if err := server.ListenAndServe(); err != nil {
		log.Fatal("Server failed:", err)
	}
}
