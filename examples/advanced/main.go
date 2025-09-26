package main

import (
	"fmt"
	"strings"

	"github.com/astronomer/cadwyn-go/cadwyn"
)

// UserV1 - Original user model
type UserV1 struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// UserV2 - Added email field
type UserV2 struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

// UserV3 - Added phone and status fields
type UserV3 struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Email  string `json:"email"`
	Phone  string `json:"phone"`
	Status string `json:"status"`
}

// Advanced example showing complex version changes and migrations
func main() {
	fmt.Println("🚀 Cadwyn-Go - Advanced Example")
	fmt.Println("Complex Version Changes & Migration Instructions")
	fmt.Println(strings.Repeat("=", 60))

	// Create versions using date-based versioning
	v1, _ := cadwyn.NewDateVersion("2023-01-01")
	v2, _ := cadwyn.NewDateVersion("2023-06-01")
	v3, _ := cadwyn.NewDateVersion("2024-01-01")
	head := cadwyn.NewHeadVersion()

	// Create version changes with instructions
	v1ToV2Change := createV1ToV2Change(v1, v2)
	v2ToV3Change := createV2ToV3Change(v2, v3)

	// Create Cadwyn instance with all versions and changes
	cadwynInstance, err := cadwyn.NewCadwyn().
		WithVersions(v1, v2, v3, head).
		WithChanges(v1ToV2Change, v2ToV3Change).
		WithTypes(UserV1{}, UserV2{}, UserV3{}).
		WithVersionLocation(cadwyn.VersionLocationHeader).
		WithVersionParameter("X-API-Version").
		WithVersionFormat(cadwyn.VersionFormatDate).
		Build()

	if err != nil {
		fmt.Printf("❌ Error creating Cadwyn instance: %v\n", err)
		return
	}

	fmt.Printf("✅ Created Cadwyn instance with %d versions\n", len(cadwynInstance.GetVersions()))
	fmt.Printf("✅ Head version: %s\n", cadwynInstance.GetHeadVersion().String())

	// Demonstrate advanced features
	demonstrateVersionBundle(cadwynInstance)
	demonstrateMigrationChain(cadwynInstance)
	demonstrateSchemaGeneration(cadwynInstance)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 Advanced Example Complete!")
	fmt.Println("📚 Key Features Demonstrated:")
	fmt.Println("   • Multi-version API with complex changes")
	fmt.Println("   • Migration chains for request/response transformation")
	fmt.Println("   • Schema generation for different versions")
	fmt.Println("   • Date-based versioning")
}

func demonstrateVersionBundle(cadwynInstance *cadwyn.Cadwyn) {
	fmt.Println("\n📦 Version Bundle Operations:")

	// Show all versions
	versions := cadwynInstance.GetVersions()
	fmt.Printf("   Available versions: ")
	for i, v := range versions {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(v.String())
	}
	fmt.Println()

	// Test version parsing
	testVersions := []string{"2023-01-01", "2023-06-01", "2024-01-01", "head", "invalid"}
	fmt.Println("   Version parsing results:")
	for _, vStr := range testVersions {
		if v, err := cadwynInstance.ParseVersion(vStr); err == nil {
			fmt.Printf("      ✅ '%s' -> %s (Type: %s)\n", vStr, v.String(), v.Type.String())
		} else {
			fmt.Printf("      ❌ '%s' -> Error: %s\n", vStr, err.Error())
		}
	}

	// Show version relationships
	fmt.Println("   Version relationships:")
	for i, v := range versions {
		if i == 0 {
			continue
		}
		prev := versions[i-1]
		if v.IsNewerThan(prev) {
			fmt.Printf("      %s > %s ✅\n", v.String(), prev.String())
		}
	}
}

func demonstrateMigrationChain(cadwynInstance *cadwyn.Cadwyn) {
	fmt.Println("\n🔄 Migration Chain Operations:")

	migrationChain := cadwynInstance.GetMigrationChain()
	changes := migrationChain.GetChanges()

	fmt.Printf("   Migration chain has %d changes:\n", len(changes))
	for i, change := range changes {
		fmt.Printf("      %d. %s (%s -> %s)\n",
			i+1,
			change.Description(),
			change.FromVersion().String(),
			change.ToVersion().String())
	}

	// Demonstrate migration path calculation
	versions := cadwynInstance.GetVersions()
	if len(versions) >= 3 {
		from := versions[0] // 2023-01-01
		to := versions[2]   // 2024-01-01

		fmt.Printf("   Migration path from %s to %s:\n", from.String(), to.String())
		path := migrationChain.GetMigrationPath(from, to)
		for i, change := range path {
			fmt.Printf("      %d. %s\n", i+1, change.Description())
		}
	}
}

