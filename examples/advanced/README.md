# Advanced Example - Production Patterns

**For experienced developers** ready to implement complex versioning scenarios and optimize for production.

## 🎯 What You'll Master

- **Complex Field Transformations**: Field renaming, type changes, nested structures
- **Performance Optimization**: Large datasets, concurrent requests, memory efficiency
- **Production Patterns**: Error recovery, content-type handling, version-specific routes
- **Advanced Migrations**: Custom transformations, semantic versioning, mixed version types
- **Scalability**: Handling thousands of requests and large data payloads

## 🚀 Running the Example

```bash
cd examples/advanced
go run main.go
```

## ⚡ Advanced Scenarios Covered

### 1. **Complex Field Transformations**
```go
// V1: {id, amount}
// V2: {id, amount, currency, tax}  
// V3: {id, total_amount, currency, tax_amount, created_at, items[]}
//     ↑ renamed fields + nested structures
```

### 2. **Performance Testing**
- **Large Datasets**: 1000+ record migrations
- **Concurrent Requests**: 10+ simultaneous version conversions
- **Memory Efficiency**: Streaming transformations

### 3. **Production Features**
```go
// Semantic versioning support
app.WithSemverVersions("1.0.0", "1.1.0", "2.0.0")

// Mixed version types
app.WithVersions(
    cadwyn.DateVersion("2023-01-01"),
    cadwyn.SemverVersion("1.0.0"),
    cadwyn.DateVersion("2024-01-01"),
)
```

### 4. **Custom Version Changes**
```go
type CustomTransformChange struct {
    *migration.BaseVersionChange
}

func (c *CustomTransformChange) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
    // Custom business logic transformations
    return transformData(data), nil
}
```

## 🧪 What Gets Tested

1. **🔄 Complex Field Transformations** - Renaming, type changes, nested data
2. **🏗️ Nested Structure Handling** - Arrays, objects, complex hierarchies  
3. **📊 Semantic Versioning** - Major.minor.patch version support
4. **🔀 Mixed Version Types** - Date + semver in same application
5. **⚙️ Custom Version Changes** - Advanced transformation logic
6. **🏎️ Performance with Large Data** - 1000+ records, timing benchmarks
7. **🔀 Concurrent Request Handling** - Thread safety, race conditions
8. **🎯 Version-Specific Routes** - Routes that only exist in certain versions
9. **📄 Content-Type Handling** - Non-JSON responses, binary data
10. **🛡️ Error Recovery** - Graceful degradation, fallback strategies

## 📈 Performance Benchmarks

The example includes real performance testing:

```
✅ Large dataset (1000 items) processed in 15ms
✅ 10 concurrent requests handled correctly
✅ Memory usage remains stable under load
```

## 🏗️ Production Architecture Patterns

### Version-Specific Routes
```go
// Route only available in v2+
app.Router().GET("/v2-feature", handler, v2, v3)
```

### Error Recovery
```go
// Graceful fallback for invalid versions
req.Header.Set("x-api-version", "invalid-version")
// → Automatically uses default version
```

### Content-Type Flexibility
```go
// Handle non-JSON responses
app.Router().GET("/binary-data", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/octet-stream")
    w.Write(binaryData)
})
```

## ✅ Success Output

```
⚡ Cadwyn-Go Advanced Example
Complex scenarios, performance testing, and production patterns
=================================================================

1. 🔄 Testing Complex Field Transformations
   ✅ V3 has renamed fields and items
   ✅ V2 has old field names, no items
   ✅ V1 has only basic fields

2. 🏗️ Testing Nested Structure Handling
   ✅ Nested structures handled correctly

... (all 10 advanced tests)

=================================================================
🎉 Excellent! You've mastered advanced Cadwyn-Go patterns!
🚀 You're ready to build production-grade versioned APIs!
```

## 🚀 Production Deployment Tips

1. **Performance**: Use connection pooling and caching for version metadata
2. **Monitoring**: Track version usage and migration performance
3. **Scaling**: Consider async migration for very large payloads
4. **Security**: Validate version strings to prevent injection attacks
5. **Backwards Compatibility**: Plan migration paths carefully

## 🎓 What's Next?

You've mastered Cadwyn-Go! Now you can:

- **Build Production APIs** with confidence
- **Handle Complex Migrations** with custom transformations  
- **Optimize Performance** for high-traffic applications
- **Implement Advanced Patterns** like semantic versioning

Check out specific features: `cd ../features/major_minor_versioning`

## 💎 Expert Level Achieved

You're now ready to implement enterprise-grade API versioning with Cadwyn-Go! 🚀
