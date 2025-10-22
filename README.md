# Epoch

**API versioning for Go with automatic request/response migrations**

Epoch lets you version your Go APIs the way Stripe does - write your handlers once for the latest version, then define migrations to transform requests and responses for older API versions automatically.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Why Epoch?
- **One operation, multiple transformations** - `RenameField("old", "new")` generates request, response, AND error transformations
- **Type-safe** - Compile-time checking with Go's type system
- **Automatic error field transformation** - Field names in error messages automatically update for each version
- **Path-based routing** - Explicit control over which endpoints each migration affects

### Core Features
- **Write once** - Implement handlers for your latest API version only
- **Bidirectional migrations** - Declarative operations work in both directions automatically
- **Field order preservation** - JSON responses maintain original field order
- **No duplication** - No need to maintain multiple versions of the same endpoint
- **Flexible versioning** - Support date-based (`2024-01-01`), semantic (`v1.0.0`), or string versions
- **Gin integration** - Drop into existing Gin applications with minimal changes
- **High performance** - Utilizes ByteDance Sonic for fast JSON processing

## Installation

```bash
go get github.com/astronomer/epoch
```

## Quick Start

```go
package main

import (
    "github.com/astronomer/epoch/epoch"
    "github.com/gin-gonic/gin"
)

type User struct {
    ID    int    `json:"id"`
    Name  string `json:"name"`
    Email string `json:"email"` // Added in v2.0.0
}

func main() {
    // Define version migration
    v1, _ := epoch.NewSemverVersion("1.0.0")
    v2, _ := epoch.NewSemverVersion("2.0.0")
    
    // ✨ One line instead of 20+ lines!
    migration := epoch.NewVersionChangeBuilder(v1, v2).
        Description("Add email to User").
        // PATH-BASED ROUTING: Explicit which endpoints are affected
        ForPath("/users", "/users/:id").
            AddField("email", "user@example.com").  // Automatic bidirectional!
        Build()

    // Setup Epoch (with automatic cycle detection)
    epochInstance, err := epoch.NewEpoch().
        WithVersions(v1, v2).
        WithHeadVersion().
        WithChanges(migration).
        Build()
    
    if err != nil {
        panic(err)  // Will catch cycles: "cycle detected in version chain: ..."
    }

    // Add to Gin
    r := gin.Default()
    r.Use(epochInstance.Middleware())
    
    // Wrap handlers that need versioning
    r.GET("/users/:id", epochInstance.WrapHandler(getUser))
    r.POST("/users", epochInstance.WrapHandler(createUser))
    
    r.Run(":8080")
}

func getUser(c *gin.Context) {
    // Always implement for HEAD (latest) version
    user := User{ID: 1, Name: "John", Email: "john@example.com"}
    c.JSON(200, user)
}

func createUser(c *gin.Context) {
    var user User
    c.ShouldBindJSON(&user)
    c.JSON(201, user)
}
```

**What just happened?**
- ✅ `ForPath()` explicitly defines which endpoints this migration applies to (path-based routing)
- ✅ `AddField("email", "user@example.com")` automatically creates BOTH migrations:
  - Request migration (v1→v2): adds `email` field if missing
  - Response migration (v2→v1): removes `email` field
- ✅ Error messages automatically transform field names for each version
- ✅ **91% less code** for common operations!

### Old API (Still Supported)

The imperative API still works for complex custom logic:

<details>
<summary>Click to see old API example</summary>

```go
// Old way - still supported for complex cases
migration := epoch.NewVersionChange(
    "Add email to User",
    v1, v2,
    &epoch.AlterRequestInstruction{
        Path: "/users",  // Path-based routing required
        Transformer: func(req *epoch.RequestInfo) error {
            if !req.HasField("email") {
                req.SetField("email", "user@example.com")
            }
            return nil
        },
    },
    &epoch.AlterResponseInstruction{
        Path: "/users",  // Path-based routing required
        Transformer: func(resp *epoch.ResponseInfo) error {
            resp.DeleteField("email")
            return nil
        },
    },
)
```
</details>

**Test it:**
```bash
# v1.0.0 - No email in response
curl http://localhost:8080/users/1 -H "X-API-Version: 1.0.0"
# {"id":1,"name":"John"}

# v2.0.0 - Email included
curl http://localhost:8080/users/1 -H "X-API-Version: 2.0.0"
# {"id":1,"name":"John","email":"john@example.com"}
```

## Declarative Operations

The new declarative API makes common migrations incredibly simple. Here are all available operations:

### Field Operations

```go
migration := epoch.NewVersionChangeBuilder(v1, v2).
    Description("Multiple field operations").
    // PATH-BASED ROUTING: Explicit which endpoints are affected
    ForPath("/users", "/users/:id").
        AddField("email", "default@example.com").      // Add field with default
        RemoveField("temp_field").                     // Remove field
        RenameField("name", "full_name").              // Rename field (+ auto error transform!)
        MapEnumValues("status", map[string]string{    // Map enum values
            "pending": "active",
            "suspended": "inactive",
        }).
    Build()
```

> **Note:** `ForPath()` is required - it specifies which endpoints the migration applies to at runtime.

