# Contributing to Cadwyn-Go

Thank you for your interest in contributing to Cadwyn-Go! 🎉

This document provides guidelines and instructions for contributing to this project.

## 📋 Table of Contents

- [Getting Started](#getting-started)
- [Development Setup](#development-setup)
- [Making Changes](#making-changes)
- [Testing](#testing)
- [Code Style](#code-style)
- [Submitting Changes](#submitting-changes)
- [Reporting Issues](#reporting-issues)
- [Community Guidelines](#community-guidelines)

## 🚀 Getting Started

### Prerequisites

- **Go 1.23+** - We support multiple Go versions, but use the latest stable version for development
- **Git** - For version control
- **GitHub account** - For submitting pull requests

### Development Setup

1. **Fork the repository** on GitHub
2. **Clone your fork** locally:
   ```bash
   git clone https://github.com/YOUR_USERNAME/cadwyn-go.git
   cd cadwyn-go
   ```
3. **Add the upstream repository**:
   ```bash
   git remote add upstream https://github.com/astronomer/cadwyn-go.git
   ```
4. **Install dependencies**:
   ```bash
   go mod download
   ```
5. **Verify your setup** by running tests:
   ```bash
   go test ./...
   ```

## 🔧 Making Changes

### Branching Strategy

1. **Create a new branch** from `main` for your changes:
   ```bash
   git checkout main
   git pull upstream main
   git checkout -b feature/your-feature-name
   ```

2. **Use descriptive branch names**:
   - `feature/add-middleware-caching`
   - `fix/version-parsing-bug`
   - `docs/improve-readme-examples`

### Code Organization

- **Core logic**: `cadwyn/` directory
- **Examples**: `examples/` directory
- **Tests**: Use `*_test.go` files alongside the code they test
- **Documentation**: Update README.md and code comments as needed

### Commit Messages

Use clear, descriptive commit messages:

```
feat: add caching middleware for version detection
fix: resolve panic in semver parsing with invalid input
docs: add example for custom version formats
test: improve coverage for migration chain
refactor: simplify version bundle creation logic
```

**Format**: `type: description`

**Types**:
- `feat`: New feature
- `fix`: Bug fix  
- `docs`: Documentation changes
- `test`: Test changes
- `refactor`: Code refactoring
- `perf`: Performance improvements
- `chore`: Maintenance tasks

## 🧪 Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -race -coverprofile=coverage.out -covermode=atomic ./...

# Run tests for a specific package
go test ./cadwyn

# Run a specific test
go test ./cadwyn -run TestVersionParsing
```

### Test Requirements

- **Add tests** for new functionality
- **Update tests** when modifying existing functionality
- **Maintain or improve** test coverage
- **Test edge cases** and error conditions
- **Write clear test names** that describe what they test

### Example Validation

Ensure all examples compile and run:

```bash
go run validate.go
```

## 🎨 Code Style

### Go Standards

- Follow standard Go conventions and idioms
- Use `gofmt` to format code
- Use meaningful variable and function names
- Add comments for exported functions and types
- Follow the [Effective Go](https://golang.org/doc/effective_go.html) guide

### Linting

We use `golangci-lint` for code quality. Run it locally:

```bash
# Install golangci-lint
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Run linting
golangci-lint run
```

### Documentation

- **Public APIs**: Must have clear godoc comments
- **Complex logic**: Add inline comments explaining the "why"
- **Examples**: Include runnable examples in documentation
- **README**: Update if your changes affect usage

## 📤 Submitting Changes

### Before Submitting

1. **Run tests**: `go test ./...`
2. **Run linting**: `golangci-lint run`
3. **Validate examples**: `go run validate.go`
4. **Update documentation** if needed
5. **Write clear commit messages**

### Pull Request Process

1. **Push your changes** to your fork:
   ```bash
   git push origin your-branch-name
   ```

2. **Create a Pull Request** on GitHub with:
   - Clear title describing the change
   - Detailed description of what and why
   - Link to related issues (if applicable)
   - Screenshots/examples (if applicable)

3. **Fill out the PR template** completely

4. **Be responsive** to review feedback

### PR Guidelines

- **One feature per PR** - Keep changes focused
- **Small, reviewable PRs** - Easier to review and merge
- **Update tests** and documentation
- **Resolve merge conflicts** before requesting review
- **Pass all CI checks**

## 🐛 Reporting Issues

### Before Reporting

1. **Search existing issues** to avoid duplicates
2. **Check the documentation** and examples
3. **Test with the latest version**

### Bug Reports

Use the bug report template and include:
- Clear description of the issue
- Steps to reproduce
- Expected vs actual behavior
- Minimal code example
- Environment information
- Error logs/stack traces

### Feature Requests

Use the feature request template and include:
- Problem statement
- Proposed solution
- Use cases and examples
- Alternative solutions considered

## 🚀 Development Tips

### Running Examples

```bash
# Basic example
cd examples/basic && go run main.go

# Advanced example  
cd examples/advanced && go run main.go

# Gin server example
cd examples/gin_server && go run main.go
```

---

Thank you for contributing to Cadwyn-Go! Every contribution, no matter how small, helps make the project better for everyone. 🎉
