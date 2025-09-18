# Cadwyn-Go

Production-ready API versioning library for Go, inspired by [Cadwyn](https://github.com/zmievsa/cadwyn) for FastAPI.

**Cadwyn-Go allows you to maintain the implementation just for your newest API version and get all the older versions generated automatically.** You keep API backward compatibility encapsulated in small and independent "version change" modules while your business logic stays simple and knows nothing about versioning.

[![Go Version](https://img.shields.io/badge/go-1.18+-blue.svg)](https://golang.org)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## ✨ Features

- **🔄 Automatic Version Generation**: Maintain only your latest API version, older versions are generated automatically
- **🛡️ Backward Compatibility**: Keep API changes encapsulated in version change modules  
- **📅 Multiple Version Formats**: Support date-based (`2023-01-01`), semantic versioning (`1.2.3`), or custom formats
- **🔍 Flexible Version Detection**: Detect versions from headers, query parameters, or URL paths
- **🚀 Request/Response Migration**: Automatic transformation of requests and responses between versions
- **🔌 Middleware Integration**: Easy integration with existing HTTP servers and frameworks
- **🏗️ Schema Analysis**: Optional struct introspection and automatic migration generation
- **🎯 Type Safety**: Full Go type system integration with compile-time safety
- **📊 Route Introspection**: Debug and analyze your versioned API structure
- **🧪 Comprehensive Testing**: Extensive test coverage ensures reliability

## 🚀 Quick Start

### Installation

```bash
go get github.com/isaacchung/cadwyn-go
```

### Basic Usage

```go
package main

import (
    "encoding/json"
    "net/http"
    "github.com/isaacchung/cadwyn-go/pkg/cadwyn"
    "github.com/isaacchung/cadwyn-go/pkg/migration"
)

// Define your latest data model
type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"`
}

// Define how to migrate between versions
type AddEmailChange struct {
    *migration.BaseVersionChange
}

func (c *AddEmailChange) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
    // Remove email field for older clients
    if userMap, ok := data.(map[string]interface{}); ok {
        delete(userMap, "email")
    }
    return data, nil
}

func main() {
    // Create Cadwyn app with fluent API
    app, err := cadwyn.NewBuilder().
        WithDateVersions("2023-01-01", "2023-06-01").  // Your API versions
        WithVersionChanges(&AddEmailChange{
            BaseVersionChange: migration.NewBaseVersionChange(
                "Added email field",
                cadwyn.DateVersion("2023-01-01"),
                cadwyn.DateVersion("2023-06-01"),
            ),
        }).
        Build()
    
    if err != nil {
        panic(err)
    }

    // Register your latest API handlers
    app.Router().GET("/users", func(w http.ResponseWriter, r *http.Request) {
        users := []User{
            {ID: 1, Name: "Alice", Email: "alice@example.com"},
            {ID: 2, Name: "Bob", Email: "bob@example.com"},
        }
        json.NewEncoder(w).Encode(users)
    })

    // Start server - Cadwyn handles version detection and migration automatically
    http.ListenAndServe(":8080", app)
}
```

### Test Different Versions

```bash
# Latest version (includes email)
curl http://localhost:8080/users

# Version 2023-06-01 (includes email)  
curl -H 'x-api-version: 2023-06-01' http://localhost:8080/users

# Version 2023-01-01 (email automatically removed)
curl -H 'x-api-version: 2023-01-01' http://localhost:8080/users
```

## 🏗️ Architecture

Cadwyn-Go is built with a modular architecture:

### Core Components

1. **Version Management** (`pkg/version`) - Version parsing, comparison, and management
2. **Migration Logic** (`pkg/migration`) - Request/response transformation between versions
3. **HTTP Middleware** (`pkg/middleware`) - Version detection and automatic migration
4. **Schema Analysis** (`pkg/schema`) - Struct introspection and migration generation
5. **Router Integration** (`pkg/router`) - Versioned HTTP routing with automatic endpoint generation
6. **Main Orchestrator** (`pkg/cadwyn`) - Unified API that ties everything together

### How It Works

```
Client Request (v1.0) → [Version Detection] → [Request Migration] → Your Handler (v3.0) → [Response Migration] → Client Response (v1.0)
```

1. **Version Detection**: Middleware detects API version from headers/query/path
2. **Request Migration**: Incoming data is migrated to your latest version
3. **Handler Execution**: Your code runs with the latest data model
4. **Response Migration**: Response is migrated back to the requested version
5. **Client Receives**: Data in the format they expect

## 📖 Documentation

### Version Detection

Cadwyn-Go supports multiple ways to specify API versions:

```go
// Header-based (default)
app := cadwyn.NewBuilder().
    WithVersionLocation(middleware.VersionLocationHeader).
    WithVersionParameter("x-api-version").
    Build()

// Query parameter
app := cadwyn.NewBuilder().
    WithVersionLocation(middleware.VersionLocationQuery).
    WithVersionParameter("version").
    Build()

// URL path
app := cadwyn.NewBuilder().
    WithVersionLocation(middleware.VersionLocationPath).
    WithVersionParameter("v").
    Build()
```

### Version Formats

```go
// Date-based versions
app := cadwyn.NewBuilder().
    WithDateVersions("2023-01-01", "2023-06-01", "2024-01-01").
    Build()

// Semantic versions  
app := cadwyn.NewBuilder().
    WithSemverVersions("1.0.0", "1.1.0", "2.0.0").
    Build()

// Mixed versions
app := cadwyn.NewBuilder().
    WithVersions(
        cadwyn.DateVersion("2023-01-01"),
        cadwyn.SemverVersion("1.0.0"),
        cadwyn.HeadVersion(),
    ).
    Build()
```

### Version Changes

Define how to transform data between versions:

```go
type MyVersionChange struct {
    *migration.BaseVersionChange
}

func (c *MyVersionChange) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
    // Transform request from old version to new version
    return transformedData, nil
}

