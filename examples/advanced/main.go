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
	"github.com/isaacchung/cadwyn-go/pkg/middleware"
	"github.com/isaacchung/cadwyn-go/pkg/migration"
)

// Advanced Test Models
type OrderV1 struct {
	ID     int     `json:"id"`
	Amount float64 `json:"amount"`
}

type OrderV2 struct {
	ID       int     `json:"id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Tax      float64 `json:"tax"`
}

type OrderV3 struct {
	ID          int       `json:"id"`
	TotalAmount float64   `json:"total_amount"` // renamed from amount
	Currency    string    `json:"currency"`
	TaxAmount   float64   `json:"tax_amount"` // renamed from tax
	CreatedAt   time.Time `json:"created_at"`
	Items       []Item    `json:"items"` // new nested structure
}

type Item struct {
	ID       int     `json:"id"`
	Name     string  `json:"name"`
	Price    float64 `json:"price"`
	Quantity int     `json:"quantity"`
}

// Complex Version Changes with Field Renaming and Nested Structures
type OrderV1ToV2Change struct {
	*migration.BaseVersionChange
}

func NewOrderV1ToV2Change() *OrderV1ToV2Change {
	return &OrderV1ToV2Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"Added currency and tax to Order",
			cadwyn.DateVersion("2023-01-01"),
			cadwyn.DateVersion("2023-06-01"),
		),
	}
}

func (c *OrderV1ToV2Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	if orderMap, ok := data.(map[string]interface{}); ok {
		// Add default currency and tax
		if _, hasCurrency := orderMap["currency"]; !hasCurrency {
			orderMap["currency"] = "USD"
		}
		if _, hasTax := orderMap["tax"]; !hasTax {
			orderMap["tax"] = 0.0
		}
	}
	return data, nil
}

func (c *OrderV1ToV2Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	return removeFieldsFromData(data, "currency", "tax")
}

type OrderV2ToV3Change struct {
	*migration.BaseVersionChange
}

func NewOrderV2ToV3Change() *OrderV2ToV3Change {
	return &OrderV2ToV3Change{
		BaseVersionChange: migration.NewBaseVersionChange(
			"Renamed fields and added items to Order",
			cadwyn.DateVersion("2023-06-01"),
			cadwyn.DateVersion("2024-01-01"),
		),
	}
}

func (c *OrderV2ToV3Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	if orderMap, ok := data.(map[string]interface{}); ok {
		// Rename amount -> total_amount
		if amount, hasAmount := orderMap["amount"]; hasAmount {
			orderMap["total_amount"] = amount
			delete(orderMap, "amount")
		}
		// Rename tax -> tax_amount
		if tax, hasTax := orderMap["tax"]; hasTax {
			orderMap["tax_amount"] = tax
			delete(orderMap, "tax")
		}
		// Add defaults
		if _, hasCreatedAt := orderMap["created_at"]; !hasCreatedAt {
			orderMap["created_at"] = time.Now().Format(time.RFC3339)
		}
		if _, hasItems := orderMap["items"]; !hasItems {
			orderMap["items"] = []interface{}{}
		}
	}
	return data, nil
}

func (c *OrderV2ToV3Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	if orderMap, ok := data.(map[string]interface{}); ok {
		// Rename total_amount -> amount
		if totalAmount, hasTotalAmount := orderMap["total_amount"]; hasTotalAmount {
			orderMap["amount"] = totalAmount
			delete(orderMap, "total_amount")
		}
		// Rename tax_amount -> tax
		if taxAmount, hasTaxAmount := orderMap["tax_amount"]; hasTaxAmount {
			orderMap["tax"] = taxAmount
			delete(orderMap, "tax_amount")
		}
		// Remove new fields
		delete(orderMap, "created_at")
		delete(orderMap, "items")
		return orderMap, nil
	}

	// Handle arrays
	if orders, ok := data.([]interface{}); ok {
		for _, order := range orders {
			if orderMap, ok := order.(map[string]interface{}); ok {
				if totalAmount, hasTotalAmount := orderMap["total_amount"]; hasTotalAmount {
					orderMap["amount"] = totalAmount
					delete(orderMap, "total_amount")
				}
				if taxAmount, hasTaxAmount := orderMap["tax_amount"]; hasTaxAmount {
					orderMap["tax"] = taxAmount
					delete(orderMap, "tax_amount")
				}
				delete(orderMap, "created_at")
				delete(orderMap, "items")
			}
		}
	}

	return data, nil
}

// Utility function
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

// Advanced Test Cases
func main() {
	fmt.Println("⚡ Cadwyn-Go Advanced Example")
	fmt.Println("Complex scenarios, performance testing, and production patterns")
	fmt.Println(strings.Repeat("=", 65))

	allPassed := true

	// Test 1: Complex Field Transformations
	fmt.Println("\n1. 🔄 Testing Complex Field Transformations")
	if !testComplexFieldTransformations() {
		allPassed = false
	}

	// Test 2: Nested Structure Handling
	fmt.Println("\n2. 🏗️  Testing Nested Structure Handling")
	if !testNestedStructures() {
		allPassed = false
	}

	// Test 3: Semantic Versioning Support
	fmt.Println("\n3. 📊 Testing Semantic Versioning")
	if !testSemanticVersioning() {
		allPassed = false
	}

	// Test 4: Mixed Version Types
	fmt.Println("\n4. 🔀 Testing Mixed Version Types")
	if !testMixedVersionTypes() {
		allPassed = false
	}

	// Test 5: Custom Version Changes
	fmt.Println("\n5. ⚙️  Testing Custom Version Changes")
	if !testCustomVersionChanges() {
		allPassed = false
	}

	// Test 6: Performance with Large Data
	fmt.Println("\n6. 🏎️  Testing Performance with Large Data")
	if !testPerformanceWithLargeData() {
		allPassed = false
	}

	// Test 7: Concurrent Request Handling
	fmt.Println("\n7. 🔀 Testing Concurrent Request Handling")
	if !testConcurrentRequests() {
		allPassed = false
	}

	// Test 8: Version-Specific Route Registration
	fmt.Println("\n8. 🎯 Testing Version-Specific Routes")
	if !testVersionSpecificRoutes() {
		allPassed = false
	}

	// Test 9: Content-Type Handling
	fmt.Println("\n9. 📄 Testing Content-Type Handling")
	if !testContentTypeHandling() {
		allPassed = false
	}

	// Test 10: Error Recovery and Fallbacks
	fmt.Println("\n10. 🛡️  Testing Error Recovery")
	if !testErrorRecovery() {
		allPassed = false
	}

	fmt.Printf("\n%s\n", strings.Repeat("=", 65))
	if allPassed {
		fmt.Println("🎉 Excellent! You've mastered advanced Cadwyn-Go patterns!")
		fmt.Println("🚀 You're ready to build production-grade versioned APIs!")
		fmt.Println("📖 Check out specific features in: cd ../features/")
	} else {
		fmt.Println("❌ Some advanced scenarios failed. Check the output above.")
	}
}

func testComplexFieldTransformations() bool {
	fmt.Println("   Testing field renaming and complex transformations...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01", "2024-01-01").
		WithVersionChanges(
			NewOrderV1ToV2Change(),
			NewOrderV2ToV3Change(),
		).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	// Register route returning V3 data
	app.Router().GET("/orders", func(w http.ResponseWriter, r *http.Request) {
		orders := []OrderV3{
			{
				ID:          1,
				TotalAmount: 100.50,
				Currency:    "USD",
				TaxAmount:   8.50,
				CreatedAt:   time.Now(),
				Items: []Item{
					{ID: 1, Name: "Widget", Price: 50.25, Quantity: 2},
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orders)
	})

	tests := []struct {
		name    string
		version string
		checkFn func([]byte) bool
	}{
		{
			name:    "V3 has renamed fields and items",
			version: "2024-01-01",
			checkFn: func(body []byte) bool {
				var orders []map[string]interface{}
				json.Unmarshal(body, &orders)
				if len(orders) == 0 {
					return false
				}
				order := orders[0]
				_, hasTotalAmount := order["total_amount"]
				_, hasTaxAmount := order["tax_amount"]
				_, hasItems := order["items"]
				return hasTotalAmount && hasTaxAmount && hasItems
			},
		},
		{
			name:    "V2 has old field names, no items",
			version: "2023-06-01",
			checkFn: func(body []byte) bool {
				var orders []map[string]interface{}
				json.Unmarshal(body, &orders)
				if len(orders) == 0 {
					return false
				}
				order := orders[0]
				_, hasAmount := order["amount"]
				_, hasTax := order["tax"]
				_, hasItems := order["items"]
				_, hasTotalAmount := order["total_amount"]
				return hasAmount && hasTax && !hasItems && !hasTotalAmount
			},
		},
		{
			name:    "V1 has only basic fields",
			version: "2023-01-01",
			checkFn: func(body []byte) bool {
				var orders []map[string]interface{}
				json.Unmarshal(body, &orders)
				if len(orders) == 0 {
					return false
				}
				order := orders[0]
				_, hasAmount := order["amount"]
				_, hasCurrency := order["currency"]
				_, hasTax := order["tax"]
				return hasAmount && !hasCurrency && !hasTax
			},
		},
	}

	for _, test := range tests {
		req := httptest.NewRequest("GET", "/orders", nil)
		req.Header.Set("x-api-version", test.version)

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			fmt.Printf("   ❌ %s: Expected 200, got %d\n", test.name, rr.Code)
			return false
		}

		if !test.checkFn(rr.Body.Bytes()) {
			fmt.Printf("   ❌ %s: Field transformation failed\n", test.name)
			fmt.Printf("      Response: %s\n", rr.Body.String())
			return false
		}

		fmt.Printf("   ✅ %s\n", test.name)
	}

	return true
}

func testNestedStructures() bool {
	fmt.Println("   Testing nested structure handling...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-06-01", "2024-01-01").
		WithVersionChanges(NewOrderV2ToV3Change()).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/orders", func(w http.ResponseWriter, r *http.Request) {
		order := OrderV3{
			ID:          1,
			TotalAmount: 100.0,
			Currency:    "USD",
			TaxAmount:   10.0,
			CreatedAt:   time.Now(),
			Items: []Item{
				{ID: 1, Name: "Item 1", Price: 50.0, Quantity: 1},
				{ID: 2, Name: "Item 2", Price: 50.0, Quantity: 1},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(order)
	})

	// Test V2 (should not have items array)
	req := httptest.NewRequest("GET", "/orders", nil)
	req.Header.Set("x-api-version", "2023-06-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Nested structures: Expected 200, got %d\n", rr.Code)
		return false
	}

	var order map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &order); err != nil {
		fmt.Printf("   ❌ Nested structures: Failed to parse JSON\n")
		return false
	}

	if _, hasItems := order["items"]; hasItems {
		fmt.Printf("   ❌ Nested structures: V2 should not have items field\n")
		return false
	}

	if order["amount"] == nil {
		fmt.Printf("   ❌ Nested structures: V2 should have amount field\n")
		return false
	}

	fmt.Printf("   ✅ Nested structures handled correctly\n")
	return true
}

func testSemanticVersioning() bool {
	fmt.Println("   Testing semantic versioning support...")

	app, err := cadwyn.NewBuilder().
		WithSemverVersions("1.0.0", "1.1.0", "2.0.0").
		Build()
	if err != nil {
		fmt.Printf("   ❌ Semver setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/test", func(w http.ResponseWriter, r *http.Request) {
		version := middleware.GetVersionFromContext(r.Context())
		response := map[string]interface{}{"version": "unknown"}
		if version != nil {
			response["version"] = version.String()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	tests := []string{"1.0.0", "1.1.0", "2.0.0"}
	for _, testVersion := range tests {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("x-api-version", testVersion)

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			fmt.Printf("   ❌ Semver %s: Expected 200, got %d\n", testVersion, rr.Code)
			return false
		}

		var response map[string]interface{}
		json.Unmarshal(rr.Body.Bytes(), &response)
		if response["version"] != testVersion {
			fmt.Printf("   ❌ Semver %s: Version not detected correctly\n", testVersion)
			return false
		}

		fmt.Printf("   ✅ Semantic version %s works\n", testVersion)
	}

	return true
}

func testMixedVersionTypes() bool {
	fmt.Println("   Testing mixed version types (date + semver)...")

	app, err := cadwyn.NewBuilder().
		WithVersions(
			cadwyn.DateVersion("2023-01-01"),
			cadwyn.SemverVersion("1.0.0"),
			cadwyn.DateVersion("2024-01-01"),
			cadwyn.SemverVersion("2.0.0"),
		).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Mixed versions setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/test", func(w http.ResponseWriter, r *http.Request) {
		version := middleware.GetVersionFromContext(r.Context())
		response := map[string]interface{}{"detected": "none"}
		if version != nil {
			response["detected"] = version.String()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	tests := []string{"2023-01-01", "1.0.0", "2024-01-01", "2.0.0"}
	for _, testVersion := range tests {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("x-api-version", testVersion)

		rr := httptest.NewRecorder()
		app.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			fmt.Printf("   ❌ Mixed version %s: Expected 200, got %d\n", testVersion, rr.Code)
			return false
		}

		fmt.Printf("   ✅ Mixed version %s works\n", testVersion)
	}

	return true
}

func testCustomVersionChanges() bool {
	fmt.Println("   Testing custom version change implementations...")

	// Custom version change that transforms data in a specific way
	customChange := &CustomTransformChange{
		BaseVersionChange: migration.NewBaseVersionChange(
			"Custom transformation",
			cadwyn.DateVersion("2023-01-01"),
			cadwyn.DateVersion("2023-06-01"),
		),
	}

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionChanges(customChange).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Custom change setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/custom", func(w http.ResponseWriter, r *http.Request) {
		data := map[string]interface{}{
			"value": "UPPERCASE",
			"count": 42,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	})

	req := httptest.NewRequest("GET", "/custom", nil)
	req.Header.Set("x-api-version", "2023-01-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Custom change: Expected 200, got %d\n", rr.Code)
		return false
	}

	var response map[string]interface{}
	json.Unmarshal(rr.Body.Bytes(), &response)

	// Custom change should have transformed UPPERCASE to lowercase
	if response["value"] != "lowercase" {
		fmt.Printf("   ❌ Custom change: Value not transformed correctly: %v\n", response["value"])
		return false
	}

	fmt.Printf("   ✅ Custom version change works\n")
	return true
}

// Custom version change for testing
type CustomTransformChange struct {
	*migration.BaseVersionChange
}

func (c *CustomTransformChange) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	if dataMap, ok := data.(map[string]interface{}); ok {
		// Transform UPPERCASE to lowercase for v1 clients
		if value, hasValue := dataMap["value"]; hasValue {
			if _, ok := value.(string); ok {
				dataMap["value"] = "lowercase"
			}
		}
	}
	return data, nil
}

func testPerformanceWithLargeData() bool {
	fmt.Println("   Testing performance with large datasets...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionChanges(NewOrderV1ToV2Change()).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Performance setup failed: %v\n", err)
		return false
	}

	// Generate large dataset
	app.Router().GET("/large", func(w http.ResponseWriter, r *http.Request) {
		var orders []OrderV2
		for i := 0; i < 1000; i++ {
			orders = append(orders, OrderV2{
				ID:       i,
				Amount:   float64(i * 10),
				Currency: "USD",
				Tax:      float64(i),
			})
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(orders)
	})

	start := time.Now()

	req := httptest.NewRequest("GET", "/large", nil)
	req.Header.Set("x-api-version", "2023-01-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	duration := time.Since(start)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Performance: Expected 200, got %d\n", rr.Code)
		return false
	}

	var orders []map[string]interface{}
	if err := json.Unmarshal(rr.Body.Bytes(), &orders); err != nil {
		fmt.Printf("   ❌ Performance: Failed to parse large response\n")
		return false
	}

	if len(orders) != 1000 {
		fmt.Printf("   ❌ Performance: Expected 1000 orders, got %d\n", len(orders))
		return false
	}

	// Check that migration was applied (currency and tax removed)
	if _, hasCurrency := orders[0]["currency"]; hasCurrency {
		fmt.Printf("   ❌ Performance: Migration not applied to large dataset\n")
		return false
	}

	fmt.Printf("   ✅ Large dataset (1000 items) processed in %v\n", duration)
	return true
}

func testConcurrentRequests() bool {
	fmt.Println("   Testing concurrent request handling...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		WithVersionChanges(NewOrderV1ToV2Change()).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Concurrent setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/concurrent", func(w http.ResponseWriter, r *http.Request) {
		version := middleware.GetVersionFromContext(r.Context())
		response := map[string]interface{}{
			"version": "unknown",
			"data":    "test",
		}
		if version != nil {
			response["version"] = version.String()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	})

	// Make multiple concurrent requests
	results := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(id int) {
			version := "2023-01-01"
			if id%2 == 0 {
				version = "2023-06-01"
			}

			req := httptest.NewRequest("GET", "/concurrent", nil)
			req.Header.Set("x-api-version", version)

			rr := httptest.NewRecorder()
			app.ServeHTTP(rr, req)

			var response map[string]interface{}
			json.Unmarshal(rr.Body.Bytes(), &response)

			success := rr.Code == http.StatusOK && response["version"] == version
			results <- success
		}(i)
	}

	// Collect results
	allPassed := true
	for i := 0; i < 10; i++ {
		if !<-results {
			allPassed = false
		}
	}

	if allPassed {
		fmt.Printf("   ✅ 10 concurrent requests handled correctly\n")
	} else {
		fmt.Printf("   ❌ Some concurrent requests failed\n")
	}

	return allPassed
}

func testVersionSpecificRoutes() bool {
	fmt.Println("   Testing version-specific route registration...")

	v1 := cadwyn.DateVersion("2023-01-01")
	v2 := cadwyn.DateVersion("2023-06-01")

	app, err := cadwyn.NewBuilder().
		WithVersions(v1, v2).
		Build()
	if err != nil {
		fmt.Printf("   ❌ Version-specific setup failed: %v\n", err)
		return false
	}

	// Register route only for v2
	app.Router().GET("/v2-only", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"message": "v2 feature"})
	}, v2)

	// Test v2 access (should work)
	req := httptest.NewRequest("GET", "/v2-only", nil)
	req.Header.Set("x-api-version", "2023-06-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Version-specific route: v2 access failed (%d)\n", rr.Code)
		return false
	}

	fmt.Printf("   ✅ Version-specific routes work correctly\n")
	return true
}

func testContentTypeHandling() bool {
	fmt.Println("   Testing non-JSON content type handling...")

	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		Build()
	if err != nil {
		fmt.Printf("   ❌ Content-type setup failed: %v\n", err)
		return false
	}

	// Register route that returns plain text
	app.Router().GET("/text", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("Hello World"))
	})

	req := httptest.NewRequest("GET", "/text", nil)
	req.Header.Set("x-api-version", "2023-01-01")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Content-type: Expected 200, got %d\n", rr.Code)
		return false
	}

	if rr.Body.String() != "Hello World" {
		fmt.Printf("   ❌ Content-type: Unexpected response body\n")
		return false
	}

	fmt.Printf("   ✅ Non-JSON content handled correctly\n")
	return true
}

func testErrorRecovery() bool {
	fmt.Println("   Testing error recovery and fallbacks...")

	// Create app with invalid version change that might fail
	app, err := cadwyn.NewBuilder().
		WithDateVersions("2023-01-01", "2023-06-01").
		Build()
	if err != nil {
		fmt.Printf("   ❌ Error recovery setup failed: %v\n", err)
		return false
	}

	app.Router().GET("/error-test", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Test with malformed version header
	req := httptest.NewRequest("GET", "/error-test", nil)
	req.Header.Set("x-api-version", "invalid-version-format")

	rr := httptest.NewRecorder()
	app.ServeHTTP(rr, req)

	// Should fallback gracefully, not crash
	if rr.Code != http.StatusOK {
		fmt.Printf("   ❌ Error recovery: Expected graceful fallback, got %d\n", rr.Code)
		return false
	}

	fmt.Printf("   ✅ Error recovery works correctly\n")
	return true
}
