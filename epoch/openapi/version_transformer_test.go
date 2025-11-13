package openapi

import (
	"reflect"
	"testing"

	"github.com/astronomer/epoch/epoch"
	"github.com/getkin/kin-openapi/openapi3"
)

// Test types for version transformation tests
type TestUser struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Status string `json:"status"`
}

func TestVersionTransformer_NewVersionTransformer(t *testing.T) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})

	transformer := NewVersionTransformer(vb)

	if transformer == nil {
		t.Fatal("expected transformer to be created")
	}

	if transformer.versionBundle != vb {
		t.Error("version bundle not set correctly")
	}

	if transformer.typeParser == nil {
		t.Error("type parser not initialized")
	}
}

func TestVersionTransformer_TransformSchemaForVersion_HeadVersion(t *testing.T) {
	// Setup: Create HEAD version
	head := epoch.NewHeadVersion()
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{head})
	transformer := NewVersionTransformer(vb)

	// Create a base schema
	baseSchema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"id":    openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
			"name":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
			"email": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
		},
	}

	// Transform for HEAD version (should return unchanged)
	result, err := transformer.TransformSchemaForVersion(
		baseSchema,
		reflect.TypeOf(TestUser{}),
		head,
		SchemaDirectionResponse,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// HEAD version should not modify the schema
	if len(result.Properties) != 3 {
		t.Errorf("expected 3 properties, got %d", len(result.Properties))
	}
}

func TestVersionTransformer_TransformSchemaForVersion_ResponseRemoveField(t *testing.T) {
	// Setup: Create v1 and v2 with a migration that removes a field
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")

	// Create version change: v1 -> v2 adds email, so v2 -> v1 removes email
	change := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email field").
		ForType(TestUser{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		Build()

	// Build version bundle with HEAD
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v2, v1})
	v2.Changes = []epoch.VersionChangeInterface{change}

	transformer := NewVersionTransformer(vb)

	// Create base schema (HEAD version with all fields)
	baseSchema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"id":    openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
			"name":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
			"email": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
		},
	}

	// Transform for v1 (should remove email)
	result, err := transformer.TransformSchemaForVersion(
		baseSchema,
		reflect.TypeOf(TestUser{}),
		v1,
		SchemaDirectionResponse,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that email was removed
	if _, hasEmail := result.Properties["email"]; hasEmail {
		t.Error("email field should be removed for v1")
	}

	// Check that other fields remain
	if _, hasID := result.Properties["id"]; !hasID {
		t.Error("id field should remain")
	}
	if _, hasName := result.Properties["name"]; !hasName {
		t.Error("name field should remain")
	}

	if len(result.Properties) != 2 {
		t.Errorf("expected 2 properties after removal, got %d", len(result.Properties))
	}
}

func TestVersionTransformer_TransformSchemaForVersion_ResponseRenameField(t *testing.T) {
	// Setup: Create v1 and v2 with a field rename
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")

	// v1 -> v2 renames "full_name" to "name", so v2 -> v1 renames "name" to "full_name"
	change := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Rename name to full_name").
		ForType(TestUser{}).
		ResponseToPreviousVersion().
		RenameField("name", "full_name").
		Build()

	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v2, v1})
	v2.Changes = []epoch.VersionChangeInterface{change}

	transformer := NewVersionTransformer(vb)

	// Base schema has "name"
	baseSchema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"id":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
			"name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
		},
	}

	// Transform for v1 (should rename name -> full_name)
	result, err := transformer.TransformSchemaForVersion(
		baseSchema,
		reflect.TypeOf(TestUser{}),
		v1,
		SchemaDirectionResponse,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check that "name" was removed
	if _, hasName := result.Properties["name"]; hasName {
		t.Error("name field should be renamed")
	}

	// Check that "full_name" exists
	if _, hasFullName := result.Properties["full_name"]; !hasFullName {
		t.Error("full_name field should exist after rename")
	}

	if len(result.Properties) != 2 {
		t.Errorf("expected 2 properties, got %d", len(result.Properties))
	}
}

