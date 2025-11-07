# Epoch OpenAPI Schema Generation

Generate versioned OpenAPI 3.0.x specs from your Epoch-versioned Go API.

## Overview

The OpenAPI schema generation library creates separate, version-specific OpenAPI specifications from your Epoch type registry and version migrations. It uses reflection to introspect Go struct types, extracts validation constraints from struct tags, and applies version transformations to generate accurate schemas for each API version.

## Features

- ✅ **Reflection-based type parsing** - Automatically generates schemas from Go types
- ✅ **Full type support** - Primitives, structs, slices, maps, interfaces, pointers, embedded structs
- ✅ **Dual tag support** - Parses both `binding` (request) and `validate` (response) tags
- ✅ **Version transformations** - Applies Epoch migrations to generate version-specific schemas
- ✅ **Separate specs per version** - Clean, standalone OpenAPI files for each version

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│  TypeParser                                                   │
│  - Uses reflect to inspect Go struct types                   │
│  - Generates OpenAPI schemas with proper constraints         │
│  - Handles nested types, arrays, maps, interfaces            │
└───────────────────────┬───────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  TagParser                                                    │
│  - Extracts validation rules from struct tags                │
│  - binding:"required,max=50,email" → OpenAPI constraints     │
│  - validate:"required" → required field                       │
│  - example, enums, format, description tags                  │
└───────────────────────┬───────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  VersionTransformer                                           │
│  - Applies Epoch VersionChange operations to schemas         │
│  - Request: walks FORWARD (Client→HEAD)                      │
│  - Response: walks BACKWARD (HEAD→Client)                    │
│  - Handles add/remove/rename field operations                │
└───────────────────────┬───────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  SchemaGenerator                                              │
│  - Orchestrates type parsing and version transformations     │
│  - Generates complete OpenAPI specs per version              │
│  - Replaces components/schemas with versioned types          │
└───────────────────────┬───────────────────────────────────────┘
                        │
                        ▼
┌─────────────────────────────────────────────────────────────┐
│  Writer                                                       │
│  - Outputs specs in YAML or JSON                             │
│  - Validates specs using kin-openapi                         │
│  - Handles file naming conventions                           │
└─────────────────────────────────────────────────────────────┘
```

## Type Support Matrix

| Go Type | OpenAPI Type | Notes |
|---------|--------------|-------|
| `string` | `string` | |
| `int`, `int64` | `integer` (format: int64) | |
| `int32` | `integer` (format: int32) | |
| `float32` | `number` (format: float) | |
| `float64` | `number` (format: double) | |
| `bool` | `boolean` | |
| `time.Time` | `string` (format: date-time) | |
| `[]T` | `array` (items: T) | |
| `[N]T` | `array` (minItems/maxItems: N) | |
| `*T` | Same as T (not in required) | Pointer = optional |
| `map[string]T` | `object` (additionalProperties: T) | |
| `map[string]interface{}` | `object` (additionalProperties: true) | |
| `interface{}` | `object` (free-form) | |
| `struct` | `object` (with properties) | |
| Embedded struct | Properties promoted | |

## Tag Parsing Reference

### Request Tags (`binding`)

Used with request structs (e.g., `CreateUserRequest`):

```go
type CreateUserRequest struct {
    Name   string `json:"name" binding:"required,max=50"`
    Email  string `json:"email" binding:"required,email"`
    Age    int    `json:"age" binding:"min=18,max=100"`
    Status string `json:"status" binding:"oneof=active inactive pending"`
}
```

Generates:
```yaml
CreateUserRequest:
  type: object
  required: [name, email]
  properties:
    name:
      type: string
      maxLength: 50
    email:
      type: string
      format: email
    age:
      type: integer
      minimum: 18
      maximum: 100
    status:
      type: string
      enum: [active, inactive, pending]
