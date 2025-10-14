# 🧪 Testing Guide for Epoch

This document provides comprehensive information about testing in the Epoch project.

## 📋 Overview

Epoch uses [Ginkgo](https://onsi.github.io/ginkgo/) as the BDD testing framework with [Gomega](https://onsi.github.io/gomega/) for assertions. The test suite provides comprehensive coverage of all core functionality.

## 🚀 Quick Start

### Prerequisites

1. **Go 1.21+** - Required for running tests
2. **Ginkgo CLI** (optional but recommended):
   ```bash
   go install github.com/onsi/ginkgo/v2/ginkgo@latest
   ```

### Running Tests

```bash
# Quick test run
make test

# Run with Ginkgo (better output)
make test-ginkgo

# Run only unit tests
make test-unit

# Validate examples
make test-examples

# Generate coverage report
make coverage

# Full test suite with script
./scripts/test.sh --coverage
```

## 📁 Test Structure

```
epoch/
├── version_test.go               # Version creation and comparison
├── version_bundle_test.go        # Version bundle management
├── epoch_test.go                 # Main API and builder pattern
├── middleware_test.go            # Version detection middleware
├── version_change_test.go        # Migration logic
├── request_response_info_test.go # Request/response helper methods
├── ast_helpers_test.go           # AST helper functions and edge cases
├── migration_instructions_test.go # Migration instruction types
├── schema_generator_test.go      # AST and schema generation
├── integration_test.go           # End-to-end integration and order preservation
├── epoch_suite_test.go           # Test suite configuration
└── ginkgo.config                 # Ginkgo configuration
```

## 🎯 Test Categories

### Unit Tests
- **Version Management**: Creation, parsing, comparison of different version types
- **Builder Pattern**: Fluent API construction and validation
- **AST Helper Functions**: Field manipulation, type checking, and error handling
- **Request/Response Info**: Convenience methods for JSON data access
- **Migration Instructions**: Transformation logic and instruction types
- **Schema Generation**: AST parsing and Go code generation

### Integration Tests
- **Middleware Integration**: Version detection and context handling
- **Request/Response Migration**: Automatic data transformation with Sonic AST
- **JSON Field Order Preservation**: Verification that Sonic maintains field order
- **End-to-End Workflows**: Complete API versioning scenarios
- **Error Response Handling**: Migration behavior with HTTP errors

### Performance & Edge Case Tests
- **Sonic vs Standard JSON**: Order preservation comparison tests
- **Complex Nested Structures**: Deep object and array handling
- **Edge Cases**: Null values, empty objects, type conversion errors
- **Memory & Performance**: Large JSON structures and concurrent access

## 🔧 Make Targets

| Target | Description |
|--------|-------------|
| `make test` | Run all tests with coverage |
| `make test-ginkgo` | Run tests with Ginkgo (better output) |
| `make test-unit` | Run only unit tests |
| `make test-examples` | Validate example code compilation |
| `make coverage` | Generate HTML coverage report |
| `make validate-fmt` | Check code formatting |

## 🏗️ Test Configuration

### Ginkgo Configuration (`epoch/ginkgo.config`)
- Race detection enabled
- Coverage reporting with atomic mode
- Verbose output
- Test randomization for reliability
- 10-minute timeout per test

### Makefile Integration
- Automatic fallback from Ginkgo to `go test`
- Coverage report generation
- Example validation
- Code formatting checks

## 🔄 Continuous Integration

### GitHub Actions Workflows

#### Main CI (`ci.yml`)
- **Build and Test**: Primary test execution with Ginkgo
- **Multi-platform Matrix**: Tests on Linux, Windows, macOS
- **Multi-version Matrix**: Tests on Go 1.21, 1.22, 1.23
- **Lint**: Code formatting and quality checks
- **Coverage Upload**: Codecov integration

#### Coverage Workflow (`coverage.yml`)
- Dedicated coverage report generation
- Artifact uploads for coverage files
- PR comments with coverage summaries
- Codecov integration with detailed reporting

### Status Badges
- Build status
- Coverage percentage  
- Go version compatibility
- License information
- Go Report Card score

## 🧩 Writing Tests

### Test Structure Example

```go
var _ = Describe("ComponentName", func() {
    var (
        component *ComponentType
        // other variables
    )

    BeforeEach(func() {
        // Setup before each test
        component = NewComponent()
    })

    Describe("MethodName", func() {
        Context("with valid input", func() {
            It("should perform expected behavior", func() {
                result, err := component.MethodName("valid-input")
                Expect(err).NotTo(HaveOccurred())
                Expect(result).To(Equal("expected-output"))
            })
        })

        Context("with invalid input", func() {
            It("should return appropriate error", func() {
                _, err := component.MethodName("invalid-input")
                Expect(err).To(HaveOccurred())
                Expect(err.Error()).To(ContainSubstring("expected error"))
            })
        })
    })
})
```

### Testing Migration Transformers

When writing tests for migration transformers, use the helper methods:

```go
It("should transform user data correctly", func() {
    // Create test data using Sonic
    userJSON := `{"name": "John", "age": 30}`
    userNode, err := sonic.Get([]byte(userJSON))
    Expect(err).NotTo(HaveOccurred())
    err = userNode.Load()
    Expect(err).NotTo(HaveOccurred())
    
    requestInfo := epoch.NewRequestInfo(c, &userNode)
    
    // Test transformation
    err = transformer(requestInfo)
    Expect(err).NotTo(HaveOccurred())
    
    // Verify results using helper methods
    Expect(requestInfo.HasField("email")).To(BeTrue())
    email, err := requestInfo.GetFieldString("email")
    Expect(err).NotTo(HaveOccurred())
    Expect(email).To(Equal("default@example.com"))
})
```

### Testing Field Order Preservation

```go
It("should preserve field order in responses", func() {
    // Test with non-alphabetical order
    originalJSON := `{"zebra": 1, "alpha": 2, "middle": 3}`
    
    // Process through Epoch middleware
    // ... (setup middleware and make request)
    
    responseBody := recorder.Body.String()
    
    // Verify order is preserved
    zebraPos := strings.Index(responseBody, `"zebra"`)
    alphaPos := strings.Index(responseBody, `"alpha"`)
    Expect(zebraPos).To(BeNumerically("<", alphaPos))
})
```

## 📊 Coverage Goals

- **Target**: 80%+ overall coverage
- **Critical Paths**: 90%+ coverage for core functionality  
- **New Features**: 100% coverage required for new code
- **Current Status**: 282 tests covering all major functionality

### Test Statistics
- **Total Test Files**: 9 focused test suites
- **Unit Tests**: ~190 tests covering individual components
- **Integration Tests**: ~90 tests covering end-to-end scenarios
- **Edge Case Tests**: Comprehensive error handling and boundary conditions
- **Order Preservation Tests**: Sonic AST functionality validation

## 🚀 Local Development

### Running Tests During Development

```bash
# Watch mode (if using ginkgo)
ginkgo watch ./epoch

# Run specific test
ginkgo -focus "Version creation" ./epoch

# Run tests with specific labels
ginkgo --label-filter="unit" ./epoch

# Debug mode
ginkgo -v --trace ./epoch
```

### Pre-commit Checks

The `scripts/test.sh` script provides a comprehensive pre-commit check:

```bash
# Full validation
./scripts/test.sh

# With coverage report
./scripts/test.sh --coverage
```

## 🔍 Debugging Tests

### Common Issues

1. **Import Cycles**: Ensure test files use correct package names
2. **Missing Dependencies**: Run `go mod tidy` to install test dependencies
3. **Race Conditions**: Use proper synchronization in concurrent tests
4. **Test Isolation**: Ensure tests don't interfere with each other

### Debug Commands

```bash
# Verbose test output
go test -v ./epoch

# Race detection
go test -race ./epoch

# CPU profiling
go test -cpuprofile=cpu.prof ./epoch

# Memory profiling
go test -memprofile=mem.prof ./epoch
```

## 🎉 Contributing Tests

When contributing new features:

1. **Write tests first** (TDD approach recommended)
2. **Test both happy and error paths**
3. **Include integration tests** for component interactions
4. **Update this documentation** if adding new test patterns
5. **Ensure CI passes** before submitting PR

## 📚 Resources

- [Ginkgo Documentation](https://onsi.github.io/ginkgo/)
- [Gomega Matchers](https://onsi.github.io/gomega/)
- [Go Testing Package](https://pkg.go.dev/testing)
- [Codecov Documentation](https://docs.codecov.com/)

---

For questions about testing, please check the existing tests for examples or open an issue for clarification.
