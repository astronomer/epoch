package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/isaacchung/cadwyn-go/pkg/cadwyn"
	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

// API Models demonstrating major.minor versioning

// UserV1_0 represents a user in API version 1.0
type UserV1_0 struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// UserV1_1 represents a user in API version 1.1 - added email
type UserV1_1 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserV2_0 represents a user in API version 2.0 - major change: added role and status
type UserV2_0 struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Role   string `json:"role"`
	Status string `json:"status"`
}

// UserV2_1 represents a user in API version 2.1 - added created_at timestamp
type UserV2_1 struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Version Changes for Major.Minor versioning

// V1_0ToV1_1Change - minor version bump: adds email field
type V1_0ToV1_1Change struct {
	*migration.BaseVersionChange
}

func NewV1_0ToV1_1Change() *V1_0ToV1_1Change {
	return &V1_0ToV1_1Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"v1.0 → v1.1: Added email field",
			cadwyn.SemverVersion("1.0"),
			cadwyn.SemverVersion("1.1"),
		),
	}
}

func (c *V1_0ToV1_1Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	if userMap, ok := data.(map[string]interface{}); ok {
		if _, hasEmail := userMap["email"]; !hasEmail {
			userMap["email"] = ""
		}
	}
	return data, nil
}

func (c *V1_0ToV1_1Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	return removeFields(data, "email")
}

// V1_1ToV2_0Change - major version bump: adds role and status
type V1_1ToV2_0Change struct {
	*migration.BaseVersionChange
}

func NewV1_1ToV2_0Change() *V1_1ToV2_0Change {
	return &V1_1ToV2_0Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"v1.1 → v2.0: Added role and status fields (breaking change)",
			cadwyn.SemverVersion("1.1"),
			cadwyn.SemverVersion("2.0"),
		),
	}
}

func (c *V1_1ToV2_0Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	if userMap, ok := data.(map[string]interface{}); ok {
		if _, hasRole := userMap["role"]; !hasRole {
			userMap["role"] = "user" // Default role
		}
		if _, hasStatus := userMap["status"]; !hasStatus {
			userMap["status"] = "active" // Default status
		}
	}
	return data, nil
}

func (c *V1_1ToV2_0Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	return removeFields(data, "role", "status")
}

// V2_0ToV2_1Change - minor version bump: adds created_at timestamp
type V2_0ToV2_1Change struct {
	*migration.BaseVersionChange
}

func NewV2_0ToV2_1Change() *V2_0ToV2_1Change {
	return &V2_0ToV2_1Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"v2.0 → v2.1: Added created_at timestamp",
			cadwyn.SemverVersion("2.0"),
			cadwyn.SemverVersion("2.1"),
		),
	}
}

func (c *V2_0ToV2_1Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	if userMap, ok := data.(map[string]interface{}); ok {
		if _, hasCreatedAt := userMap["created_at"]; !hasCreatedAt {
			userMap["created_at"] = time.Now().Format(time.RFC3339)
		}
	}
	return data, nil
}

func (c *V2_0ToV2_1Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	return removeFields(data, "created_at")
}

// Utility function to remove fields from data
func removeFields(data interface{}, fields ...string) (interface{}, error) {
	if dataMap, ok := data.(map[string]interface{}); ok {
		for _, field := range fields {
			delete(dataMap, field)
		}
		return dataMap, nil
	}

	if dataSlice, ok := data.([]interface{}); ok {
		for _, item := range dataSlice {
			if itemMap, ok := item.(map[string]interface{}); ok {
				for _, field := range fields {
					delete(itemMap, field)
				}
			}
		}
	}

	return data, nil
}

