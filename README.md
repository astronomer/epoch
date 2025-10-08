# Epoch

**API versioning for Go with automatic request/response migrations**

Epoch lets you version your Go APIs the way Stripe does - write your handlers once for the latest version, then define migrations to transform requests and responses for older API versions automatically.

[![Go Version](https://img.shields.io/badge/go-%3E%3D1.21-blue.svg)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

## Why Epoch?

- **Write once** - Implement handlers for your latest API version only
- **Automatic migrations** - Define transformations between versions declaratively
- **No duplication** - No need to maintain multiple versions of the same endpoint
- **Flexible versioning** - Support date-based (`2024-01-01`), semantic (`v1.0.0`), or string versions
- **Gin integration** - Drop into existing Gin applications with minimal changes

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
    
    migration := epoch.NewVersionChange(
        "Add email to User",
        v1, v2,
        // Forward: v1 request → v2 (add email)
        &epoch.AlterRequestInstruction{
            Schemas: []interface{}{User{}},
            Transformer: func(req *epoch.RequestInfo) error {
                if user, ok := req.Body.(map[string]interface{}); ok {
                    if _, hasEmail := user["email"]; !hasEmail {
                        user["email"] = "user@example.com"
                    }
                }
                return nil
            },
        },
        // Backward: v2 response → v1 (remove email)
        &epoch.AlterResponseInstruction{
            Schemas: []interface{}{User{}},
            Transformer: func(resp *epoch.ResponseInfo) error {
                if user, ok := resp.Body.(map[string]interface{}); ok {
                    delete(user, "email")
                }
                return nil
            },
        },
    )

    // Setup Epoch
    epochInstance, _ := epoch.NewEpoch().
        WithVersions(v1, v2).
        WithHeadVersion().
        WithChanges(migration).
        Build()

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

**Test it:**
```bash
# v1.0.0 - No email in response
curl http://localhost:8080/users/1 -H "X-API-Version: 1.0.0"
# {"id":1,"name":"John"}

# v2.0.0 - Email included
curl http://localhost:8080/users/1 -H "X-API-Version: 2.0.0"
# {"id":1,"name":"John","email":"john@example.com"}
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
        // Modify req.Body to add missing fields, rename fields, etc.
        return nil
    },
}
```

**AlterResponseInstruction** - Transform new responses to old format:
```go
&epoch.AlterResponseInstruction{
    Schemas: []interface{}{User{}},
    Transformer: func(resp *epoch.ResponseInfo) error {
        // Modify resp.Body to remove new fields, rename fields, etc.
        return nil
    },
}
```

### 4. Version Detection

Cadwyn automatically detects versions from:
- **Headers** (default): `X-API-Version: 2024-01-01`
- **URL path**: `/v2024-01-01/users` or `/2024-01-01/users`

Configure the detection method:
```go
epoch.NewEpoch().
    WithVersionLocation(epoch.VersionLocationHeader).  // or Path
    WithVersionParameter("X-API-Version").              // Custom header name
    Build()
```

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

// Configure version detection
builder.WithVersionLocation(epoch.VersionLocationHeader)
builder.WithVersionParameter("X-API-Version")
builder.WithVersionFormat(epoch.VersionFormatDate)

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
            if userMap, ok := resp.Body.(map[string]interface{}); ok {
                delete(userMap, "email")
            }
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
            if productMap, ok := resp.Body.(map[string]interface{}); ok {
                delete(productMap, "sku")
            }
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

# Verify examples compile
cd examples/basic && go build
cd examples/advanced && go build
```

## Contributing

Contributions welcome! Please feel free to submit a Pull Request.

## License

MIT License - see [LICENSE](LICENSE) file for details.

## Acknowledgments

Inspired by Stripe-style API versioning - bringing elegant version management to the Go ecosystem.
