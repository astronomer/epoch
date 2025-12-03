# Epoch

**API versioning for Go with automatic request/response migrations**

Epoch lets you version your Go APIs the way Stripe does - write your handlers once for the latest version, then define migrations to transform requests and responses for older API versions automatically.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Why Epoch?

- **Type-based routing** - Explicit type registration at endpoint setup for predictable migrations
- **Flow-based operations** - Clear separation: requests go Client→HEAD, responses go HEAD→Client
- **Automatic bidirectional** - One operation generates both request and response transformations
- **Field order preservation** - JSON responses maintain original field order using Sonic
- **Cycle detection** - Built-in validation prevents circular migration dependencies

### Core Features

- **Write once** - Implement handlers for your latest API version only
- **Type-safe** - Register types at endpoint setup with compile-time checking
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
    
    migration := epoch.NewVersionChangeBuilder(v1, v2).
        Description("Add email to User").
        ForType(User{}).
            RequestToNextVersion().
                AddField("email", "user@example.com").
            ResponseToPreviousVersion().
                RemoveField("email").
        Build()

    // Setup Epoch
    epochInstance, err := epoch.NewEpoch().
        WithVersions(v1, v2).
        WithHeadVersion().
        WithChanges(migration).
        Build()
    
    if err != nil {
        panic(err) 
    }

    // Add to Gin
    r := gin.Default()
    r.Use(epochInstance.Middleware())
    
    // Register endpoints with type information
    r.GET("/users/:id", epochInstance.WrapHandler(getUser).Returns(User{}).ToHandlerFunc("GET", "/users/:id"))
    r.POST("/users", epochInstance.WrapHandler(createUser).Accepts(User{}).Returns(User{}).ToHandlerFunc("POST", "/users"))
    
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
- ✅ `ForType(User{})` explicitly registers which type this migration applies to
- ✅ `RequestToNextVersion().AddField()` handles Client→HEAD transformations
- ✅ `ResponseToPreviousVersion().RemoveField()` handles HEAD→Client transformations
- ✅ `WrapHandler().Returns(User{})` registers the endpoint with type information

**Test it:**
```bash
# v1.0.0 - No email in response
curl http://localhost:8080/users/1 -H "X-API-Version: 1.0.0"
# {"id":1,"name":"John"}

# v2.0.0 - Email included
curl http://localhost:8080/users/1 -H "X-API-Version: 2.0.0"
# {"id":1,"name":"John","email":"john@example.com"}
```

## Flow-Based Operations

The new framework uses **flow-based operations** that match the actual migration direction:

### Request Operations (Client → HEAD)

When a v1 client sends a request, it needs to be migrated TO the HEAD version:

```go
migration := epoch.NewVersionChangeBuilder(v1, v2).
    ForType(User{}).
        RequestToNextVersion().
            AddField("email", "default@example.com").      // Add field for old clients
            RemoveField("deprecated_field").               // Remove deprecated field
            RenameField("name", "full_name").              // Rename old field to new
        Build()
```

### Response Operations (HEAD → Client)

When returning to a v1 client, response needs to be migrated FROM HEAD to v1:

```go
migration := epoch.NewVersionChangeBuilder(v1, v2).
    ForType(User{}).
        ResponseToPreviousVersion().
            RemoveField("email").                          // Remove new fields
            AddField("old_field", "default").              // Restore old fields
            RenameField("full_name", "name").              // Rename back to old name
        Build()
```

### Available Operations

**Request Operations** (Client → HEAD):
- `AddField(name, default)` - Add field if missing
- `RemoveField(name)` - Remove field
- `RenameField(from, to)` - Rename field
- `Custom(func)` - Custom transformation logic

**Response Operations** (HEAD → Client):
- `AddField(name, default)` - Add field if missing
- `RemoveField(name)` - Remove field
- `RenameField(from, to)` - Rename field
- `RemoveFieldIfDefault(name, default)` - Conditional removalz
- `Custom(func)` - Custom transformation logic

## Type-Based Routing

Epoch requires **explicit type registration** at endpoint setup. When you call `ToHandlerFunc(method, path)`, it immediately registers the endpoint with its type information in Epoch's internal registry.

```go
// Register endpoints with type information (method and path required for immediate registration)
r.GET("/users/:id", 
    epochInstance.WrapHandler(getUser).
        Returns(User{}).                    // Response type
        ToHandlerFunc("GET", "/users/:id")) // Registers endpoint immediately

r.POST("/users", 
    epochInstance.WrapHandler(createUser).
        Accepts(User{}).                    // Request type
        Returns(User{}).                    // Response type
        ToHandlerFunc("POST", "/users"))    // Registers endpoint immediately

// Array responses
r.GET("/users",
    epochInstance.WrapHandler(listUsers).
        Returns([]User{}).                  // Returns array of Users
        ToHandlerFunc("GET", "/users"))
```

**Important**: The method and path parameters passed to `ToHandlerFunc()` must match the route being registered. This enables immediate endpoint registration for features like OpenAPI schema generation.

## Multiple Types in One Migration

You can migrate multiple types together:

```go
migration := epoch.NewVersionChangeBuilder(v2, v3).
    Description("Update User and Product").
    ForType(User{}).
        ResponseToPreviousVersion().
            RenameField("full_name", "name").
    ForType(Product{}).
        ResponseToPreviousVersion().
            RemoveField("currency").
    Build()
```