func TestVersionTransformer_TransformSchemaForVersion_ResponseAddField(t *testing.T) {
	// Setup: Create v1 and v2 with field addition
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")

	// v1 -> v2 adds "status", so response to v1 from v2 removes it
	// But we can also test the opposite: adding a field when going to older version
	// Actually, ResponseToPreviousVersion typically removes, so let's test that
	change := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add status field").
		ForType(TestUser{}).
		ResponseToPreviousVersion().
		RemoveField("status").
		Build()

	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v2, v1})
	v2.Changes = []epoch.VersionChangeInterface{change}

	transformer := NewVersionTransformer(vb)

	baseSchema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"id":     openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
			"name":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
			"status": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
		},
	}

	result, err := transformer.TransformSchemaForVersion(
		baseSchema,
		reflect.TypeOf(TestUser{}),
		v1,
		SchemaDirectionResponse,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// status should be removed for v1
	if _, hasStatus := result.Properties["status"]; hasStatus {
		t.Error("status field should be removed for v1")
	}
}

func TestVersionTransformer_TransformSchemaForVersion_MultipleChanges(t *testing.T) {
	// Setup: Test that multiple field operations in one change work correctly
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")

	// v1 -> v2: adds email, phone, and status (so remove all three when going to v1)
	change := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add multiple fields").
		ForType(TestUser{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		RemoveField("phone").
		RemoveField("status").
		Build()

	// v1 is oldest (no changes), v2 has the change
	v2.Changes = []epoch.VersionChangeInterface{change}

	vb, err := epoch.NewVersionBundle([]*epoch.Version{v2, v1})
	if err != nil {
		t.Fatalf("failed to create version bundle: %v", err)
	}

	transformer := NewVersionTransformer(vb)

	// Base schema (v2/HEAD) has all fields
	baseSchema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"id":     openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
			"name":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
			"email":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
			"phone":  openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
			"status": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
		},
	}

	// Transform for v1 (should remove email, phone, status)
	result, err := transformer.TransformSchemaForVersion(
		baseSchema,
		reflect.TypeOf(TestUser{}),
		v1,
		SchemaDirectionResponse,
	)

	if err != nil {
		t.Fatalf("unexpected error for v1: %v", err)
	}

	// v1 should only have id and name (3 fields removed)
	if len(result.Properties) != 2 {
		t.Errorf("expected 2 properties for v1, got %d", len(result.Properties))
		// Print what we got for debugging
		for name := range result.Properties {
			t.Logf("  has field: %s", name)
		}
	}

	if _, hasEmail := result.Properties["email"]; hasEmail {
		t.Error("v1 should not have email")
	}
	if _, hasPhone := result.Properties["phone"]; hasPhone {
		t.Error("v1 should not have phone")
	}
	if _, hasStatus := result.Properties["status"]; hasStatus {
		t.Error("v1 should not have status")
	}
}

func TestVersionTransformer_CloneSchema(t *testing.T) {
	original := &openapi3.Schema{
		Type:        &openapi3.Types{"object"},
		Format:      "custom",
		Description: "Test schema",
		Properties: map[string]*openapi3.SchemaRef{
			"field1": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
		},
		Required: []string{"field1"},
	}

	clone := CloneSchema(original)

	// Verify clone is different object
	if clone == original {
		t.Error("clone should be a different object")
	}

	// Verify properties are copied
	if clone.Format != original.Format {
		t.Error("format not copied")
	}
	if clone.Description != original.Description {
		t.Error("description not copied")
	}
	if len(clone.Properties) != len(original.Properties) {
		t.Error("properties not copied correctly")
	}
	if len(clone.Required) != len(original.Required) {
		t.Error("required array not copied correctly")
	}

	// Modify clone and verify original is unchanged
	clone.Properties["field2"] = openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}})
	if len(original.Properties) != 1 {
		t.Error("modifying clone affected original")
	}
}

func TestVersionTransformer_AddFieldToSchema(t *testing.T) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
	transformer := NewVersionTransformer(vb)

	schema := &openapi3.Schema{
		Type:       &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{},
	}

	fieldSchema := openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}})

	// Add field without required
	transformer.AddFieldToSchema(schema, "test_field", fieldSchema, false)

	if _, exists := schema.Properties["test_field"]; !exists {
		t.Error("field not added to schema")
	}

	if len(schema.Required) != 0 {
		t.Error("field should not be in required array")
	}

	// Add field with required
	transformer.AddFieldToSchema(schema, "required_field", fieldSchema, true)

	if len(schema.Required) != 1 {
		t.Error("required field not added to required array")
	}

	if schema.Required[0] != "required_field" {
		t.Error("wrong field name in required array")
	}
}

