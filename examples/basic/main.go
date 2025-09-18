package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"time"

	"github.com/isaacchung/cadwyn-go/pkg/cadwyn"
	"github.com/isaacchung/cadwyn-go/pkg/middleware"
	"github.com/isaacchung/cadwyn-go/pkg/migration"
)

// Test Models
type UserV1 struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type UserV2 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type UserV3 struct {
	ID        int       `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// Version Changes
type V1ToV2Change struct {
	*migration.BaseVersionChange
}

func NewV1ToV2Change() *V1ToV2Change {
	return &V1ToV2Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"Added email field",
			cadwyn.DateVersion("2023-01-01"),
			cadwyn.DateVersion("2023-06-01"),
		),
	}
}

func (c *V1ToV2Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	if userMap, ok := data.(map[string]interface{}); ok {
		if _, hasEmail := userMap["email"]; !hasEmail {
			userMap["email"] = ""
		}
	}
	return data, nil
}

func (c *V1ToV2Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	return removeFieldsFromData(data, "email")
}

type V2ToV3Change struct {
	*migration.BaseVersionChange
}

func NewV2ToV3Change() *V2ToV3Change {
	return &V2ToV3Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"Added status and timestamp",
			cadwyn.DateVersion("2023-06-01"),
			cadwyn.DateVersion("2024-01-01"),
		),
	}
}

func (c *V2ToV3Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
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
	return removeFieldsFromData(data, "status", "created_at")
}

func removeFieldsFromData(data interface{}, fields ...string) (interface{}, error) {
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

// Test Cases
func main() {
	fmt.Println("🚀 Cadwyn-Go Basic Example")
	fmt.Println("Learn the fundamentals of API versioning with Cadwyn-Go")
	fmt.Println(strings.Repeat("=", 60))

	allPassed := true

	// Test 1: Basic Version Detection
	fmt.Println("\n1. 🔍 Testing Version Detection")
	if !testVersionDetection() {
		allPassed = false
	}

	// Test 2: Field Addition/Removal Migration
	fmt.Println("\n2. ➕ Testing Field Addition/Removal")
	if !testFieldMigration() {
		allPassed = false
	}

	// Test 3: Multi-Version Migration Chain
	fmt.Println("\n3. 🔗 Testing Multi-Version Migration")
	if !testMultiVersionMigration() {
		allPassed = false
	}

	// Test 4: Request vs Response Migration
	fmt.Println("\n4. 🔄 Testing Request vs Response Migration")
	if !testRequestResponseMigration() {
		allPassed = false
	}

	// Test 5: Array/Collection Handling
	fmt.Println("\n5. 📋 Testing Array/Collection Migration")
	if !testArrayMigration() {
		allPassed = false
	}

	// Test 6: Version Detection Methods
	fmt.Println("\n6. 🎯 Testing Different Version Detection Methods")
	if !testVersionDetectionMethods() {
		allPassed = false
	}

	// Test 7: Edge Cases and Error Handling
	fmt.Println("\n7. ⚠️  Testing Edge Cases")
	if !testEdgeCases() {
		allPassed = false
	}

	// Test 8: Default Values and Null Handling
	fmt.Println("\n8. 🔧 Testing Default Values")
	if !testDefaultValues() {
		allPassed = false
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 60))
	if allPassed {
		fmt.Println("🎉 Congratulations! You've learned the basics of Cadwyn-Go!")
		fmt.Println("📚 Next steps:")
		fmt.Println("   • Try the intermediate example: cd ../intermediate && go run main.go")
		fmt.Println("   • Explore advanced features: cd ../advanced && go run main.go")
		fmt.Println("   • Check out specific features: cd ../features/major_minor_versioning && go run main.go")
	} else {
		fmt.Println("❌ Some tests failed. Check the output above.")
	}
}

func testVersionDetection() bool {
	fmt.Println("   Testing header-based version detection...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01", "2024-01-01").
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	// Register test route
	app.Router().GET("/test", func(w http.ResponseWriter, r *http.Request) {
		version := middleware.GetVersionFromContext(r.Context())
		response := map[string]interface{}{
			"message": "test",
		}
		if version != nil {
			response["detected_version"] = version.String()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	tests := []struct {
		name    string
		headers map[string]string
		check   func(body []byte) bool
	}{
		{
			name:    "Specific version in header",
			headers: map[string]string{"x-api-version": "2023-01-01"},
			check: func(body []byte) bool {
				var resp map[string]interface{}
				json.Unmarshal(body, &resp)
				return resp["detected_version"] == "2023-01-01"
			},
		},
		{
			name:    "No version header (uses default)",
			headers: map[string]string{},
			check: func(body []byte) bool {
				var resp map[string]interface{}
				json.Unmarshal(body, &resp)
				// Should use the latest version as default
				return resp["detected_version"] == "2024-01-01"
			},
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/test", nil)
		for k, v := range test.headers {
			req.Header.Set(k, v)
		}

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			fmt.Printf("   ❌ %s: Expected 200, got %d\n", test.name, rr.Code)
			return false
		}

		if !test.check(rr.Body.Bytes()) {
			fmt.Printf("   ❌ %s: Response validation failed\n", test.name)
			fmt.Printf("      Response: %s\n", rr.Body.String())
			return false
		}

		fmt.Printf("   ✅ %s\n", test.name)
	}

	return true
}

func testFieldMigration() bool {
	fmt.Println("   Testing field addition/removal migration...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionChanges(NewV1ToV2Change()).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	// Register test route that returns v2 data
	app.Router().GET("/users", func(w http.ResponseWriter, r *http.Request) {
		users := []UserV2{
			{ID: 1, Name: "Alice", Email: "alice@example.com"},
			{ID: 2, Name: "Bob", Email: "bob@example.com"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	tests := []struct {
		name    string
		version string
		check   func(body []byte) bool
	}{
		{
			name:    "V2 includes email field",
			version: "2023-06-01",
			check: func(body []byte) bool {
				var users []map[string]interface{}
				json.Unmarshal(body, &users)
				return len(users) > 0 && users[0]["email"] != nil
			},
		},
		{
			name:    "V1 excludes email field",
			version: "2023-01-01",
			check: func(body []byte) bool {
				var users []map[string]interface{}
				json.Unmarshal(body, &users)
				if len(users) == 0 {
					return false
				}
				_, hasEmail := users[0]["email"]
				return !hasEmail // Should NOT have email field
			},
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("x-api-version", test.version)

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			fmt.Printf("   ❌ %s: Expected 200, got %d\n", test.name, rr.Code)
			return false
		}

		if !test.check(rr.Body.Bytes()) {
			fmt.Printf("   ❌ %s: Field migration failed\n", test.name)
			fmt.Printf("      Response: %s\n", rr.Body.String())
			return false
		}

		fmt.Printf("   ✅ %s\n", test.name)
	}

	return true
}

func testMultiVersionMigration() bool {
	fmt.Println("   Testing multi-version migration chain...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01", "2024-01-01").
		WithVersionChanges(
			NewV1ToV2Change(),
			NewV2ToV3Change(),
		).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	// Register test route that returns v3 data (latest)
	app.Router().GET("/users", func(w http.ResponseWriter, r *http.Request) {
		users := []UserV3{
			{
				ID:        1,
				Name:      "Alice",
				Email:     "alice@example.com",
				Status:    "active",
				CreatedAt: time.Now(),
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	tests := []struct {
		name         string
		version      string
		hasEmail     bool
		hasStatus    bool
		hasCreatedAt bool
	}{
		{
			name:         "V3 has all fields",
			version:      "2024-01-01",
			hasEmail:     true,
			hasStatus:    true,
			hasCreatedAt: true,
		},
		{
			name:         "V2 has email but not status/timestamp",
			version:      "2023-06-01",
			hasEmail:     true,
			hasStatus:    false,
			hasCreatedAt: false,
		},
		{
			name:         "V1 has only basic fields",
			version:      "2023-01-01",
			hasEmail:     false,
			hasStatus:    false,
			hasCreatedAt: false,
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/users", nil)
		req.Header.Set("x-api-version", test.version)

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			fmt.Printf("   ❌ %s: Expected 200, got %d\n", test.name, rr.Code)
			return false
		}

		var users []map[string]interface{}
		if err := json.Unmarshal(rr.Body.Bytes(), &users); err != nil {
			fmt.Printf("   ❌ %s: Failed to parse JSON\n", test.name)
			return false
		}

		if len(users) == 0 {
			fmt.Printf("   ❌ %s: No users in response\n", test.name)
			return false
		}

		user := users[0]

		// Check email field
		_, hasEmail := user["email"]
		if hasEmail != test.hasEmail {
			fmt.Printf("   ❌ %s: Email field expectation failed (has: %v, expected: %v)\n", test.name, hasEmail, test.hasEmail)
			return false
		}

		// Check status field
		_, hasStatus := user["status"]
		if hasStatus != test.hasStatus {
			fmt.Printf("   ❌ %s: Status field expectation failed (has: %v, expected: %v)\n", test.name, hasStatus, test.hasStatus)
			return false
		}

		// Check created_at field
		_, hasCreatedAt := user["created_at"]
		if hasCreatedAt != test.hasCreatedAt {
			fmt.Printf("   ❌ %s: CreatedAt field expectation failed (has: %v, expected: %v)\n", test.name, hasCreatedAt, test.hasCreatedAt)
			return false
		}

		fmt.Printf("   ✅ %s\n", test.name)
	}

	return true
}

func testRequestResponseMigration() bool {
	fmt.Println("   Testing request vs response migration...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionChanges(NewV1ToV2Change()).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	// Register POST route that accepts and returns user data
	app.Router().POST("/users", func(w http.ResponseWriter, r *http.Request) {
		var user map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// Add ID and ensure email exists (simulating v2 processing)
		user["id"] = 999
		if _, hasEmail := user["email"]; !hasEmail {
			user["email"] = "default@example.com"
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)
	})

	// Test: Send v1 request (no email), should get v1 response (no email)
	requestData := map[string]interface{}{
		"name": "John Doe",
	}
	requestBody, _ := json.Marshal(requestData)

	req := httptest.NewRequest("POST", "/users", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-version", "2023-01-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		fmt.Printf("   ❌ Request migration: Expected 201, got %d\n", rr.Code)
		fmt.Printf("      Response: %s\n", rr.Body.String())
		return false
	}

	var responseUser map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &responseUser); err != nil {
		fmt.Printf("   ❌ Request migration: Failed to parse response JSON\n")
		return false
	}

	// Response should not contain email field for v1 client
	if _, hasEmail := responseUser["email"]; hasEmail {
		fmt.Printf("   ❌ Request migration: v1 response should not contain email field\n")
		fmt.Printf("      Response: %s\n", rr.Body.String())
		return false
	}

	// But should contain other fields
	if responseUser["name"] != "John Doe" || responseUser["id"] == nil {
		fmt.Printf("   ❌ Request migration: Missing expected fields\n")
		fmt.Printf("      Response: %s\n", rr.Body.String())
		return false
	}

	fmt.Printf("   ✅ Request/Response migration works correctly\n")
	return true
}

func testArrayMigration() bool {
	fmt.Println("   Testing array/collection migration...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionChanges(NewV1ToV2Change()).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	// Register route that returns array of users
	app.Router().GET("/users", func(w http.ResponseWriter, r *http.Request) {
		users := []UserV2{
			{ID: 1, Name: "Alice", Email: "alice@example.com"},
			{ID: 2, Name: "Bob", Email: "bob@example.com"},
			{ID: 3, Name: "Carol", Email: "carol@example.com"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)
	})

	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("x-api-version", "2023-01-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Array migration: Expected 200, got %d\n", rr.Code)
		return false
	}

	var users []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &users); err != nil {
		fmt.Printf("   ❌ Array migration: Failed to parse JSON\n")
		return false
	}

	if len(users) != 3 {
		fmt.Printf("   ❌ Array migration: Expected 3 users, got %d\n", len(users))
		return false
	}

	// Check that all users have email field removed
	for i, user := range users {
		if _, hasEmail := user["email"]; hasEmail {
			fmt.Printf("   ❌ Array migration: User %d should not have email field\n", i)
			return false
		}
		if user["name"] == nil || user["id"] == nil {
			fmt.Printf("   ❌ Array migration: User %d missing required fields\n", i)
			return false
		}
	}

	fmt.Printf("   ✅ Array migration works correctly\n")
	return true
}

func testVersionDetectionMethods() bool {
	fmt.Println("   Testing different version detection methods...")

	// Test query parameter detection
	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionLocation(middleware.VersionLocationQuery).
		WithVersionParameter("api_version").
		Build()
	if err != nil {
		fmt.Printf("   ❌ Query setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/test", func(w http.ResponseWriter, r *http.Request) {
		version := middleware.GetVersionFromContext(r.Context())
		response := map[string]interface{}{"detected_version": "unknown"}
		if version != nil {
			response["detected_version"] = version.String()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	req := httptest.NewRequest("GET", "/test?api_version=2023-01-01", nil)
	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Query parameter detection failed: %d\n", rr.Code)
		return false
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)
	if response["detected_version"] != "2023-01-01" {
		fmt.Printf("   ❌ Query parameter version not detected correctly: %v\n", response["detected_version"])
		return false
	}

	fmt.Printf("   ✅ Query parameter version detection works\n")
	return true
}

func testEdgeCases() bool {
	fmt.Println("   Testing edge cases...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "ok"})
	})

	tests := []struct {
		name    string
		version string
		expect  int
	}{
		{
			name:    "Invalid version format",
			version: "invalid-version",
			expect:  http.StatusOK, // Should fallback to default
		},
		{
			name:    "Future version",
			version: "2025-01-01",
			expect:  http.StatusOK, // Should fallback to default
		},
		{
			name:    "Empty version",
			version: "",
			expect:  http.StatusOK, // Should use default
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/test", nil)
		if test.version != "" {
			req.Header.Set("x-api-version", test.version)
		}

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != test.expect {
			fmt.Printf("   ❌ %s: Expected %d, got %d\n", test.name, test.expect, rr.Code)
			return false
		}

		fmt.Printf("   ✅ %s\n", test.name)
	}

	return true
}

func testDefaultValues() bool {
	fmt.Println("   Testing default values...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionChanges(NewV1ToV2Change()).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	// Test that request migration adds default values
	app.Router().POST("/users", func(w http.ResponseWriter, r *http.Request) {
		var user map[string]interface{}
		json.NewDecoder(r.Body).Decode(&user)

		// The migration should have added email field
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(user)
	})

	// Send v1 request (no email field)
	requestData := map[string]interface{}{"name": "Test User"}
	requestBody, _ := json.Marshal(requestData)

	req := httptest.NewRequest("POST", "/users", bytes.NewReader(requestBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-version", "2023-01-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Default values: Expected 200, got %d\n", rr.Code)
		return false
	}

	fmt.Printf("   ✅ Default values handling works\n")
	return true
}
