package main

import (
	"fmt"
	"strings"

	"github.com/isaacchung/cadwyn-go/cadwyn"
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

func main() {
	fmt.Println("🚀 Cadwyn-Go - Basic Example")
	fmt.Println("Getting Started with API Versioning")
	fmt.Println(strings.Repeat("=", 50))

	// Create versions using the simple builder
	cadwynInstance, err := cadwyn.NewCadwyn().
		WithSemverVersions("1.0", "2.0").
		WithHeadVersion().
		WithTypes(UserV1{}, UserV2{}).
		Build()

	if err != nil {
		fmt.Printf("❌ Error creating Cadwyn instance: %v\n", err)
		return
	}

	fmt.Printf("✅ Created Cadwyn instance with %d versions\n", len(cadwynInstance.GetVersions()))

	// Demonstrate version parsing
	fmt.Println("\n📋 Version Parsing:")
	testVersions := []string{"1.0", "2.0", "head", "invalid"}
	for _, vStr := range testVersions {
		if v, err := cadwynInstance.ParseVersion(vStr); err == nil {
			fmt.Printf("   ✅ %s -> %s (Type: %s)\n", vStr, v.String(), v.Type.String())
		} else {
			fmt.Printf("   ❌ %s -> Error: %s\n", vStr, err.Error())
		}
	}

	// Show version information
	fmt.Println("\n📦 Version Information:")
	fmt.Printf("   • Head version: %s\n", cadwynInstance.GetHeadVersion().String())
	fmt.Printf("   • All versions: ")
	for i, v := range cadwynInstance.GetVersions() {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Print(v.String())
	}
	fmt.Println()

	// Demonstrate instruction-based migrations
	fmt.Println("\n🔄 Migration Instructions:")
	demonstrateMigrations()

	// Show schema generation
	fmt.Println("\n🏗️  Schema Generation:")
	demonstrateSchemaGeneration(cadwynInstance)

	fmt.Println("\n" + strings.Repeat("=", 50))
	fmt.Println("🎉 Basic Example Complete!")
	fmt.Println("📚 Next Steps:")
	fmt.Println("   • Check out examples/gin_server/ for a full web server")
	fmt.Println("   • Check out examples/advanced/ for complex migrations")
	fmt.Println("   • Start building your versioned API!")
}

func demonstrateMigrations() {
	// Create a simple migration instruction
	requestInstruction := &cadwyn.AlterRequestInstruction{
		Schemas: []interface{}{UserV1{}},
		Transformer: func(requestInfo *cadwyn.RequestInfo) error {
			if userMap, ok := requestInfo.Body.(map[string]interface{}); ok {
				// Add email field for v1 -> v2 migration
				if _, hasEmail := userMap["email"]; !hasEmail {
					userMap["email"] = "migrated@example.com"
					fmt.Printf("   📥 Added email field to request: %+v\n", userMap)
				}
				requestInfo.Body = userMap
			}
			return nil
		},
	}

	responseInstruction := &cadwyn.AlterResponseInstruction{
		Schemas: []interface{}{UserV2{}},
		Transformer: func(responseInfo *cadwyn.ResponseInfo) error {
			if userMap, ok := responseInfo.Body.(map[string]interface{}); ok {
				// Remove email field for v2 -> v1 migration
				delete(userMap, "email")
				fmt.Printf("   📤 Removed email field from response: %+v\n", userMap)
				responseInfo.Body = userMap
			}
			return nil
		},
	}

	// Test request migration
	fmt.Println("   Testing Request Migration (v1 -> v2):")
	requestInfo := &cadwyn.RequestInfo{
		Body: map[string]interface{}{
			"id":   1,
			"name": "John Doe",
		},
	}
	fmt.Printf("      Before: %+v\n", requestInfo.Body)
	requestInstruction.Transformer(requestInfo)

	// Test response migration
	fmt.Println("   Testing Response Migration (v2 -> v1):")
	responseInfo := &cadwyn.ResponseInfo{
		Body: map[string]interface{}{
			"id":    1,
			"name":  "John Doe",
			"email": "john@example.com",
		},
	}
	fmt.Printf("      Before: %+v\n", responseInfo.Body)
	responseInstruction.Transformer(responseInfo)
}

func demonstrateSchemaGeneration(cadwynInstance *cadwyn.Cadwyn) {
	// Generate struct for different versions
	versions := []string{"1.0", "2.0", "head"}

	for _, version := range versions {
		fmt.Printf("   Generating UserV2 for version %s:\n", version)
		if generatedCode, err := cadwynInstance.GenerateStructForVersion(UserV2{}, version); err == nil {
			// Show just the first few lines to keep output manageable
			lines := strings.Split(generatedCode, "\n")
			maxLines := 5
			if len(lines) > maxLines {
				lines = lines[:maxLines]
				lines = append(lines, "      ... (truncated)")
			}
			for _, line := range lines {
				if strings.TrimSpace(line) != "" {
					fmt.Printf("      %s\n", line)
				}
			}
		} else {
			fmt.Printf("      ❌ Error: %s\n", err.Error())
		}
		fmt.Println()
	}
}
