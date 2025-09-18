# Basic Example - Getting Started

**Start here!** This example teaches you the fundamentals of API versioning with Cadwyn-Go.

## 🎯 What You'll Learn

- **Version Detection**: How Cadwyn-Go detects API versions from headers
- **Field Migration**: Adding and removing fields between API versions  
- **Multi-Version Support**: Handling requests across multiple API versions
- **Request vs Response Migration**: Different transformations for incoming and outgoing data
- **Array Handling**: Migrating collections and nested data
- **Default Values**: Providing sensible defaults for new fields
- **Error Handling**: Graceful fallbacks for invalid versions

## 🚀 Running the Example

```bash
cd examples/basic
go run main.go
```

## 📚 Key Concepts Demonstrated

### 1. Version Detection
```go
// Cadwyn automatically detects versions from headers
app, err := cadwyn.NewBuilder().
    WithDateVersions("2023-01-01", "2023-06-01", "2024-01-01").
    Build()
```

### 2. Field Migration
```go
// V1 User: {id, name}
// V2 User: {id, name, email}
// Cadwyn automatically adds/removes email field based on version
```

### 3. Version Changes
```go
type V1ToV2Change struct {
    *migration.BaseVersionChange
}

func (c *V1ToV2Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
    // Remove email field for v1 clients
    return removeFields(data, "email")
}
```

## 🧪 What the Example Tests

1. **🔍 Version Detection** - Headers, defaults, fallbacks
2. **➕ Field Addition/Removal** - Email field migration
3. **🔗 Multi-Version Migration** - V1 → V2 → V3 chains
4. **🔄 Request vs Response** - Different migrations for each direction
5. **📋 Array Migration** - Collections of users
6. **🎯 Detection Methods** - Query parameters, headers
7. **⚠️ Edge Cases** - Invalid versions, error recovery
8. **🔧 Default Values** - Automatic field population

## ✅ Success Output

When you run this example, you should see:

```
🚀 Cadwyn-Go Basic Example
Learn the fundamentals of API versioning with Cadwyn-Go
============================================================

1. 🔍 Testing Version Detection
   ✅ Specific version in header
   ✅ No version header (uses default)

2. ➕ Testing Field Addition/Removal  
   ✅ V2 includes email field
   ✅ V1 excludes email field

... (all 8 tests)

============================================================
🎉 Congratulations! You've learned the basics of Cadwyn-Go!
📚 Next steps:
   • Try the intermediate example: cd ../intermediate && go run main.go
   • Explore advanced features: cd ../advanced && go run main.go
```

## 🎓 Next Steps

Once you've mastered the basics:

1. **Intermediate**: Build a production API server → `cd ../intermediate`
2. **Advanced**: Learn complex patterns and performance → `cd ../advanced`  
3. **Features**: Explore specific features → `cd ../features/major_minor_versioning`

## 💡 Key Takeaways

- **Write once, support all versions**: Maintain only your latest API
- **Automatic migration**: Cadwyn handles all the version conversions
- **Flexible detection**: Headers, query params, URL paths all supported
- **Production ready**: Handles edge cases and errors gracefully

Ready to build versioned APIs? Start here and work your way up! 🚀