### What Happens Automatically

Each declarative operation generates **three transformations**:

| Operation | Request (v1→v2) | Response (v2→v1) | Error Messages |
|-----------|----------------|------------------|----------------|
| `AddField("email", "default")` | Adds field if missing | Removes field | N/A |
| `RemoveField("temp")` | Removes field | (Cannot restore) | N/A |
| `RenameField("name", "full_name")` | Renames `name` → `full_name` | Renames `full_name` → `name` | Transforms "full_name" → "name" |
| `MapEnumValues("status", {...})` | Maps values forward | Maps values backward | N/A |

### Multiple Paths

Apply migrations to multiple paths in one change:

```go
migration := epoch.NewVersionChangeBuilder(v2, v3).
    Description("Update User endpoints").
    ForPath("/users", "/users/:id").
        RenameField("name", "full_name").
        AddField("phone", "").
    ForPath("/products", "/products/:id").
        AddField("currency", "USD").
        AddField("description", "").
    Build()
```

### Custom Logic

For complex transformations, mix declarative + custom:

```go
migration := epoch.NewVersionChangeBuilder(v1, v2).
    Description("Complex migration").
    ForPath("/users", "/users/:id").
        AddField("email", "default@example.com").
        // Schema-specific custom logic
        OnRequest(func(req *epoch.RequestInfo) error {
            // Complex validation or transformation
            return nil
        }).
    // Global custom logic
    CustomRequest(func(req *epoch.RequestInfo) error {
        // Applies to all schemas
        return nil
    }).
    Build()
```

## Core Concepts

### 1. Version Types

**Date-based** (recommended for public APIs):
```go
v1, _ := epoch.NewDateVersion("2024-01-01")
v2, _ := epoch.NewDateVersion("2024-06-15")
```

**Semantic versioning**:
```go
v1, _ := epoch.NewSemverVersion("1.0.0")
v2, _ := epoch.NewSemverVersion("2.0.0")
```

**String-based**:
```go
v1 := epoch.NewStringVersion("alpha")
v2 := epoch.NewStringVersion("beta")
```

### 2. Version Changes

Define what changed between versions:

```go
change := epoch.NewVersionChange(
    "Description of change",
    fromVersion,
    toVersion,
    // Instructions for migration
    &epoch.AlterRequestInstruction{...},
    &epoch.AlterResponseInstruction{...},
)
```

### 3. Migration Instructions

**AlterRequestInstruction** - Transform old requests to new format:
```go
&epoch.AlterRequestInstruction{
    Schemas: []interface{}{User{}},
    Transformer: func(req *epoch.RequestInfo) error {
        // Use helper methods to modify request body
        if !req.HasField("email") {
            req.SetField("email", "default@example.com")
        }
        return nil
    },
}
```

**AlterResponseInstruction** - Transform new responses to old format:
```go
&epoch.AlterResponseInstruction{
    Schemas: []interface{}{User{}},
    Transformer: func(resp *epoch.ResponseInfo) error {
        // Use helper methods to modify response body
        resp.DeleteField("new_field")
        return nil
    },
}
```

### 4. Helper Methods

**RequestInfo and ResponseInfo** provide convenient methods for working with JSON data:

```go
// Field access
hasEmail := req.HasField("email")
email, err := req.GetFieldString("email")
age, err := req.GetFieldInt("age") 
price, err := req.GetFieldFloat("price")

// Field modification
req.SetField("email", "new@example.com")
req.DeleteField("old_field")

// Array transformation
err := resp.TransformArrayField("users", func(user *ast.Node) error {
    return epoch.DeleteNodeField(user, "internal_field")
})
```

**Global AST Helper Functions** for working with individual nodes:

```go
// Direct node manipulation (useful in TransformArrayField callbacks)
epoch.SetNodeField(node, "key", "value")
epoch.DeleteNodeField(node, "key")
epoch.RenameNodeField(node, "old_key", "new_key")
epoch.CopyNodeField(sourceNode, destNode, "key")

// Field access
value, err := epoch.GetNodeFieldString(node, "key")
exists := epoch.HasNodeField(node, "key")

// Type checking
if epoch.IsNodeArray(node) { /* handle array */ }
if epoch.IsNodeObject(node) { /* handle object */ }
```

### 5. Version Detection

Epoch automatically detects versions from **all locations** simultaneously:
- **Headers**: `X-API-Version: 2024-01-01` (checked first, highest priority)
- **URL path**: `/v2024-01-01/users` or `/2024-01-01/users` (checked second)

If both header and path contain a version, the header version takes priority.

Customize the header name:
```go
epoch.NewEpoch().
    WithVersionParameter("X-API-Version").  // Custom header name (default: "X-API-Version")
    Build()
```

#### Partial Version Matching

Epoch supports **major version shortcuts** in URL paths - specify just the major version and it automatically resolves to the latest minor/patch version:

```go
// Configure versions: 1.0.0, 1.1.0, 1.2.0, 2.0.0, 2.1.0
epochInstance, _ := epoch.NewEpoch().
    WithSemverVersions("1.0.0", "1.1.0", "1.2.0", "2.0.0", "2.1.0").
    WithHeadVersion().
    Build()

// Setup routes using major version
r.GET("/api/v1/users", epochInstance.WrapHandler(getUsers))  // Routes to v1.x
r.GET("/api/v2/users", epochInstance.WrapHandler(getUsers))  // Routes to v2.x
```

**Request examples:**
```bash
# Path with major version only → resolves to latest matching version
curl http://localhost:8080/api/v1/users
# Automatically uses v1.2.0 (latest v1.x)

curl http://localhost:8080/api/v2/users  
# Automatically uses v2.1.0 (latest v2.x)

# Full version still works
curl http://localhost:8080/api/v1.1.0/users
# Uses exactly v1.1.0

# Header takes priority over path
curl http://localhost:8080/api/v1/users -H "X-API-Version: 2.0.0"
# Uses v2.0.0 (from header, not path)
```

This pattern makes it easy for clients to request "latest v1" without needing to know the exact minor/patch version.

## Builder API

```go
builder := epoch.NewEpoch()

// Add versions
builder.WithVersions(v1, v2, v3)
builder.WithDateVersions("2023-01-01", "2024-01-01")    // Convenience method
builder.WithSemverVersions("1.0.0", "2.0.0")             // Convenience method
builder.WithHeadVersion()                                 // Always latest

// Add migrations
builder.WithChanges(change1, change2, change3)

// Configure version detection (optional)
builder.WithVersionParameter("X-API-Version")        // Custom header name
builder.WithVersionFormat(epoch.VersionFormatDate)   // Expected format

// Build
epochInstance, err := builder.Build()
```

## Examples

### Basic Example
A minimal example showing simple version migration:
```bash
cd examples/basic
go run main.go
```

Demonstrates:
- Semantic versioning (v1.0.0, v2.0.0)
- Single field addition (email)
- Request/response transformations

### Advanced Example
A full REST API with complex migrations:
```bash
cd examples/advanced
go run main.go
```

Demonstrates:
- Date-based versioning
- Multiple models (User, Product)
- Field additions and renames
- Array transformations
- Full CRUD operations

## How It Works

1. **You write handlers for the HEAD (latest) version only**
2. **Epoch middleware detects the requested API version** from headers or URL path
3. **Request migration**: Transforms incoming request from old → new format
4. **Your handler executes** with the migrated request
5. **Response migration**: Transforms outgoing response from new → old format
6. **Client receives response** in the format matching their requested version

```
Client (v1) → [v1 Request] → Migration → [v2 Request] → Handler (v2)
                                                             ↓
Client (v1) ← [v1 Response] ← Migration ← [v2 Response] ← Handler (v2)
```

## Best Practices

### One VersionChange Per Schema

Create separate `VersionChange` objects for different models/schemas. All migrations in a single `VersionChange` are applied together, so mixing multiple schemas can cause unwanted side effects.

**✅ Good:**
```go
// Separate changes for different schemas
userChange := epoch.NewVersionChange(
    "Add email to User",
    v1, v2,
    &epoch.AlterResponseInstruction{
        Schemas: []interface{}{User{}},
        Transformer: func(resp *epoch.ResponseInfo) error {
            // Only transforms User responses
            resp.DeleteField("email")
            return nil
        },
    },
)

productChange := epoch.NewVersionChange(
    "Add SKU to Product",
    v1, v2,
    &epoch.AlterResponseInstruction{
        Schemas: []interface{}{Product{}},
        Transformer: func(resp *epoch.ResponseInfo) error {
            // Only transforms Product responses
            resp.DeleteField("sku")
            return nil
        },
    },
)

epoch.NewEpoch().
    WithChanges(userChange, productChange).
    Build()
```

**❌ Avoid:**
```go
// Multiple schemas in one VersionChange - transformers run on ALL responses!
change := epoch.NewVersionChange(
    "Multiple changes",
    v1, v2,
    &epoch.AlterResponseInstruction{
        Schemas: []interface{}{User{}},
        Transformer: func(resp) { /* runs on User AND Product */ },
    },
    &epoch.AlterResponseInstruction{
        Schemas: []interface{}{Product{}},
        Transformer: func(resp) { /* runs on User AND Product */ },
    },
)
```

## Architecture: Middleware Approach

Epoch uses a **middleware approach** for Go API versioning:

**Go (Middleware)**:
- Single router with runtime transformation
- Integrates naturally with Gin's middleware chain
- Lower memory footprint
- Simpler mental model

The approach achieves **transparent API versioning with automatic migrations** following Go idioms and best practices.

## Testing

```bash
# Run all tests 
go test ./epoch/...

# Run with coverage
go test ./epoch/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Run specific test suites
go test ./epoch -run TestMigrationChain  # Cycle detection tests
go test ./epoch -run TestIntegration     # Integration tests
go test ./epoch -run TestBuilderAPI      # Declarative API tests

# Verify examples compile and run
cd examples/basic && go build
cd examples/advanced && go build
```

## Contributing

Contributions welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Inspired by Stripe-style API versioning - bringing elegant version management to the Go ecosystem.
