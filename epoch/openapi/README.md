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
import (
    "fmt"
    "log"
    
    "github.com/astronomer/epoch/epoch"
    "github.com/astronomer/epoch/epoch/openapi"
    "github.com/getkin/kin-openapi/openapi3"
)

// After creating your Epoch instance
epochInstance, _ := epoch.NewEpoch().
    WithHeadVersion().
    WithVersions(v1, v2, v3).
    WithChanges(userV1ToV2, userV2ToV3).
    WithTypes(UserResponse{}, CreateUserRequest{}).
    Build()

// Create schema generator
generator := openapi.NewSchemaGenerator(openapi.SchemaGeneratorConfig{
    VersionBundle: epochInstance.VersionBundle(),  // Note: method call
    TypeRegistry:  epochInstance.EndpointRegistry(),  // Note: changed from TypeRegistry
    OutputFormat:  "yaml", // or "json"
})

// Load your existing OpenAPI spec (from swag or other tools)
loader := openapi3.NewLoader()
loader.IsExternalRefsAllowed = true
baseSpec, err := loader.LoadFromFile("docs/api.yaml")
if err != nil {
    log.Fatalf("Failed to load base spec: %v", err)
}

// Generate versioned specs
versionedSpecs, err := generator.GenerateVersionedSpecs(baseSpec)
if err != nil {
    log.Fatal(err)
}

// Write specs to files
filenamePattern := "docs/api_%s.yaml"
if err := generator.WriteVersionedSpecs(versionedSpecs, filenamePattern); err != nil {
    log.Fatalf("Failed to write specs: %v", err)
}

fmt.Println("✓ Generated versioned OpenAPI specs")
```

### Swag Integration Guide

This section provides a complete workflow for integrating Epoch's OpenAPI schema generator with Swag.

#### Step 1: Add Swag Annotations

Annotate handlers with Swag comments and register types with Epoch:

```go
// @Summary Get user by ID
// @Success 200 {object} UserResponse
// @Router /users/{id} [get]
func (h *Handler) GetUser(c *gin.Context) {
    epochInstance.WrapHandler(h.getUserImpl).
        Returns(UserResponse{}).  // Must match @Success type
        ToHandlerFunc()(c)
}
```

Types in `@Success`, `@Param body` annotations must match `.Returns()` and `.Accepts()` calls.

#### Step 2: Generate Base Spec with Swag

Run Swag to generate your base OpenAPI specification:

```bash
# Install swag if needed
go install github.com/swaggo/swag/cmd/swag@latest

# Generate OpenAPI spec (creates docs/swagger.json and docs/swagger.yaml)
swag init -g cmd/api/main.go -o docs/swagger --parseDependency --parseInternal
```

This creates a base spec with:
- All your endpoints (paths and operations)
- Schema definitions with package prefixes (e.g., `versionedapi.UserResponse`)
- Parameter definitions
- Security schemes

**Key Detail**: Swag names schemas using the package name as a prefix. For example, if your types are in the `versionedapi` package, the schema will be named `versionedapi.UserResponse`.

#### Step 3: Configure Epoch Schema Generator

Create a schema generation command:

```go
// Configure with SchemaNameMapper to match Swag's naming
generator := openapi.NewSchemaGenerator(openapi.SchemaGeneratorConfig{
    VersionBundle: epochInstance.VersionBundle(),
    TypeRegistry:  epochInstance.EndpointRegistry(),
    OutputFormat:  "yaml",
    SchemaNameMapper: func(typeName string) string {
        return "versionedapi." + typeName  // Match Swag's package prefix
    },
})

// Load base spec, generate, and write
loader := openapi3.NewLoader()
loader.IsExternalRefsAllowed = true
baseSpec, _ := loader.LoadFromFile("docs/swagger/swagger.yaml")
versionedSpecs, _ := generator.GenerateVersionedSpecs(baseSpec)
generator.WriteVersionedSpecs(versionedSpecs, "docs/api/api_%s.yaml")
```

See "Basic Integration" section above for complete example with error handling and Epoch instance creation.

#### Step 4: Generate Specs

```bash
go run cmd/schema/main.go
# Outputs: docs/api/api_2024-01-01.yaml, api_2024-06-01.yaml, api_head.yaml, etc.
```

#### Common Issues

**Schema Not Found**: Ensure `SchemaNameMapper` matches Swag's naming (check base spec with `grep "schemas:" docs/swagger/swagger.yaml`)

**Type Not Registered**: Must call `.Returns()` or `.Accepts()` on `WrapHandler()`

**Package Prefix Mismatch**: Update `SchemaNameMapper` to match what Swag generates (e.g., `"versionedapi." + typeName`)

#### Makefile Integration

```makefile
generate-openapi:
	swag init -g cmd/api/main.go -o docs/swagger --parseDependency
	go run cmd/schema/main.go
