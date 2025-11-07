package main

import (
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/astronomer/epoch/epoch"
	"github.com/astronomer/epoch/epoch/openapi"
)

// Example types that match the advanced example

type CreateUserRequest struct {
	FullName string `json:"full_name" binding:"required,max=100"`
	Email    string `json:"email" binding:"required,email"`
	Phone    string `json:"phone,omitempty"`
	Status   string `json:"status" binding:"required,oneof=active inactive pending suspended"`
}

type UserResponse struct {
	ID        int        `json:"id,omitempty" validate:"required"`
	FullName  string     `json:"full_name" validate:"required"`
	Email     string     `json:"email,omitempty" validate:"required,email"`
	Phone     string     `json:"phone,omitempty"`
	Status    string     `json:"status,omitempty"`
	CreatedAt *time.Time `json:"created_at,omitempty" format:"date-time"`
}

type OrganizationResponse struct {
	ID        string            `json:"id" validate:"required" example:"clmaxoarx000008l2c5ayb9pt"`
	Name      string            `json:"name" validate:"required" example:"My organization"`
	Product   string            `json:"product,omitempty" enums:"HOSTED,HYBRID" example:"HOSTED"`
	Status    string            `json:"status,omitempty" enums:"ACTIVE,INACTIVE,SUSPENDED" example:"ACTIVE"`
	CreatedAt *time.Time        `json:"createdAt" validate:"required" format:"date-time"`
	UpdatedAt *time.Time        `json:"updatedAt" validate:"required" format:"date-time"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func main() {
	fmt.Println("=== Epoch OpenAPI Schema Generation Example ===\n")

	// Define versions
	v1, _ := epoch.NewDateVersion("2024-01-01")
	v2, _ := epoch.NewDateVersion("2024-06-01")
	v3, _ := epoch.NewDateVersion("2025-01-01")

	// Define version migrations
	v1ToV2 := epoch.NewVersionChangeBuilder(v1, v2).
		Description("Add email and status fields to User").
		ForType(UserResponse{}, CreateUserRequest{}).
		ResponseToPreviousVersion().
		RemoveField("email").
		RemoveField("status").
		Build()

	v2ToV3 := epoch.NewVersionChangeBuilder(v2, v3).
		Description("Rename name to full_name, add phone").
		ForType(UserResponse{}, CreateUserRequest{}).
		ResponseToPreviousVersion().
		RenameField("full_name", "name").
		RemoveField("phone").
		Build()

	// Create Epoch instance
	epochInstance, err := epoch.NewEpoch().
		WithVersions(v1, v2, v3).
		WithHeadVersion().
		WithChanges(v1ToV2, v2ToV3).
		WithTypes(
			CreateUserRequest{},
			UserResponse{},
			OrganizationResponse{},
		).
		Build()

	if err != nil {
		log.Fatalf("Failed to create Epoch instance: %v", err)
	}

	fmt.Println("✓ Created Epoch instance with 3 versions + HEAD")
	fmt.Println("  Versions: 2024-01-01, 2024-06-01, 2025-01-01, head")
	fmt.Println()

	// Create schema generator
	generator := openapi.NewSchemaGenerator(openapi.SchemaGeneratorConfig{
		VersionBundle: epochInstance.VersionBundle(),
		TypeRegistry:  epochInstance.EndpointRegistry(),
		OutputFormat:  "yaml",
	})

	fmt.Println("✓ Created OpenAPI schema generator")
	fmt.Println()

	// Generate schemas for each type and version
	fmt.Println("📝 Generating schemas for UserResponse type...")
	fmt.Println()

	// Generate for HEAD version
	headVersion := epochInstance.VersionBundle().GetHeadVersion()
	headSchema, err := generator.GetSchemaForType(
		reflect.TypeOf(UserResponse{}),
		headVersion,
		openapi.SchemaDirectionResponse,
	)
	if err != nil {
		log.Fatalf("Failed to generate HEAD schema: %v", err)
	}

	fmt.Println("HEAD Version Schema:")
	fmt.Printf("  Properties: %d\n", len(headSchema.Properties))
	for name := range headSchema.Properties {
		fmt.Printf("    - %s\n", name)
	}
	fmt.Println()

	// Generate for v1
	v1Schema, err := generator.GetSchemaForType(
		reflect.TypeOf(UserResponse{}),
		v1,
		openapi.SchemaDirectionResponse,
	)
	if err != nil {
		log.Fatalf("Failed to generate v1 schema: %v", err)
	}

	fmt.Println("v1 (2024-01-01) Schema:")
	fmt.Printf("  Properties: %d\n", len(v1Schema.Properties))
	for name := range v1Schema.Properties {
		fmt.Printf("    - %s\n", name)
	}
	fmt.Println()

	// Generate for v2
	v2Schema, err := generator.GetSchemaForType(
		reflect.TypeOf(UserResponse{}),
		v2,
		openapi.SchemaDirectionResponse,
	)
	if err != nil {
		log.Fatalf("Failed to generate v2 schema: %v", err)
	}

	fmt.Println("v2 (2024-06-01) Schema:")
	fmt.Printf("  Properties: %d\n", len(v2Schema.Properties))
	for name := range v2Schema.Properties {
		fmt.Printf("    - %s\n", name)
	}
	fmt.Println()

	// Generate for v3
	v3Schema, err := generator.GetSchemaForType(
		reflect.TypeOf(UserResponse{}),
		v3,
		openapi.SchemaDirectionResponse,
	)
	if err != nil {
		log.Fatalf("Failed to generate v3 schema: %v", err)
	}

	fmt.Println("v3 (2025-01-01) Schema:")
	fmt.Printf("  Properties: %d\n", len(v3Schema.Properties))
	for name := range v3Schema.Properties {
		fmt.Printf("    - %s\n", name)
	}
	fmt.Println()

	// Verify transformations
	fmt.Println("✅ Verification:")
	success := true

	// v1 should have id, name, phone (no email, no status, no created_at)
	if _, has := v1Schema.Properties["email"]; has {
		fmt.Println("  ✗ v1 should not have email field")
		success = false
	} else {
		fmt.Println("  ✓ v1 correctly excludes email field")
	}

	if _, has := v1Schema.Properties["status"]; has {
		fmt.Println("  ✗ v1 should not have status field")
		success = false
	} else {
		fmt.Println("  ✓ v1 correctly excludes status field")
	}

	// v2 should have renamed field
	if _, has := v2Schema.Properties["name"]; has {
		fmt.Println("  ✓ v2 has 'name' field (renamed from full_name)")
	} else {
		fmt.Println("  ✗ v2 should have 'name' field")
		success = false
	}

	if _, has := v2Schema.Properties["full_name"]; has {
		fmt.Println("  ✗ v2 should not have 'full_name' field (renamed to name)")
		success = false
	} else {
		fmt.Println("  ✓ v2 correctly renamed full_name to name")
	}

	if _, has := v2Schema.Properties["phone"]; has {
		fmt.Println("  ✗ v2 should not have phone field")
		success = false
	} else {
		fmt.Println("  ✓ v2 correctly excludes phone field")
	}

	fmt.Println()

	if success {
		fmt.Println("🎉 All schema transformations working correctly!")
	} else {
		fmt.Println("⚠️  Some schema transformations failed")
	}

	fmt.Println()
	fmt.Println("📚 This example demonstrates:")
	fmt.Println("  ✓ Automatic schema generation from Go types")
	fmt.Println("  ✓ Version-specific schema transformations")
	fmt.Println("  ✓ Field additions, removals, and renames")
	fmt.Println("  ✓ Validation of generated schemas")
	fmt.Println()
	fmt.Println("See epoch/openapi/README.md for full documentation")
}
