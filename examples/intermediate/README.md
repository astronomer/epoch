# Intermediate Example - Production API Server

This example demonstrates a production-ready API server with multiple versions, full CRUD operations, and automatic request/response migration. Perfect for learning how to build real applications with Cadwyn-Go.

## API Versions

### Version 1.0 (2023-01-01)
- Basic user management
- Fields: `id`, `name`

### Version 2.0 (2023-06-01)  
- Added email field
- Fields: `id`, `name`, `email`

### Version 3.0 (2024-01-01)
- Added status and timestamps
- Fields: `id`, `name`, `email`, `status`, `created_at`, `updated_at`

## Features Demonstrated

- ✅ **Multiple API Versions**: Three versions with progressive field additions
- ✅ **Automatic Migration**: Requests and responses are automatically converted between versions
- ✅ **Version Detection**: Uses `x-api-version` header
- ✅ **Fluent Builder API**: Easy application setup
- ✅ **Full CRUD Operations**: Create, read, update, delete users
- ✅ **Query Parameters**: Filter users by status (v3.0 feature)
- ✅ **Schema Analysis**: Optional struct introspection
- ✅ **Debug Logging**: Route and version information

## Running the Example

```bash
cd examples/intermediate
go run main.go
```

The server will start on `http://localhost:8080`.

## Testing Different Versions

### Get API Information
```bash
curl http://localhost:8080/api/info
```

### Get Users (Different Versions)

**Version 1.0** (only id and name):
```bash
curl -H 'x-api-version: 2023-01-01' http://localhost:8080/users
```

**Version 2.0** (adds email):
```bash
curl -H 'x-api-version: 2023-06-01' http://localhost:8080/users
```

**Version 3.0** (adds status and timestamps):
```bash
curl -H 'x-api-version: 2024-01-01' http://localhost:8080/users
```

### Create User (Version-Aware)

**Create user in v1.0** (only name required):
```bash
curl -X POST -H 'Content-Type: application/json' -H 'x-api-version: 2023-01-01' \
     -d '{"name":"John Doe"}' \
     http://localhost:8080/users
```

**Create user in v2.0** (name and email):
```bash
curl -X POST -H 'Content-Type: application/json' -H 'x-api-version: 2023-06-01' \
     -d '{"name":"Jane Smith","email":"jane@example.com"}' \
     http://localhost:8080/users
```

**Create user in v3.0** (all fields):
```bash
curl -X POST -H 'Content-Type: application/json' -H 'x-api-version: 2024-01-01' \
     -d '{"name":"Bob Johnson","email":"bob@example.com","status":"active"}' \
     http://localhost:8080/users
```

### Get Specific User
```bash
curl -H 'x-api-version: 2023-06-01' http://localhost:8080/users/1
```

### Update User
```bash
curl -X PUT -H 'Content-Type: application/json' -H 'x-api-version: 2024-01-01' \
     -d '{"name":"Alice Updated","status":"inactive"}' \
     http://localhost:8080/users/1
```

### Delete User
```bash
curl -X DELETE -H 'x-api-version: 2024-01-01' http://localhost:8080/users/1
```

### Filter Users (v3.0 Feature)
```bash
# Get only active users
curl -H 'x-api-version: 2024-01-01' 'http://localhost:8080/users?status=active'

# Get only inactive users  
curl -H 'x-api-version: 2024-01-01' 'http://localhost:8080/users?status=inactive'
```

## How It Works

### 1. Version Changes
The example defines two version changes:
- `V1ToV2Change`: Adds email field
- `V2ToV3Change`: Adds status and timestamp fields

### 2. Automatic Migration
When a client requests data in an older version:
1. **Request Migration**: Client data is migrated to the latest version for processing
2. **Response Migration**: Server response is migrated back to the requested version

### 3. Field Handling
- **Added Fields**: Default values provided during migration
- **Removed Fields**: Fields are stripped during backward migration
- **Preserved Fields**: Common fields remain unchanged

### 4. Example Migration Flow

**Client requests v1.0 data:**
```
Client (v1.0) → [Request Migration] → Server (v3.0) → [Response Migration] → Client (v1.0)
```

**Migration steps:**
1. Client sends `{"name": "John"}` with `x-api-version: 2023-01-01`
2. Request migrated to v3.0: `{"name": "John", "email": "", "status": "active", ...}`
3. Server processes with full v3.0 model
4. Response migrated back to v1.0: `{"id": 1, "name": "John"}`
5. Client receives v1.0 format

## Key Code Sections

### Version Change Implementation
```go
func (c *V1ToV2Change) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
    // Convert UserV2 to UserV1 by removing email field
    if userMap, ok := data.(map[string]interface{}); ok {
        delete(userMap, "email")
        return userMap, nil
    }
    return data, nil
}
```

### Application Setup
```go
app, err := cadwyn.NewBuilder().
    WithDateVersions("2023-01-01", "2023-06-01", "2024-01-01").
    WithVersionChanges(NewV1ToV2Change(), NewV2ToV3Change()).
    WithSchemaAnalysis().
    WithDebugLogging().
    Build()
```

### Route Registration
```go
router := app.Router()
router.GET("/users", handleGetUsers)
router.POST("/users", handleCreateUser)
// Server implements http.Handler directly
server := &http.Server{Addr: ":8080", Handler: app}
```

This example showcases the power of Cadwyn-Go: **write once for the latest version, support all previous versions automatically**.