func (c *MyVersionChange) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
    // Transform response from new version to old version  
    return transformedData, nil
}
```

### Route Groups and Middleware

```go
// Create API groups with common middleware
api := app.Router().Group("/api")
api.Use(authMiddleware)
api.Use(loggingMiddleware)

// Register versioned endpoints
api.GET("/users", handleUsers)
api.POST("/users", handleCreateUser)
api.PUT("/users/{id}", handleUpdateUser)
```

### Schema Analysis

```go
// Enable schema analysis
app := cadwyn.NewBuilder().
    WithSchemaAnalysis().
    Build()

// Analyze struct differences
changes, err := app.GenerateMigrationChanges(UserV1{}, UserV2{})
if err != nil {
    log.Fatal(err)
}

// Inspect schema
schema, err := app.AnalyzeSchema(User{})
```

## 🔧 Advanced Usage

### Integration with Existing Servers

```go
// Use as middleware with existing handlers
existingHandler := http.HandlerFunc(myHandler)
wrappedHandler := app.Middleware()(existingHandler)

// Use with popular frameworks
mux := http.NewServeMux()
mux.Handle("/api/", app.Middleware()(apiHandler))

// Gin integration
r := gin.Default()
r.Use(gin.WrapH(app.Middleware()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    // Your handler
}))))
```

### Custom Version Changes

```go
// Field transformations
type RenameFieldChange struct {
    *migration.BaseVersionChange
}

func (c *RenameFieldChange) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
    if userMap, ok := data.(map[string]interface{}); ok {
        // Rename field for backward compatibility
        if fullName, exists := userMap["full_name"]; exists {
            userMap["name"] = fullName
            delete(userMap, "full_name")
        }
    }
    return data, nil
}
```

### Debug and Introspection

```go
// Enable debug logging
app := cadwyn.NewBuilder().
    WithDebugLogging().
    Build()

// Print route information
app.PrintRoutes()

// Get route info programmatically
for _, route := range app.GetRouteInfo() {
    fmt.Printf("%s %s (versions: %v)\n", route.Method, route.Pattern, route.Versions)
}
```

## 📁 Examples

Learn API versioning through a **clear progression** from basic to advanced:

#### 🚀 [Basic](examples/basic/) - **Start Here!**
Learn the fundamentals: version detection, field migration, and core concepts.

```bash
cd examples/basic && go run main.go
```

#### 🏗️ [Intermediate](examples/intermediate/) - **Production API**
Build a real API server with multiple versions, CRUD operations, and HTTP routing.

```bash
cd examples/intermediate && go run main.go
```

#### ⚡ [Advanced](examples/advanced/) - **Complex Scenarios**  
Master performance, concurrency, complex transformations, and production patterns.

```bash
cd examples/advanced && go run main.go
```

#### 🔧 [Features](examples/features/) - **Specific Capabilities**
Explore focused demonstrations of particular Cadwyn-Go features.

- **[Major.Minor Versioning](examples/features/major_minor_versioning/)** - Semantic versioning without patch numbers

```bash
cd examples/features/major_minor_versioning && go run main.go
```

**Validate everything works:**
```bash
go run validate_all.go
```

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run with coverage
go test -cover ./...

# Run specific package tests
go test ./pkg/cadwyn/
```

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request. For major changes, please open an issue first to discuss what you would like to change.

### Development

```bash
# Clone the repository
git clone https://github.com/isaacchung/cadwyn-go.git
cd cadwyn-go

# Run tests
go test ./...

# Start with the basics
cd examples/basic && go run main.go

# Or validate everything works
go run validate_all.go
```

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## 🙏 Acknowledgments

- Inspired by [Cadwyn](https://github.com/zmievsa/cadwyn) for Python/FastAPI
- Built with ❤️ for the Go community

## 🔗 Related Projects

- [Cadwyn (Python)](https://github.com/zmievsa/cadwyn) - The original Cadwyn for FastAPI
- [API Versioning Best Practices](https://docs.cadwyn.dev/theory/) - Theory and best practices
