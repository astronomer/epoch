# 🔄 Cadwyn-Go

**Stripe-like API versioning for Go applications**

Cadwyn-Go is a Go implementation inspired by [Python Cadwyn](https://github.com/zmievsa/cadwyn), providing automatic API versioning with a clean, instruction-based architecture. **Now with a simplified approach that works with your existing Gin applications!**

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.19-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## ✨ Features

- **🎯 Simple & Flexible** - Add versioning to existing Gin apps with minimal changes
- **📅 Multiple Version Formats** - Date-based (`2023-01-15`), semantic (`1.0.0`, `2.1`), and string (`alpha`) versioning
- **🔄 Instruction-Based Migrations** - Transform requests and responses with clear instructions
- **🏗️ Builder Pattern** - Fluent API for easy configuration
- **🧪 Type-Safe** - Full Go type safety with compile-time checks
- **📦 Lightweight** - Minimal dependencies, focused on core functionality
- **🌐 Gin Integration** - Works with your existing Gin setup
- **🔧 Schema Generation** - Generate version-specific Go structs
- **⚡ Version Detection Middleware** - Automatic version detection from headers/query/path
- **🎛️ Selective Versioning** - Only version the endpoints that need it

## 🚀 Quick Start

### Installation

```bash
go get github.com/astronomer/cadwyn-go
```

### Basic Usage with Existing Gin App

```go
package main

import (
    "github.com/gin-gonic/gin"
    "github.com/astronomer/cadwyn-go/cadwyn"
)

// Define your API models
type CreateUserRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
    Phone string `json:"phone,omitempty"` // Added in v2.0
}

type UserResponse struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
    Phone string `json:"phone,omitempty"` // Added in v2.0
}

func main() {
    // Create your Gin app as usual
    r := gin.Default()
    
    // Add your own middleware
    r.Use(gin.Logger())
    r.Use(gin.Recovery())
    
    // Create Cadwyn with just the versioning logic
    cadwynInstance, err := cadwyn.NewCadwyn().
        WithDateVersions("2023-01-01", "2024-01-01").
        WithHeadVersion().
        WithTypes(CreateUserRequest{}, UserResponse{}).
        WithVersionLocation(cadwyn.VersionLocationHeader).
        Build()
    
    if err != nil {
        panic(err)
    }
    
    // Apply Cadwyn's version detection middleware
    r.Use(cadwynInstance.Middleware())
    
    // Wrap only the handlers that need versioning
    r.GET("/users", cadwynInstance.WrapHandler(getUsersHandler))
    r.POST("/users", cadwynInstance.WrapHandler(createUserHandler))
    
    // Regular handlers don't need versioning
    r.GET("/health", healthHandler)
    
    r.Run(":8080")
}

func getUsersHandler(c *gin.Context) {
    // Get version from context (set by Cadwyn middleware)
    version := cadwyn.GetVersionFromContext(c)
    
    // Your handler logic with version-aware responses
    users := getUsers()
    c.JSON(200, transformUsersForVersion(users, version))
}
```

## 🎯 Key Benefits of the New Approach

### ✅ **Use Your Existing Gin Setup**
```go
// You control your Gin engine
r := gin.New() // or gin.Default()
r.Use(yourCustomMiddleware())
r.Use(cors.Default())

// Just add versioning where needed
r.Use(cadwyn.Middleware())
r.GET("/api/users", cadwyn.WrapHandler(handler))
```

### ✅ **Selective Versioning**
```go
// Version only what needs it
r.GET("/api/users", cadwyn.WrapHandler(getUsersHandler))     // ✅ Versioned
r.GET("/api/posts", cadwyn.WrapHandler(getPostsHandler))     // ✅ Versioned
r.GET("/health", healthHandler)                              // ❌ Not versioned
r.GET("/metrics", metricsHandler)                            // ❌ Not versioned
```

### ✅ **Minimal Configuration**
```go
// Simple builder - just versions, changes, and types
cadwyn := cadwyn.NewCadwyn().
    WithVersions(v1, v2).
    WithChanges(userChange).
    WithTypes(UserRequest{}, UserResponse{}).
    Build()
```

## 📚 Examples

### Gin Server Example

```bash
cd examples/gin_server
go run main.go
```

Test with different versions:

```bash
# Version 2023-01-01 (no phone field)
curl -H "X-API-Version: 2023-01-01" http://localhost:8080/users

# Version 2024-01-01 (with phone field)  
curl -H "X-API-Version: 2024-01-01" http://localhost:8080/users

# Latest version (no header)
curl http://localhost:8080/users
```

### Basic Example

```bash
cd examples/basic
go run main.go
```

### Advanced Example

```bash
cd examples/advanced  
go run main.go
```

## 🏗️ Core Concepts

### Version Types

Cadwyn supports multiple version formats to suit different needs:

#### 📅 **Date-Based Versions**
```go
cadwyn := cadwyn.NewCadwyn().
    WithDateVersions("2023-01-01", "2023-06-15", "2024-01-01").
    WithHeadVersion().
    Build()

// Or using convenience function
cadwyn := cadwyn.QuickStart("2023-01-01", "2024-01-01")
```

#### 🔢 **Semantic Versions (supports both major.minor.patch and major.minor)**
```go
cadwyn := cadwyn.NewCadwyn().
    WithSemverVersions("1.0.0", "1.2", "2.0.0").  // Mix of full and short semver
    WithHeadVersion().
    Build()

// Or using convenience function
cadwyn := cadwyn.WithSemver("1.0", "2.0")  // Both major.minor and major.minor.patch work
```

#### 📝 **String Versions**
```go
cadwyn := cadwyn.NewCadwyn().
    WithStringVersions("alpha", "beta", "stable").
    WithHeadVersion().
    Build()

// Or using convenience function
cadwyn := cadwyn.WithStrings("alpha", "beta", "stable")
```

#### 🏁 **Head Version**
```go
// Always represents the latest version
cadwyn := cadwyn.NewCadwyn().
    WithHeadVersion().
    Build()

// Or using convenience function
cadwyn := cadwyn.Simple()
```

### Version Changes

Define how your API evolves between versions:

```go
change := cadwyn.NewVersionChange(
    fromVersion,
    toVersion,
    "Add phone field to User",
    []cadwyn.MigrationInstruction{
        &cadwyn.AlterRequestInstruction{
            Schemas: []interface{}{CreateUserRequest{}},
            Transformer: func(req *cadwyn.RequestInfo) error {
                // Transform request from old to new version
                return nil
            },
        },
        &cadwyn.AlterResponseInstruction{
            Schemas: []interface{}{UserResponse{}},
            Transformer: func(resp *cadwyn.ResponseInfo) error {
                // Transform response from new to old version
                return nil
            },
        },
    },
)
```

### Version Detection

Cadwyn can detect versions from:

- **Headers**: `X-API-Version: 2024-01-01`
- **Query parameters**: `?version=2024-01-01`  
- **Path**: `/v2024-01-01/users`

```go
cadwyn := cadwyn.NewCadwyn().
    WithVersionLocation(cadwyn.VersionLocationHeader).  // or Query, Path
    WithVersionParameter("X-API-Version").              // custom header/param name
    Build()
```

## 📖 API Reference

### CadwynBuilder

- `NewCadwyn()` - Create a new builder
- `WithVersions(...*Version)` - Add versions
- `WithDateVersions(...string)` - Add date-based versions
- `WithSemverVersions(...string)` - Add semantic versions (supports both major.minor.patch and major.minor)
- `WithStringVersions(...string)` - Add string-based versions
- `WithHeadVersion()` - Add head version
- `WithChanges(...*VersionChange)` - Add version changes
- `WithTypes(...interface{})` - Register types for schema generation
- `WithVersionLocation(VersionLocation)` - Set version detection location
- `WithVersionParameter(string)` - Set version parameter name
- `WithVersionFormat(VersionFormat)` - Set version format
- `WithDefaultVersion(*Version)` - Set default version
- `Build() (*Cadwyn, error)` - Build the Cadwyn instance

### Cadwyn

- `Middleware() gin.HandlerFunc` - Version detection middleware
- `WrapHandler(gin.HandlerFunc) gin.HandlerFunc` - Wrap handler for versioning
- `GetVersions() []*Version` - Get all versions
- `ParseVersion(string) (*Version, error)` - Parse version string
- `GenerateStructForVersion(interface{}, string) (string, error)` - Generate version-specific struct

### Convenience Functions

- `cadwyn.QuickStart(dates ...string)` - Quick setup with date versions
- `cadwyn.WithSemver(semvers ...string)` - Quick setup with semantic versions
- `cadwyn.WithStrings(versions ...string)` - Quick setup with string versions
- `cadwyn.Simple()` - Simple setup with just head version

### Version Helpers

- `cadwyn.DateVersion(date string)` - Create date version
- `cadwyn.SemverVersion(semver string)` - Create semantic version (supports both formats)
- `cadwyn.StringVersion(version string)` - Create string version
- `cadwyn.HeadVersion()` - Create head version

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Validate examples compile
go run validate.go
```

## 📝 License

MIT License - see [LICENSE](LICENSE) file for details.

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## 🙏 Acknowledgments

Inspired by [Python Cadwyn](https://github.com/zmievsa/cadwyn) - bringing Stripe-like API versioning to the Go ecosystem.