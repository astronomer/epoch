# Cadwyn-Go Examples

Learn API versioning with Cadwyn-Go through a **clear progression** from basic concepts to advanced production patterns.

## 🎯 Learning Path

### 1. 🚀 [Basic](basic/) - **Start Here!**
Learn the fundamentals of API versioning.

**Perfect for:** First-time users, learning core concepts

**What you'll learn:**
- Version detection from headers
- Field addition/removal between versions
- Multi-version migration chains
- Request vs response transformations
- Array handling and default values
- Error handling and edge cases

```bash
cd examples/basic && go run main.go
```

### 2. 🏗️ [Intermediate](intermediate/) - **Production API**
Build a real API server with versioning.

**Perfect for:** Building production applications

**What you'll learn:**
- Complete REST API with CRUD operations
- Multiple API versions (v1.0, v2.0, v3.0)
- HTTP server setup and routing
- Real-world field evolution patterns
- Query parameters and filtering
- Production deployment patterns

```bash
cd examples/intermediate && go run main.go
# Starts a real server on http://localhost:8080
```

### 3. ⚡ [Advanced](advanced/) - **Complex Scenarios**
Master advanced patterns and performance.

**Perfect for:** High-traffic applications, complex requirements

**What you'll learn:**
- Complex field transformations and renaming
- Nested structures and type changes
- Performance with large datasets (1000+ records)
- Concurrent request handling
- Custom version change implementations
- Mixed version types (date + semver)
- Version-specific routes and content types
- Error recovery and production patterns

```bash
cd examples/advanced && go run main.go
```

### 4. 🔧 [Features](features/) - **Specific Capabilities**
Explore specific Cadwyn-Go features.

**Perfect for:** Learning particular features, implementing specific patterns

**Available features:**
- **[Major.Minor Versioning](features/major_minor_versioning/)** - Semantic versioning without patch numbers

```bash
cd examples/features/major_minor_versioning && go run main.go
```

## 🚀 Quick Start

**New to Cadwyn-Go?** Follow the learning path:

```bash
# 1. Start with basics
cd examples/basic && go run main.go

# 2. Build a production API  
cd examples/intermediate && go run main.go

# 3. Master advanced patterns
cd examples/advanced && go run main.go

# 4. Explore specific features
cd examples/features/major_minor_versioning && go run main.go
```

**Want to validate everything works?** Run the full test suite:

```bash
# From the project root
go run validate_all.go
```

## 🎯 Choose Your Path

| **I want to...** | **Go to** | **Time needed** |
|-------------------|-----------|-----------------|
| **Learn the basics** | [`basic/`](basic/) | 10 minutes |
| **Build a real API** | [`intermediate/`](intermediate/) | 20 minutes |
| **Master advanced patterns** | [`advanced/`](advanced/) | 30 minutes |
| **Explore specific features** | [`features/`](features/) | 5-15 minutes each |

## 💡 Core Concepts

All examples demonstrate the **key Cadwyn principle**:

> **Write once for the latest version, support all previous versions automatically**

### Version Detection
```bash
# Multiple ways to specify API version
curl -H 'x-api-version: 1.0' http://localhost:8080/users        # Header
curl 'http://localhost:8080/users?api_version=1.1'              # Query param
curl http://localhost:8080/v2.0/users                           # URL path
```

### Automatic Migration
```go
// Define once, works for all versions
type UserV3 struct {
    ID        int       `json:"id"`
    Name      string    `json:"name"`
    Email     string    `json:"email"`     // Added in v2
    Status    string    `json:"status"`    // Added in v3
    CreatedAt time.Time `json:"created_at"` // Added in v3
}

// Clients get exactly what they expect:
// v1.0 client → {id, name}
// v2.0 client → {id, name, email}  
// v3.0 client → {id, name, email, status, created_at}
```

## 🏆 What You'll Master

By completing all examples, you'll know how to:

- ✅ **Version APIs** with dates, semantic versions, or mixed approaches
- ✅ **Migrate data** automatically between any API versions
- ✅ **Handle complexity** like field renaming, nested structures, arrays
- ✅ **Optimize performance** for large datasets and high concurrency
- ✅ **Deploy to production** with proper error handling and monitoring
- ✅ **Test thoroughly** with comprehensive validation patterns

## 🎓 Learning Outcomes

**After Basic:** You understand how API versioning works with Cadwyn-Go
**After Intermediate:** You can build production APIs with multiple versions
**After Advanced:** You can handle complex scenarios and optimize for scale
**After Features:** You know all the specialized capabilities available

## 🚀 Ready to Start?

Begin your journey: **[`cd examples/basic && go run main.go`](basic/)**
