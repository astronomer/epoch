package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// validationTest represents a test to run
type validationTest struct {
	name        string
	command     string
	description string
}

// Define all validation tests
var tests = []validationTest{
	{
		name:        "Core Package Tests",
		command:     "go test ./pkg/...",
		description: "Run all unit tests for core packages",
	},
	{
		name:        "Build Verification",
		command:     "go build ./...",
		description: "Verify all packages compile successfully",
	},
	{
		name:        "Basic Example",
		command:     "go run examples/basic/main.go",
		description: "Getting started with API versioning",
	},
	{
		name:        "Advanced Example",
		command:     "go run examples/advanced/main.go",
		description: "Complex version changes and migrations",
	},
}

func main() {
	fmt.Println("🧪 Cadwyn-Go Validation Suite")
	fmt.Println("Testing Clean Architecture")
	fmt.Println(strings.Repeat("=", 60))

	totalTests := len(tests)
	passedTests := 0
	failedTests := []string{}

	for i, test := range tests {
		fmt.Printf("\n[%d/%d] %s\n", i+1, totalTests, test.name)
		fmt.Printf("📝 %s\n", test.description)
		fmt.Printf("🔧 Running: %s\n", test.command)

		// Parse command
		parts := strings.Fields(test.command)
		cmd := exec.Command(parts[0], parts[1:]...)

		// Run command and capture output
		output, err := cmd.CombinedOutput()

		if err != nil {
			fmt.Printf("❌ FAILED: %v\n", err)
			if len(output) > 0 {
				fmt.Printf("📄 Output:\n%s\n", string(output))
			}
			failedTests = append(failedTests, test.name)
		} else {
			fmt.Printf("✅ PASSED\n")
			passedTests++

			// Show output for examples (they're meant to be seen)
			if strings.Contains(test.command, "examples/") && len(output) > 0 {
				fmt.Printf("📄 Output:\n%s\n", string(output))
			}
		}
	}

	// Print summary
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("📊 VALIDATION SUMMARY\n")
	fmt.Printf("✅ Passed: %d/%d tests\n", passedTests, totalTests)

	if len(failedTests) > 0 {
		fmt.Printf("❌ Failed: %d/%d tests\n", len(failedTests), totalTests)
		fmt.Println("Failed tests:")
		for _, testName := range failedTests {
			fmt.Printf("   • %s\n", testName)
		}
		os.Exit(1)
	} else {
		fmt.Printf("🎉 All tests passed! Clean architecture is working perfectly.\n")
	}
}
