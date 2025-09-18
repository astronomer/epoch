# Major.Minor Versioning Example

This example demonstrates Cadwyn-Go's support for **major.minor semantic versioning** without requiring patch numbers.

## 🎯 Overview

Many APIs use major.minor versioning where:
- **Major version** changes indicate breaking changes
- **Minor version** changes indicate new features (backward compatible)
- **Patch versions** are not needed for API versioning

## 📊 API Evolution Demonstrated

### Version 1.0 → 1.1 (Minor Update)
- **v1.0**: Basic user (`id`, `name`)
- **v1.1**: Added `email` field (backward compatible)

### Version 1.1 → 2.0 (Major Update) 
- **v2.0**: Added `role` and `status` fields (breaking change - new required fields)

### Version 2.0 → 2.1 (Minor Update)
- **v2.1**: Added `created_at` timestamp (backward compatible)

## 🚀 Running the Example

```bash
cd examples/major_minor_versioning
go run main.go
```

## ✨ Key Features Demonstrated

### 1. **Major.Minor Version Support**
```go
app, err := cadwyn.NewBuilder().
    WithSemverVersions("1.0", "1.1", "2.0", "2.1"). // No patch required!
    WithVersionChanges(
        NewV1_0ToV1_1Change(),
        NewV1_1ToV2_0Change(), 
        NewV2_0ToV2_1Change(),
    ).
    Build()
```

### 2. **Automatic Migration Between Versions**
- Request v1.0 → Get only `id` and `name`
- Request v1.1 → Get `id`, `name`, and `email`
- Request v2.0 → Get `id`, `name`, `email`, `role`, and `status`
- Request v2.1 → Get all fields including `created_at`

### 3. **Semantic Version Comparison**
```go
v1_0 := cadwyn.SemverVersion("1.0")
v2_1 := cadwyn.SemverVersion("2.1")

if v1_0.IsOlderThan(v2_1) {
    // true - v1.0 < v2.1
}
```

## 🧪 Testing Different Versions

The example automatically tests all versions and shows:

```
1. 📋 Testing API v1.0 - Basic user (id, name only)
   📊 Response fields: [id name]
   ✅ Migration successful

2. 📋 Testing API v1.1 - Added email  
   📊 Response fields: [id name email]
   ✅ Migration successful

3. 📋 Testing API v2.0 - Added role and status (major change)
   📊 Response fields: [id name email role status]
   ✅ Migration successful

4. 📋 Testing API v2.1 - Added created_at timestamp
   📊 Response fields: [id name email role status created_at]
   ✅ Migration successful
```

## 🎯 Version Change Examples

### Minor Version Change (1.0 → 1.1)
```go
type V1_0ToV1_1Change struct {
    *migration.BaseVersionChange
}

func (c *V1_0ToV1_1Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
    // Add default email for v1.0 requests
    if userMap, ok := data.(map[string]interface{}); ok {
        if _, hasEmail := userMap["email"]; !hasEmail {
            userMap["email"] = ""
        }
    }
    return data, nil
}

func (c *V1_0ToV1_1Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
    // Remove email field for v1.0 responses
    return removeFields(data, "email")
}
```

### Major Version Change (1.1 → 2.0)
```go
type V1_1ToV2_0Change struct {
    *migration.BaseVersionChange
}

func (c *V1_1ToV2_0Change) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
    // Add default role and status for older requests
    if userMap, ok := data.(map[string]interface{}); ok {
        if _, hasRole := userMap["role"]; !hasRole {
            userMap["role"] = "user" // Default role
        }
        if _, hasStatus := userMap["status"]; !hasStatus {
            userMap["status"] = "active" // Default status  
        }
    }
    return data, nil
}
```

## 🌟 Benefits of Major.Minor Versioning

1. **🎯 Semantic Clarity**: Major = breaking, Minor = new features
2. **🚀 Simpler Versions**: No patch numbers needed for API versioning
3. **🔄 Automatic Migration**: Write once for latest, support all versions
4. **📈 Clear Evolution Path**: Easy to understand API progression
5. **🛡️ Backward Compatibility**: Clear expectations for each version type

## 🔧 Usage in Production

```bash
# Request different API versions
curl -H 'x-api-version: 1.0' http://localhost:8080/users  # Basic fields
curl -H 'x-api-version: 1.1' http://localhost:8080/users  # + email
curl -H 'x-api-version: 2.0' http://localhost:8080/users  # + role, status  
curl -H 'x-api-version: 2.1' http://localhost:8080/users  # + timestamps
```

## 📋 Supported Version Formats

Cadwyn-Go supports multiple versioning schemes:

| Format | Example | Use Case |
|--------|---------|----------|
| **Major.Minor** | `1.0`, `2.1` | **API versioning** |
| Major.Minor.Patch | `1.0.0`, `2.1.3` | Full semantic versioning |
| Date-based | `2023-01-01` | Time-based releases |
| Mixed | `1.0` + `2023-01-01` | Flexible versioning |

## 🎉 Conclusion

This example shows that **Cadwyn-Go fully supports major.minor versioning** - perfect for API versioning where you don't need patch numbers but want the semantic meaning of major vs minor changes.

The library automatically handles all migration between versions, so you only need to maintain your latest API and define the transformation rules!
