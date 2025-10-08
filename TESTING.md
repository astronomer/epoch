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
├── version_test.go              # Version creation and comparison
├── version_bundle_test.go       # Version bundle management
├── epoch_test.go                # Main API and builder pattern
├── middleware_test.go           # Version detection middleware
├── version_change_test.go       # Migration logic
├── migration_types_test.go      # Request/response migration
├── schema_generator_test.go     # AST and schema generation
├── router_test.go               # Versioned routing
├── route_generator_test.go      # Route transformation
├── gin_application_test.go      # Application setup
├── epoch_suite_test.go          # Test suite configuration
└── ginkgo.config                # Ginkgo configuration
```

## 🎯 Test Categories

### Unit Tests
- **Version Management**: Creation, parsing, comparison of different version types
- **Builder Pattern**: Fluent API construction and validation
- **Schema Generation**: AST parsing and Go code generation
- **Type Registry**: Type introspection and metadata management

### Integration Tests
- **Middleware Integration**: Version detection and context handling
- **Request/Response Migration**: Automatic data transformation
- **Route Management**: Versioned routing and handler wrapping
- **Application Setup**: Full application configuration and startup

### HTTP Tests
- **Gin Middleware**: HTTP request/response handling
- **Version Detection**: Header, query, and path-based version extraction
- **Handler Wrapping**: Version-aware request processing
- **Utility Endpoints**: Health checks, version info, documentation

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

### Best Practices

1. **Descriptive Test Names**: Use clear, behavior-focused descriptions
2. **Proper Setup/Teardown**: Use `BeforeEach` and `AfterEach` appropriately
3. **Context Grouping**: Group related test cases with `Context`
4. **Error Testing**: Always test both success and failure scenarios
5. **Isolation**: Each test should be independent and not rely on others

## 📊 Coverage Goals

- **Target**: 80%+ overall coverage
- **Critical Paths**: 90%+ coverage for core functionality
- **New Features**: 100% coverage required for new code

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
