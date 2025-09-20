# 🔄 Cadwyn-Go

**Stripe-like API versioning for Go applications**

Cadwyn-Go is a Go implementation inspired by [Python Cadwyn](https://github.com/zmievsa/cadwyn), providing automatic API versioning with a clean, instruction-based architecture.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## ✨ Features

- **🎯 Simple Architecture** - Clean, Python Cadwyn-inspired design
- **📅 Multiple Version Formats** - Date-based (`2023-01-15`) and semantic (`1.0`, `2.1`) versioning
- **🔄 Instruction-Based Migrations** - Transform requests and responses with clear instructions
- **🏗️ Builder Pattern** - Fluent API for easy configuration
- **🧪 Type-Safe** - Full Go type safety with compile-time checks
- **📦 Lightweight** - Minimal dependencies, focused on core functionality

## 🚀 Quick Start

### Installation

```bash
go get github.com/isaacchung/cadwyn-go
```

### Basic Usage

```go
package main

import (
    "fmt"
    "github.com/isaacchung/cadwyn-go/pkg/cadwyn"
    "github.com/isaacchung/cadwyn-go/pkg/migration"
)

// Define your models
type UserV1 struct {
    ID   int    `json:"id"`
    Name string `json:"name"`
}

type UserV2 struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

func main() {
    // Create Cadwyn app with versions
    app, err := cadwyn.NewBuilder().
        WithSemverVersions("1.0", "2.0").
        WithHeadVersion().
        Build()
    
    if err != nil {
        panic(err)
    }
    
    // Parse and work with versions
    v1, _ := app.ParseVersion("1.0")
    v2, _ := app.ParseVersion("2.0")
    head := app.GetHeadVersion()
    
    fmt.Printf("Versions: %s, %s, %s\n", v1, v2, head)
}
```

## 🏗️ Architecture

Cadwyn-Go follows the Python Cadwyn architecture with these core concepts:

### 📦 Version Bundle
Manages a collection of API versions:

```go
// Create versions
v1, _ := version.NewSemverVersion("1.0")
v2, _ := version.NewSemverVersion("2.0")
head := version.NewHeadVersion()

// Create bundle
bundle := version.NewVersionBundle([]*version.Version{v1, v2, head})
```

### 🔄 Version Changes
Define how to migrate between versions using instructions:

```go
// Request migration: v1 -> v2 (add email field)
requestInstruction := &migration.AlterRequestInstruction{
    Schemas: []interface{}{UserV1{}},
    Transformer: func(requestInfo *migration.RequestInfo) error {
        if userMap, ok := requestInfo.Body.(map[string]interface{}); ok {
            userMap["email"] = "default@example.com"
            requestInfo.Body = userMap
        }
        return nil
    },
}

// Response migration: v2 -> v1 (remove email field)
responseInstruction := &migration.AlterResponseInstruction{
    Schemas: []interface{}{UserV2{}},
    Transformer: func(responseInfo *migration.ResponseInfo) error {
        if userMap, ok := responseInfo.Body.(map[string]interface{}); ok {
            delete(userMap, "email")
            responseInfo.Body = userMap
        }
        return nil
    },
}
```

### 🎯 Instructions
Transform data between versions:

- **`AlterRequestInstruction`** - Modify incoming requests
- **`AlterResponseInstruction`** - Modify outgoing responses  
- **`SchemaInstruction`** - Define schema changes
- **`EndpointInstruction`** - Define endpoint changes

## 📚 Examples

### Basic Example
```bash
cd examples/basic && go run main.go
```

### Advanced Example  
```bash
cd examples/advanced && go run main.go
```

## 🔧 API Reference

### Builder Pattern

```go
app, err := cadwyn.NewBuilder().
    WithSemverVersions("1.0", "2.0", "3.0").
    WithHeadVersion().
    WithVersionChanges(change1, change2).
    Build()
```

### Version Creation

```go
// Semantic versions
v1, _ := version.NewSemverVersion("1.0")
v2, _ := version.NewSemverVersion("2.1")

// Date versions  
dateV1, _ := version.NewDateVersion("2023-01-15")
dateV2, _ := version.NewDateVersion("2023-06-01")

// Head version
head := version.NewHeadVersion()
```

### Migration Instructions

```go
// Schema-based migration
instruction := &migration.AlterRequestInstruction{
    Schemas: []interface{}{MyStruct{}},
    Transformer: func(info *migration.RequestInfo) error {
        // Transform the request
        return nil
    },
}

// Path-based migration
pathInstruction := migration.ConvertRequestToNextVersionForPath(
    "/api/users", 
    []string{"POST", "PUT"}, 
    transformerFunc,
)
```

## 🧪 Testing

Run all tests:
```bash
go test ./...
```

Run examples:
```bash
go run examples/basic/main.go
go run examples/advanced/main.go
```

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- [Python Cadwyn](https://github.com/zmievsa/cadwyn) - The original inspiration
- [Stripe API Versioning](https://stripe.com/blog/api-versioning) - API versioning best practices

## 🔗 Links

- [Python Cadwyn Documentation](https://cadwyn.dev/)
- [API Versioning Best Practices](https://blog.stoplight.io/api-versioning-best-practices)
- [Go Documentation](https://golang.org/doc/)

---

**Built with ❤️ for the Go community**