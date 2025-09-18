package migration

import (
	"context"
	"testing"

	"github.com/isaacchung/cadwyn-go/pkg/version"
)

func TestBaseVersionChange(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")

	change := NewBaseVersionChange("Test change", v1, v2)

	if change.Description() != "Test change" {
		t.Errorf("expected description 'Test change', got '%s'", change.Description())
	}

	if !change.FromVersion().Equal(v1) {
		t.Errorf("from version should equal v1")
	}

	if !change.ToVersion().Equal(v2) {
		t.Errorf("to version should equal v2")
	}

	if !change.AppliesTo(v1, v2) {
		t.Errorf("change should apply to v1->v2")
	}

	if change.AppliesTo(v2, v1) {
		t.Errorf("change should not apply to v2->v1")
	}

	// Test default migration (should return data unchanged)
	testData := map[string]interface{}{"test": "value"}

	migratedReq, err := change.MigrateRequest(context.Background(), testData)
	if err != nil {
		t.Errorf("request migration should not error: %v", err)
	}

	// For maps, we need to compare the contents
	migratedReqMap, ok := migratedReq.(map[string]interface{})
	if !ok {
		t.Errorf("migrated request should be a map")
	} else if migratedReqMap["test"] != "value" {
		t.Errorf("base migration should return data unchanged")
	}

	migratedResp, err := change.MigrateResponse(context.Background(), testData)
	if err != nil {
		t.Errorf("response migration should not error: %v", err)
	}

	migratedRespMap, ok := migratedResp.(map[string]interface{})
	if !ok {
		t.Errorf("migrated response should be a map")
	} else if migratedRespMap["test"] != "value" {
		t.Errorf("base migration should return data unchanged")
	}
}

func TestStructVersionChange(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")

	requestChanges := []FieldChange{
		AddField("email", "default@example.com"),
	}

	responseChanges := []FieldChange{
		RemoveField("email"),
	}

	change := NewStructVersionChange(
		"Add email field",
		v1, v2,
		requestChanges,
		responseChanges,
	)

	// Test request migration (should add email field)
	requestData := map[string]interface{}{
		"id":   1,
		"name": "Alice",
	}

	migratedReq, err := change.MigrateRequest(context.Background(), requestData)
	if err != nil {
		t.Errorf("request migration failed: %v", err)
	}

	reqMap, ok := migratedReq.(map[string]interface{})
	if !ok {
		t.Errorf("migrated request should be a map")
	}

	if reqMap["email"] != "default@example.com" {
		t.Errorf("expected email field with default value")
	}

	// Test response migration (should remove email field)
	responseData := map[string]interface{}{
		"id":    1,
		"name":  "Alice",
		"email": "alice@example.com",
	}

	migratedResp, err := change.MigrateResponse(context.Background(), responseData)
	if err != nil {
		t.Errorf("response migration failed: %v", err)
	}

	respMap, ok := migratedResp.(map[string]interface{})
	if !ok {
		t.Errorf("migrated response should be a map")
	}

	if _, exists := respMap["email"]; exists {
		t.Errorf("email field should be removed from response")
	}
}

func TestMigrationChain(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")
	v3, _ := version.NewDateVersion("2023-03-01")

	// Create changes: v1->v2 and v2->v3
	change1 := NewStructVersionChange(
		"Add email field",
		v1, v2,
		[]FieldChange{AddField("email", "")},
		[]FieldChange{RemoveField("email")},
	)

	change2 := NewStructVersionChange(
		"Add created_at field",
		v2, v3,
		[]FieldChange{AddField("created_at", "2023-01-01T00:00:00Z")},
		[]FieldChange{RemoveField("created_at")},
	)

	chain := NewMigrationChain([]VersionChange{change1, change2})

	// Test forward migration v1->v3
	data := map[string]interface{}{
		"id":   1,
		"name": "Alice",
	}

	migrated, err := chain.MigrateRequest(context.Background(), data, v1, v3)
	if err != nil {
		t.Errorf("chain migration failed: %v", err)
	}

	migratedMap, ok := migrated.(map[string]interface{})
	if !ok {
		t.Errorf("migrated data should be a map")
	}

	// Should have both email and created_at fields
	if _, exists := migratedMap["email"]; !exists {
		t.Errorf("should have email field after migration")
	}

	if _, exists := migratedMap["created_at"]; !exists {
		t.Errorf("should have created_at field after migration")
	}

	// Test reverse migration v3->v1
	reverseData := map[string]interface{}{
		"id":         1,
		"name":       "Alice",
		"email":      "alice@example.com",
		"created_at": "2023-01-01T00:00:00Z",
	}

	reverseMigrated, err := chain.MigrateResponse(context.Background(), reverseData, v3, v1)
	if err != nil {
		t.Errorf("reverse chain migration failed: %v", err)
	}

	reverseMigratedMap, ok := reverseMigrated.(map[string]interface{})
	if !ok {
		t.Errorf("reverse migrated data should be a map")
	}

	// Should not have email or created_at fields
	if _, exists := reverseMigratedMap["email"]; exists {
		t.Errorf("should not have email field after reverse migration")
	}

	if _, exists := reverseMigratedMap["created_at"]; exists {
		t.Errorf("should not have created_at field after reverse migration")
	}
}

func TestFindChangesForMigration(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")
	v3, _ := version.NewDateVersion("2023-03-01")

	change1 := NewBaseVersionChange("Change 1", v1, v2)
	change2 := NewBaseVersionChange("Change 2", v2, v3)

	chain := NewMigrationChain([]VersionChange{change1, change2})

	// Test finding changes for v1->v3 migration
	changes := chain.FindChangesForMigration(v1, v3)

	if len(changes) == 0 {
		t.Errorf("should find changes for v1->v3 migration")
	}

	// Test finding changes for v3->v1 migration (reverse)
	reverseChanges := chain.FindChangesForMigration(v3, v1)

	if len(reverseChanges) == 0 {
		t.Errorf("should find changes for v3->v1 reverse migration")
	}
}