func main() {
	fmt.Println("🚀 Cadwyn-Go Major.Minor Versioning Example")
	fmt.Println(strings.Repeat("=", 60))

	// Create Cadwyn app with major.minor versions
	app, err := cadwyn.NewBuilder().
		WithSemverVersions("1.0", "1.1", "2.0", "2.1"). // Major.Minor format
		WithVersionChanges(
			NewV1_0ToV1_1Change(),
			NewV1_1ToV2_0Change(),
			NewV2_0ToV2_1Change(),
		).
		WithDebugLogging().
		Build()

	if err != nil {
		panic(fmt.Sprintf("Failed to create Cadwyn app: %v", err))
	}

	fmt.Printf("✅ Created app with %d versions:\n", len(app.GetVersions()))
	for _, v := range app.GetVersions() {
		fmt.Printf("   - %s\n", v.String())
	}

	fmt.Printf("✅ Configured %d version changes:\n", len(app.GetVersionChanges()))
	for _, change := range app.GetVersionChanges() {
		fmt.Printf("   - %s\n", change.Description())
	}

	// Register API endpoint that returns v2.1 data (latest)
	app.Router().GET("/users", func(w http.ResponseWriter, r *http.Request) {
		users := []UserV2_1{
			{
				ID:        1,
				Name:      "Alice Johnson",
				Email:     "alice@example.com",
				Role:      "admin",
				Status:    "active",
				CreatedAt: time.Date(2023, 1, 15, 10, 30, 0, 0, time.UTC),
			},
			{
				ID:        2,
				Name:      "Bob Smith",
				Email:     "bob@example.com",
				Role:      "user",
				Status:    "active",
				CreatedAt: time.Date(2023, 3, 22, 9, 15, 0, 0, time.UTC),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	fmt.Println("\n🧪 Testing Major.Minor Version Migration:")

	// Test each version
	versions := []struct {
		version     string
		description string
		fields      []string
	}{
		{"1.0", "Basic user (id, name only)", []string{"id", "name"}},
		{"1.1", "Added email", []string{"id", "name", "email"}},
		{"2.0", "Added role and status (major change)", []string{"id", "name", "email", "role", "status"}},
		{"2.1", "Added created_at timestamp", []string{"id", "name", "email", "role", "status", "created_at"}},
	}

	for i, test := range versions {
		fmt.Printf("\n%d. 📋 Testing API v%s - %s\n", i+1, test.version, test.description)

		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("x-api-version", test.version)

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			fmt.Printf("   ❌ Request failed with status %d\n", rr.Code)
			continue
		}

		var users []map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &users); err != nil {
			fmt.Printf("   ❌ Failed to parse JSON response\n")
			continue
		}

		if len(users) == 0 {
			fmt.Printf("   ❌ No users in response\n")
			continue
		}

		user := users[0]
		fmt.Printf("   📊 Response fields: ")

		// Check which fields are present
		presentFields := []string{}
		for _, field := range test.fields {
			if _, exists := user[field]; exists {
				presentFields = append(presentFields, field)
			}
		}

		fmt.Printf("%v\n", presentFields)

		// Verify expected fields are present and unexpected ones are absent
		allExpected := true
		for _, expectedField := range test.fields {
			if _, exists := user[expectedField]; !exists {
				fmt.Printf("   ❌ Missing expected field: %s\n", expectedField)
				allExpected = false
			}
		}

		// Check for unexpected fields (fields that shouldn't be in this version)
		allVersionFields := []string{"id", "name", "email", "role", "status", "created_at"}
		for _, field := range allVersionFields {
			if _, exists := user[field]; exists {
				// Check if this field should be present in this version
				shouldExist := false
				for _, expectedField := range test.fields {
					if field == expectedField {
						shouldExist = true
						break
					}
				}
				if !shouldExist {
					fmt.Printf("   ⚠️  Unexpected field present: %s\n", field)
					allExpected = false
				}
			}
		}

		if allExpected {
			fmt.Printf("   ✅ Migration successful - all fields correct\n")
		}
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))

	// Demonstrate version comparison
	fmt.Println("🔍 Version Comparison Examples:")

	v1_0 := cadwyn.SemverVersion("1.0")
	v1_1 := cadwyn.SemverVersion("1.1")
	v2_0 := cadwyn.SemverVersion("2.0")
	v2_1 := cadwyn.SemverVersion("2.1")

	comparisons := []struct {
		v1, v2 *version.Version
		desc   string
	}{
		{v1_0, v1_1, "v1.0 < v1.1 (minor version bump)"},
		{v1_1, v2_0, "v1.1 < v2.0 (major version bump)"},
		{v2_0, v2_1, "v2.0 < v2.1 (minor version bump)"},
		{v1_0, v2_1, "v1.0 < v2.1 (across major versions)"},
	}

	for _, comp := range comparisons {
		result := comp.v1.Compare(comp.v2)
		symbol := "="
		if result < 0 {
			symbol = "<"
		} else if result > 0 {
			symbol = ">"
		}
		fmt.Printf("   %s %s %s ✅ %s\n", comp.v1.String(), symbol, comp.v2.String(), comp.desc)
	}

	fmt.Println("\n🎯 Key Benefits of Major.Minor Versioning:")
	fmt.Println("   • ✅ Semantic meaning: Major = breaking changes, Minor = new features")
	fmt.Println("   • ✅ Simpler version numbers (no patch needed for API versioning)")
	fmt.Println("   • ✅ Clear backward compatibility expectations")
	fmt.Println("   • ✅ Automatic migration between all versions")
	fmt.Println("   • ✅ Supports mixed version types (dates + semver)")

	fmt.Println("\n🚀 Usage Examples:")
	fmt.Println("   # Request v1.0 (basic user)")
	fmt.Println("   curl -H 'x-api-version: 1.0' http://localhost:8080/users")
	fmt.Println()
	fmt.Println("   # Request v1.1 (with email)")
	fmt.Println("   curl -H 'x-api-version: 1.1' http://localhost:8080/users")
	fmt.Println()
	fmt.Println("   # Request v2.0 (with role and status)")
	fmt.Println("   curl -H 'x-api-version: 2.0' http://localhost:8080/users")
	fmt.Println()
	fmt.Println("   # Request v2.1 (with timestamps)")
	fmt.Println("   curl -H 'x-api-version: 2.1' http://localhost:8080/users")

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	fmt.Println("🎉 Major.Minor versioning is fully supported in Cadwyn-Go!")
}