func demonstrateSchemaGeneration(cadwynInstance *cadwyn.Cadwyn) {
	fmt.Println("\n🏗️  Schema Generation:")

	schemaGenerator := cadwynInstance.GetSchemaGenerator()
	if schemaGenerator == nil {
		fmt.Println("   ❌ Schema generation not available")
		return
	}

	// Generate structs for different versions
	testVersions := []string{"2023-01-01", "2023-06-01", "2024-01-01", "head"}

	for _, version := range testVersions {
		fmt.Printf("   Generating UserV3 for version %s:\n", version)
		if generatedCode, err := cadwynInstance.GenerateStructForVersion(UserV3{}, version); err == nil {
			// Show just the struct definition part
			lines := strings.Split(generatedCode, "\n")
			inStruct := false
			structLines := 0

			for _, line := range lines {
				if strings.Contains(line, "type UserV3 struct") {
					inStruct = true
				}
				if inStruct {
					fmt.Printf("      %s\n", line)
					structLines++
					if strings.Contains(line, "}") && structLines > 1 {
						break
					}
				}
			}
		} else {
			fmt.Printf("      ❌ Error: %s\n", err.Error())
		}
		fmt.Println()
	}
}

func createV1ToV2Change(from, to *cadwyn.Version) *cadwyn.VersionChange {
	return cadwyn.NewVersionChange(
		"Add email field to User model",
		from,
		to,
		&cadwyn.AlterRequestInstruction{
			Schemas: []interface{}{UserV1{}},
			Transformer: func(requestInfo *cadwyn.RequestInfo) error {
				if userMap, ok := requestInfo.Body.(map[string]interface{}); ok {
					// Add default email for v1 -> v2 migration
					if _, hasEmail := userMap["email"]; !hasEmail {
						userMap["email"] = "user@example.com"
					}
					requestInfo.Body = userMap
				}
				return nil
			},
		},
		&cadwyn.AlterResponseInstruction{
			Schemas: []interface{}{UserV2{}},
			Transformer: func(responseInfo *cadwyn.ResponseInfo) error {
				if userMap, ok := responseInfo.Body.(map[string]interface{}); ok {
					// Remove email for v2 -> v1 migration
					delete(userMap, "email")
					responseInfo.Body = userMap
				}
				return nil
			},
		},
		&cadwyn.SchemaInstruction{
			Schema: UserV2{},
			Name:   "email",
			Type:   "field_added",
			Attributes: map[string]interface{}{
				"type":     "string",
				"required": true,
			},
		},
	)
}

func createV2ToV3Change(from, to *cadwyn.Version) *cadwyn.VersionChange {
	return cadwyn.NewVersionChange(
		"Add phone and status fields to User model",
		from,
		to,
		&cadwyn.AlterRequestInstruction{
			Schemas: []interface{}{UserV2{}},
			Transformer: func(requestInfo *cadwyn.RequestInfo) error {
				if userMap, ok := requestInfo.Body.(map[string]interface{}); ok {
					// Add default phone and status for v2 -> v3 migration
					if _, hasPhone := userMap["phone"]; !hasPhone {
						userMap["phone"] = ""
					}
					if _, hasStatus := userMap["status"]; !hasStatus {
						userMap["status"] = "active"
					}
					requestInfo.Body = userMap
				}
				return nil
			},
		},
		&cadwyn.AlterResponseInstruction{
			Schemas: []interface{}{UserV3{}},
			Transformer: func(responseInfo *cadwyn.ResponseInfo) error {
				if userMap, ok := responseInfo.Body.(map[string]interface{}); ok {
					// Remove phone and status for v3 -> v2 migration
					delete(userMap, "phone")
					delete(userMap, "status")
					responseInfo.Body = userMap
				}
				return nil
			},
		},
		&cadwyn.SchemaInstruction{
			Schema: UserV3{},
			Name:   "phone",
			Type:   "field_added",
			Attributes: map[string]interface{}{
				"type":     "string",
				"required": false,
			},
		},
		&cadwyn.SchemaInstruction{
			Schema: UserV3{},
			Name:   "status",
			Type:   "field_added",
			Attributes: map[string]interface{}{
				"type":     "string",
				"required": true,
				"default":  "active",
			},
		},
	)
}