## Custom Transformations

Mix declarative operations with custom logic:

```go
migration := epoch.NewVersionChangeBuilder(v1, v2).
    ForType(User{}).
        RequestToNextVersion().
            AddField("email", "default@example.com").
            Custom(func(req *epoch.RequestInfo) error {
                // Complex validation or transformation
                if email, _ := req.GetFieldString("email"); email == "" {
                    req.SetField("email", "user@example.com")
                }
                return nil
            }).
    Build()
```

## Global Transformers

Apply transformations to all types:

```go
migration := epoch.NewVersionChangeBuilder(v1, v2).
    CustomRequest(func(req *epoch.RequestInfo) error {
        // Applies to ALL request types
        return nil
    }).
    CustomResponse(func(resp *epoch.ResponseInfo) error {
        // Applies to ALL response types
        return nil
    }).
    ForType(User{}).
        ResponseToPreviousVersion().
            RemoveField("email").
    Build()
```

## Helper Methods

**RequestInfo and ResponseInfo** provide convenient methods:

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

**Global AST Helper Functions**:

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

## Version Detection

Epoch automatically detects versions from:
- **Headers**: `X-API-Version: 2024-01-01` (highest priority)
- **URL path**: `/v2024-01-01/users` or `/v1/users`

If both are present, header takes priority.

### Partial Version Matching

Specify major version only:

```go
// Configure: 1.0.0, 1.1.0, 1.2.0, 2.0.0, 2.1.0
r.GET("/api/v1/users", handler)  // Routes to latest v1.x (1.2.0)
r.GET("/api/v2/users", handler)  // Routes to latest v2.x (2.1.0)
```

```bash
curl http://localhost:8080/api/v1/users
# Automatically uses v1.2.0 (latest v1.x)
```

## Builder API

```go
epochInstance, err := epoch.NewEpoch().
    // Add versions
    WithVersions(v1, v2, v3).
    WithDateVersions("2023-01-01", "2024-01-01").
    WithSemverVersions("1.0.0", "2.0.0").
    WithHeadVersion().
    // Add migrations
    WithChanges(change1, change2, change3).
    // Configure (optional)
    WithVersionParameter("X-API-Version").
    WithVersionFormat(epoch.VersionFormatDate).
    WithDefaultVersion(v1).
    Build()
```

## Examples

### Basic Example
```bash
cd examples/basic
go run main.go
```

Demonstrates:
- Semantic versioning (v1.0.0, v2.0.0)
- Type registration with `.Returns()` and `.Accepts()`
- Flow-based operations (`RequestToNextVersion`, `ResponseToPreviousVersion`)
- Simple field addition

### Advanced Example
```bash
cd examples/advanced
go run main.go
```

Demonstrates:
- Date-based versioning
- Multiple models (User, Product, Order)
- Field additions and renames across versions
- Array transformations
- Automatic nested type discovery (arrays and objects)
- Full CRUD operations

## How It Works

1. **Handler runs at HEAD version** - You implement handlers for the latest version only
2. **Epoch detects requested version** - From `X-API-Version` header or URL path
3. **Request migration** - Transforms incoming request: Client Version → HEAD
4. **Handler executes** - With migrated request in HEAD format
5. **Response migration** - Transforms outgoing response: HEAD → Client Version
6. **Client receives** - Response in their requested version format

```
Client (v1) → [v1 Request] → Migration (v1→v2) → [v2 Request] → Handler (v2)
                                                                      ↓
Client (v1) ← [v1 Response] ← Migration (v2→v1) ← [v2 Response] ← Handler (v2)
```

## Best Practices

### 1. Always Register Types

Always use `.Returns()` and `.Accepts()` to register endpoint types:

```go
// ✅ Good
r.GET("/users/:id", epochInstance.WrapHandler(getUser).Returns(User{}).ToHandlerFunc("GET", "/users/:id"))

// ❌ Bad - no type registration
r.GET("/users/:id", epochInstance.WrapHandler(getUser).ToHandlerFunc("GET", "/users/:id"))
```

### 2. One Type Per ForType()

Keep migrations focused on single types:

```go
// ✅ Good - separate migrations per type
userChange := epoch.NewVersionChangeBuilder(v1, v2).
    ForType(User{}).
        ResponseToPreviousVersion().RemoveField("email").
    Build()

productChange := epoch.NewVersionChangeBuilder(v1, v2).
    ForType(Product{}).
        ResponseToPreviousVersion().RemoveField("sku").
    Build()

// ❌ Avoid - mixing types in operations can be confusing
```

### 3. Use Flow-Based Operations

Use operations that match the actual migration direction:

```go
// ✅ Good - clear flow direction
migration := epoch.NewVersionChangeBuilder(v1, v2).
    ForType(User{}).
        RequestToNextVersion().      // Client → HEAD
            AddField("email", "default").
        ResponseToPreviousVersion(). // HEAD → Client
            RemoveField("email").
    Build()
```

## Testing

```bash
# Run all tests 
make test-ginkgo

# Or use go test
go test ./epoch/...

# Run with coverage
go test ./epoch/... -coverprofile=coverage.out
go tool cover -html=coverage.out

# Verify examples compile
cd examples/basic && go build
cd examples/advanced && go build
```

## Contributing

Contributions welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Inspired by Stripe-style API versioning and [Cadwyn](https://github.com/zmievsa/cadwyn) for Python.