func TestVersionTransformer_RemoveFieldFromSchema(t *testing.T) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
	transformer := NewVersionTransformer(vb)

	schema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"field1": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
			"field2": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
		},
		Required: []string{"field1", "field2"},
	}

	// Remove field
	transformer.RemoveFieldFromSchema(schema, "field1")

	if _, exists := schema.Properties["field1"]; exists {
		t.Error("field should be removed from properties")
	}

	if len(schema.Required) != 1 {
		t.Error("field should be removed from required array")
	}

	if schema.Required[0] != "field2" {
		t.Error("wrong field remaining in required array")
	}
}

func TestVersionTransformer_RenameFieldInSchema(t *testing.T) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v1})
	transformer := NewVersionTransformer(vb)

	stringSchema := openapi3.NewSchemaRef("", &openapi3.Schema{
		Type:   &openapi3.Types{"string"},
		Format: "email",
	})

	schema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"old_name": stringSchema,
			"field2":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
		},
		Required: []string{"old_name", "field2"},
	}

	// Rename field
	transformer.RenameFieldInSchema(schema, "old_name", "new_name")

	// Old name should be gone
	if _, exists := schema.Properties["old_name"]; exists {
		t.Error("old field name should be removed")
	}

	// New name should exist with same schema
	newField, exists := schema.Properties["new_name"]
	if !exists {
		t.Fatal("new field name should exist")
	}

	if newField.Value.Format != "email" {
		t.Error("field schema not preserved during rename")
	}

	// Check required array
	foundNewName := false
	for _, req := range schema.Required {
		if req == "old_name" {
			t.Error("old name should not be in required array")
		}
		if req == "new_name" {
			foundNewName = true
		}
	}

	if !foundNewName {
		t.Error("new name should be in required array")
	}
}

func TestVersionTransformer_ChangeAppliesToType(t *testing.T) {
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")

	// Create change that targets TestUser
	change := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Test change").
		ForType(TestUser{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		Build()

	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v2, v1})
	transformer := NewVersionTransformer(vb)

	// Should apply to TestUser
	if !transformer.changeAppliesToType(change, reflect.TypeOf(TestUser{}), SchemaDirectionResponse) {
		t.Error("change should apply to TestUser type")
	}

	// Should not apply to different type
	type OtherType struct {
		ID int
	}
	if transformer.changeAppliesToType(change, reflect.TypeOf(OtherType{}), SchemaDirectionResponse) {
		t.Error("change should not apply to OtherType")
	}

	// Should not apply to request direction if change is for response
	if transformer.changeAppliesToType(change, reflect.TypeOf(TestUser{}), SchemaDirectionRequest) {
		t.Error("response change should not apply to request direction")
	}
}

func TestVersionTransformer_NoOperationsForType(t *testing.T) {
	// Setup: Create change that doesn't target our test type
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")

	type DifferentType struct {
		Field string
	}

	change := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Change for different type").
		ForType(DifferentType{}).
		ResponseToPreviousVersion().
		RemoveField("field").
		Build()

	vb, _ := epoch.NewVersionBundle([]*epoch.Version{v2, v1})
	v2.Changes = []epoch.VersionChangeInterface{change}

	transformer := NewVersionTransformer(vb)

	baseSchema := &openapi3.Schema{
		Type: &openapi3.Types{"object"},
		Properties: map[string]*openapi3.SchemaRef{
			"id":   openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"integer"}}),
			"name": openapi3.NewSchemaRef("", &openapi3.Schema{Type: &openapi3.Types{"string"}}),
		},
	}

	// Transform TestUser type (no operations target it)
	result, err := transformer.TransformSchemaForVersion(
		baseSchema,
		reflect.TypeOf(TestUser{}),
		v1,
		SchemaDirectionResponse,
	)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Schema should be unchanged since no operations apply
	if len(result.Properties) != len(baseSchema.Properties) {
		t.Error("schema should be unchanged when no operations apply")
	}
}