```

### Response Tags (`validate`)

Used with response structs:

```go
type UserResponse struct {
    ID    int    `json:"id" validate:"required"`
    Name  string `json:"name" validate:"required,max=100"`
    Email string `json:"email,omitempty" validate:"email"`
}
```

### Common Tags

Apply to both request and response:

```go
type Response struct {
    Status  string     `json:"status" enums:"OK,ERROR,PENDING" example:"OK"`
    Created *time.Time `json:"created" format:"date-time" description:"Creation timestamp"`
}
```

### Supported Validators

| Tag | OpenAPI Constraint | Example |
|-----|-------------------|---------|
| `required` | In `required` array | `binding:"required"` |
| `max=N` | `maxLength` (string) or `maximum` (number) | `binding:"max=50"` |
| `min=N` | `minLength` (string) or `minimum` (number) | `binding:"min=1"` |
| `len=N` | `minLength` + `maxLength` = N | `binding:"len=10"` |
| `email` | `format: email` | `binding:"email"` |
| `url` | `format: uri` | `binding:"url"` |
| `uuid` | `format: uuid` | `binding:"uuid"` |
| `oneof=A B C` | `enum: [A, B, C]` | `binding:"oneof=a b c"` |
| `gt=N` | `minimum` (exclusive) | `binding:"gt=0"` |
| `gte=N` | `minimum` | `binding:"gte=0"` |
| `lt=N` | `maximum` (exclusive) | `binding:"lt=100"` |
| `lte=N` | `maximum` | `binding:"lte=100"` |
| `enums=A,B,C` | `enum: [A, B, C]` | `enums:"ACTIVE,INACTIVE"` |
| `example=value` | `example: value` | `example:"test"` |
| `format=fmt` | `format: fmt` | `format:"date-time"` |
| `description=desc` | `description: desc` | `description:"User ID"` |

## Usage

### Basic Integration

```go
import "github.com/astronomer/epoch/epoch/openapi"

// After creating your Epoch instance
epochInstance, _ := epoch.NewEpoch().
    WithVersions(v1, v2, v3).
    WithChanges(userV1ToV2, userV2ToV3).
    WithTypes(UserResponse{}, CreateUserRequest{}).
    Build()

// Create schema generator
generator := openapi.NewSchemaGenerator(openapi.SchemaGeneratorConfig{
    VersionBundle: epochInstance.VersionBundle,
    TypeRegistry:  epochInstance.TypeRegistry,
    OutputFormat:  "yaml", // or "json"
})

// Load your existing OpenAPI spec (from swag or other tools)
baseSpec, _ := loadOpenAPISpec("docs/api.yaml")

// Generate versioned specs
versionedSpecs, err := generator.GenerateVersionedSpecs(baseSpec)
if err != nil {
    log.Fatal(err)
}

// Write specs to files
for version, spec := range versionedSpecs {
    filename := fmt.Sprintf("docs/api_%s.yaml", version)
    generator.WriteVersionedSpecs(map[string]*openapi3.T{version: spec}, filename)
}
```

### Integration with Existing Spec Pipeline

```go
// In your cmd/spec/main.go (after swag generation and v2→v3 conversion)

func generateEpochVersionedSpecs(logger *logging.Logger) error {
    // Define your versions and migrations
    v1, _ := epoch.NewDateVersion("2024-01-01")
    v2, _ := epoch.NewDateVersion("2024-06-01")
    
    change := epoch.NewVersionChangeBuilder(v1, v2).
        ForType(UserResponse{}).
        ResponseToPreviousVersion().
            RemoveField("email").
        Build()
    
    // Create version bundle
    versionBundle, _ := epoch.NewVersionBundle([]*epoch.Version{v2, v1})
    v2.Changes = []epoch.VersionChangeInterface{change}
    
    // Create type registry (populated from your endpoint registrations)
    registry := epoch.NewEndpointRegistry()
    // ... register your endpoints ...
    
    // Create generator
    generator := openapi.NewSchemaGenerator(openapi.SchemaGeneratorConfig{
        VersionBundle: versionBundle,
        TypeRegistry:  registry,
        OutputFormat:  "yaml",
    })
    
    // Load HEAD spec
    baseSpec, err := loadOpenAPISpec("docs/public/v1alpha1/public_v1alpha1.yaml")
    if err != nil {
        return err
    }
    
    // Generate versioned specs
    versionedSpecs, err := generator.GenerateVersionedSpecs(baseSpec)
    if err != nil {
        return err
    }
    
    // Write each version
    for versionStr, spec := range versionedSpecs {
        outPath := fmt.Sprintf("docs/public/v1alpha1/public_v1alpha1_%s.yaml", versionStr)
        if err := writeOpenAPISpec(outPath, spec); err != nil {
            return err
        }
        logger.Info("Generated versioned spec", "version", versionStr, "path", outPath)
    }
    
    return nil
}
```

## Version Transformations

### How It Works

The generator applies Epoch version migrations to schemas:

**For Response Schemas** (HEAD → Client):
1. Start with HEAD version schema (parsed from Go struct)
2. Walk BACKWARD through version chain
3. Apply `ResponseToPreviousVersion()` operations in reverse
4. Result: schema representing what older clients receive

**For Request Schemas** (Client → HEAD):
1. Start with HEAD version schema
2. Walk FORWARD through version chain
3. Apply `RequestToNextVersion()` operations in sequence
4. Result: schema representing what older clients send

### Example

```go
// HEAD version
type UserResponse struct {
    ID       int    `json:"id"`
    FullName string `json:"full_name"`  // Renamed from "name" in v2
    Email    string `json:"email"`       // Added in v2
    Phone    string `json:"phone"`       // Added in v3
}