```

## Smart Transform Behavior

**Breaking Change**: As of this version, the schema generator uses smart transform logic:

1. **For each registered type:**
   - Maps the Go type name to a schema name (via `SchemaNameMapper`)
   - If schema exists in base spec → **transforms it in place** with version migrations
   - If schema doesn't exist → **generates it from scratch** using reflection

2. **This design works for:**
   - ✅ Swag-generated specs (transforms existing schemas)
   - ✅ Empty specs (generates schemas from scratch)
   - ✅ Mixed scenarios (some schemas exist, some don't)

### Schema Naming Conventions

The generator uses different naming strategies depending on whether schemas are transformed or generated:

**When transforming existing schemas** (found in base spec):
- Uses the mapped schema name from `SchemaNameMapper`
- Preserves the same name across all versions
- Example: `versionedapi.UpdateExampleRequest` in all version files

**When generating from scratch** (not found in base spec):
- **HEAD version**: Uses bare type name (e.g., `UserResponse`)
- **Versioned releases**: Uses type name + version suffix (e.g., `UserResponseV20240101`)
- Version suffix format: `V` + date without hyphens (e.g., `V20240101`)

**Unmanaged schemas** (in base spec but not in TypeRegistry):
- Preserved as-is in all versions
- Examples: `ErrorResponse`, `PaginationMeta`, common utility types
- Allows mixing Epoch-managed and standard schemas

### Configuration Options

**`SchemaNameMapper`**: Maps Go type names to OpenAPI schema names (useful for Swag integration with package prefixes)

**`ComponentNamePrefix`**: Adds prefix to version suffixes (e.g., `"Astro"` → `UserResponseAstroV20240101`)

**`OutputFormat`**: `"yaml"` or `"json"`

**`IncludeMigrationMetadata`**: Adds `x-epoch-migrations` extensions to schemas

### Two Generation Paths

**Path 1: Transform Existing Schema** (base spec has schema)
- Preserves schema name across all versions via `SchemaNameMapper`
- Transforms content per version based on migrations
- Example: `versionedapi.UserResponse` in all versions, with different fields

**Path 2: Generate From Scratch** (base spec missing schema)
- HEAD: uses bare name (`UserResponse`)
- Versioned: uses versioned name (`UserResponseV20240101`)
- Content generated from Go types + migrations applied

## Version Transformations

**Response schemas** (HEAD → Client): Walk BACKWARD through version chain, applying `ResponseToPreviousVersion()` operations to represent what older clients receive.

**Request schemas** (Client → HEAD): Walk FORWARD through version chain, applying `RequestToNextVersion()` operations to represent what older clients send.

Migrations stack: if v1→v2 removes email and v2→v3 renames name→full_name, then v1 sees both transformations applied (no email, uses name instead of full_name).

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

## What Gets Preserved vs Transformed

**Preserved across all versions:**
- Paths, operations, security schemes, tags, descriptions
- Parameters, headers, responses (non-schema components)
- Unmanaged schemas (not in TypeRegistry): `ErrorResponse`, `PaginationMeta`, etc.

**Transformed per version:**
- Only schemas registered in Epoch's TypeRegistry via `.Returns()` / `.Accepts()`

This lets you version your data models while keeping common error/pagination schemas and API docs unchanged.

## Client Generation

Use tools like `openapi-generator-cli` to generate language-specific clients from versioned specs:

```bash
# TypeScript
openapi-generator-cli generate -i docs/api_2024-01-01.yaml -g typescript-axios -o clients/v1

# Go
openapi-generator-cli generate -i docs/api_2024-01-01.yaml -g go -o clients/v1

# Python
openapi-generator-cli generate -i docs/api_2024-01-01.yaml -g python -o clients/v1
```

## Troubleshooting

**"Component not found"**: Register types via `WrapHandler().Returns(UserResponse{})`

**Missing validation constraints**: Use correct tag syntax: `binding:"required"` not `binding="required"`

**Schema not transforming**: Ensure `ForType(UserResponse{})` matches the exact type in `WrapHandler()` (not pointer)

## Contributing

See main Epoch README for contribution guidelines.

## License

MIT License - see main Epoch LICENSE file.

