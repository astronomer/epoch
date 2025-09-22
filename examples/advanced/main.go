package main

import (
	"fmt"
	"strings"

	"github.com/gin-gonic/gin"
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

// Advanced example showing complex version changes and migrations
func main() {
	gin.SetMode(gin.ReleaseMode) // Quiet Gin for cleaner output

	fmt.Println("🚀 Cadwyn-Go - Advanced Example")
	fmt.Println("Complex Version Changes & Migration Instructions")
	fmt.Println(strings.Repeat("=", 60))

	// Create versions (like Python Cadwyn)
	v1, _ := cadwyn.NewVersion("1.0")
	v2, _ := cadwyn.NewVersion("2.0")
	head := cadwyn.NewHeadVersion()

	// Create version changes with instructions (like Python Cadwyn)
	v1ToV2Change := createV1ToV2Change(v1, v2)

	// Add changes to versions
	v2.Changes = []cadwyn.VersionChangeInterface{v1ToV2Change}

	// Create version bundle
	bundle := cadwyn.NewVersionBundle([]*cadwyn.Version{v1, v2, head})

	fmt.Printf("✅ Created version bundle with %d versions\n", len(bundle.GetVersions()))
	fmt.Printf("✅ Head version: %s\n", bundle.GetHeadVersion().String())

	// Demonstrate the new architecture
	demonstrateNewArchitecture(bundle)

	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("🎉 Advanced Example Complete!")
	fmt.Println("📚 Advanced Features Demonstrated:")
	fmt.Println("   • Complex version change instructions")
	fmt.Println("   • Schema-based request/response migrations")
	fmt.Println("   • Version bundle management")
	fmt.Println("   • Instruction-based transformation pipeline")
}

// createV1ToV2Change creates a version change using the new instruction-based approach
func createV1ToV2Change(from, to *cadwyn.Version) *SimpleVersionChange {
	// Create request migration instruction
	requestInstruction := &cadwyn.AlterRequestInstruction{
		Schemas: []interface{}{UserV1{}},
		Transformer: func(requestInfo *cadwyn.RequestInfo) error {
			if userMap, ok := requestInfo.Body.(map[string]interface{}); ok {
				// Add email field if it doesn't exist
				if _, hasEmail := userMap["email"]; !hasEmail {
					userMap["email"] = "default@example.com"
				}
				requestInfo.Body = userMap
			}
			return nil
		},
	}

	// Create response migration instruction
	responseInstruction := &cadwyn.AlterResponseInstruction{
		Schemas: []interface{}{UserV2{}},
		Transformer: func(responseInfo *cadwyn.ResponseInfo) error {
			// For v1 clients, remove the email field from responses
			if userMap, ok := responseInfo.Body.(map[string]interface{}); ok {
				delete(userMap, "email")
				responseInfo.Body = userMap
			}
			return nil
		},
	}

	return &SimpleVersionChange{
		description:         "v1→v2: Add email field",
		fromVersion:         from,
		toVersion:           to,
		requestInstruction:  requestInstruction,
		responseInstruction: responseInstruction,
	}
}

// SimpleVersionChange implements the new architecture
type SimpleVersionChange struct {
	description         string
	fromVersion         *cadwyn.Version
	toVersion           *cadwyn.Version
	requestInstruction  *cadwyn.AlterRequestInstruction
	responseInstruction *cadwyn.AlterResponseInstruction
}

func (svc *SimpleVersionChange) Description() string {
	return svc.description
}

func (svc *SimpleVersionChange) FromVersion() *cadwyn.Version {
	return svc.fromVersion
}

func (svc *SimpleVersionChange) ToVersion() *cadwyn.Version {
	return svc.toVersion
}

func (svc *SimpleVersionChange) GetSchemaInstructions() interface{} {
	return []interface{}{} // Placeholder
}

func (svc *SimpleVersionChange) GetEnumInstructions() interface{} {
	return []interface{}{} // Placeholder
}

func demonstrateNewArchitecture(bundle *cadwyn.VersionBundle) {
	fmt.Println("\n🧪 Testing Advanced Features:")

	// Test version parsing
	fmt.Println("\n1. 📋 Version Parsing:")
	testVersions := []string{"1.0", "2.0", "head", "invalid"}
	for _, vStr := range testVersions {
		if v, err := bundle.ParseVersion(vStr); err == nil {
			fmt.Printf("   ✅ %s -> %s (Type: %s)\n", vStr, v.String(), v.Type.String())
		} else {
			fmt.Printf("   ❌ %s -> Error: %s\n", vStr, err.Error())
		}
	}

	// Test version bundle functionality
	fmt.Println("\n2. 📦 Version Bundle:")
	fmt.Printf("   • Total versions: %d\n", len(bundle.GetVersions()))
	fmt.Printf("   • Head version: %s\n", bundle.GetHeadVersion().String())
	fmt.Printf("   • Version values: %v\n", bundle.GetVersionValues())

	// Test instruction-based migrations
	fmt.Println("\n3. 🔄 Instruction-Based Migrations:")
	testMigrations(bundle)
}

func testMigrations(bundle *cadwyn.VersionBundle) {
	// Simulate a request migration
	fmt.Println("   📥 Request Migration (v1.0 -> v2.0):")

	// Original v1 request (missing email)
	originalRequest := map[string]interface{}{
		"id":   1,
		"name": "John Doe",
	}

	fmt.Printf("      Before: %+v\n", originalRequest)

	// Find the version change
	v2 := bundle.GetVersions()[1] // v2.0
	if len(v2.Changes) > 0 {
		change := v2.Changes[0].(*SimpleVersionChange)

		// Create request info
		requestInfo := &cadwyn.RequestInfo{
			Body: originalRequest,
		}

		// Apply migration
		if err := change.requestInstruction.Transformer(requestInfo); err == nil {
			fmt.Printf("      After:  %+v\n", requestInfo.Body)
			fmt.Printf("      ✅ Email field added successfully\n")
		} else {
			fmt.Printf("      ❌ Migration failed: %s\n", err.Error())
		}
	}

	// Simulate a response migration
	fmt.Println("   📤 Response Migration (v2.0 -> v1.0):")

	// Original v2 response (with email)
	originalResponse := map[string]interface{}{
		"id":    1,
		"name":  "John Doe",
		"email": "john@example.com",
	}

	fmt.Printf("      Before: %+v\n", originalResponse)

	// Apply reverse migration
	if len(v2.Changes) > 0 {
		change := v2.Changes[0].(*SimpleVersionChange)

		// Create response info
		responseInfo := &cadwyn.ResponseInfo{
			Body: originalResponse,
		}

		// Apply migration
		if err := change.responseInstruction.Transformer(responseInfo); err == nil {
			fmt.Printf("      After:  %+v\n", responseInfo.Body)
			fmt.Printf("      ✅ Email field removed for v1 client\n")
		} else {
			fmt.Printf("      ❌ Migration failed: %s\n", err.Error())
		}
	}
}