// Version migrations
v1ToV2 := NewVersionChangeBuilder(v1, v2).
    ForType(UserResponse{}).
    ResponseToPreviousVersion().
        RemoveField("email").  // v1 doesn't have email
    Build()

v2ToV3 := NewVersionChangeBuilder(v2, v3).
    ForType(UserResponse{}).
    ResponseToPreviousVersion().
        RenameField("full_name", "name").  // v2 calls it "name"
        RemoveField("phone").              // v2 doesn't have phone
    Build()
```

**Generated Schemas**:

```yaml
# HEAD / v3
UserResponseV20250101:
  type: object
  properties:
    id: {type: integer}
    full_name: {type: string}
    email: {type: string}
    phone: {type: string}

# v2
UserResponseV20240601:
  type: object
  properties:
    id: {type: integer}
    name: {type: string}        # Renamed from full_name
    email: {type: string}
    # phone removed

# v1
UserResponseV20240101:
  type: object
  properties:
    id: {type: integer}
    name: {type: string}
    # email removed
    # phone removed
```

## Output Structure

```
docs/
└── public/
    └── v1alpha1/
        ├── public_v1alpha1.yaml              # HEAD version (from swag)
        ├── public_v1alpha1_2024-01-01.yaml   # v1 with transformed schemas
        ├── public_v1alpha1_2024-06-01.yaml   # v2 with transformed schemas
        └── public_v1alpha1_2025-01-01.yaml   # v3 with transformed schemas
```

## Client Generation

Use versioned specs to generate language-specific clients:

### TypeScript/JavaScript

```json
{
  "scripts": {
    "apigen:v1": "openapi-generator-cli generate -i ../../core/docs/public/v1alpha1/public_v1alpha1_2024-01-01.yaml -g typescript-axios -o src/generated/v1",
    "apigen:v2": "openapi-generator-cli generate -i ../../core/docs/public/v1alpha1/public_v1alpha1_2024-06-01.yaml -g typescript-axios -o src/generated/v2",
    "apigen:all": "npm run apigen:v1 && npm run apigen:v2"
  }
}
```

### Go

```makefile
.PHONY: generate-client-v1
generate-client-v1:
	openapi-generator-cli generate \
		-i ../core/docs/public/v1alpha1/public_v1alpha1_2024-01-01.yaml \
		-g go \
		-o v1/
```

### Python

```makefile
.PHONY: generate-client-v1
generate-client-v1:
	openapi-generator-cli generate \
		-i ../core/docs/public/v1alpha1/public_v1alpha1_2024-01-01.yaml \
		-g python \
		-o v1/
```

## Current Status

### ✅ Completed

- Core package structure
- TypeParser with full type support (primitives, structs, slices, maps, interfaces)
- TagParser for validation constraints (binding + validate tags)
- VersionTransformer for bidirectional schema transformations
- SchemaGenerator with version orchestration
- Writer for YAML/JSON output
- Comprehensive unit tests
- Integration example
- Documentation

### ⚠️ Known Issues

The `kin-openapi` library was recently updated to v0.133.0, which introduced breaking API changes:

1. `Schema.Type` changed from `string` to `*openapi3.Types`
2. `Schema.AdditionalProperties` changed type
3. Need to use `Type.Is(string)` instead of direct string comparison

**Resolution needed**: Update all type comparisons and schema constructions to use the new API.

### 🔜 Next Steps

1. Fix compatibility with kin-openapi v0.133.0 API
2. Complete version transformer operation extraction
3. Run full integration tests with examples/advanced types
4. Add Makefile integration examples
5. Create client generation guide

## Troubleshooting

### "Component not found" errors

Make sure all types are registered via `WrapHandler().Returns()/.Accepts()`:

```go
r.GET("/users/:id", epochInstance.WrapHandler(getUser).
    Returns(UserResponse{}).  // ← Must register type
    ToHandlerFunc())
```

### Missing validation constraints

Check that struct tags use correct format:

```go
// ✅ Correct
`json:"name" binding:"required,max=50"`

// ❌ Wrong
`json:"name" binding="required,max=50"`  // Wrong: = instead of :
```

### Schema not transforming

Verify that VersionChange targets the correct type:

```go
// Must match exact type used in WrapHandler()
ForType(UserResponse{})  // ✅ Correct
ForType(&UserResponse{}) // ❌ Different type (pointer)
```

## Contributing

See main Epoch README for contribution guidelines.

## License

MIT License - see main Epoch LICENSE file.

