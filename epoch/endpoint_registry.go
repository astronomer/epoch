package epoch

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// NestedTypeInfo describes a nested type within a struct
type NestedTypeInfo struct {
	Path    string       // JSON path e.g., "metadata" or "profile.skills"
	Type    reflect.Type // The nested type
	IsArray bool         // True if it's an array/slice field
}

// EndpointDefinition stores type information for a specific endpoint
type EndpointDefinition struct {
	Method                string
	PathPattern           string                  // e.g., "/users/:id"
	RequestType           reflect.Type            // Type for request body
	ResponseType          reflect.Type            // Type for response body
	RequestNestedArrays   map[string]reflect.Type // field path → item type for request nested arrays (auto-populated)
	RequestNestedObjects  map[string]reflect.Type // field path → type for request nested objects (auto-populated)
	ResponseNestedArrays  map[string]reflect.Type // field path → item type for response nested arrays (auto-populated)
	ResponseNestedObjects map[string]reflect.Type // field path → type for response nested objects (auto-populated)
}

// EndpointRegistry stores and manages endpoint→type mappings
type EndpointRegistry struct {
	mu        sync.RWMutex
	endpoints map[string]*EndpointDefinition // key: "METHOD:path_pattern"
}

// NewEndpointRegistry creates a new endpoint registry
func NewEndpointRegistry() *EndpointRegistry {
	return &EndpointRegistry{
		endpoints: make(map[string]*EndpointDefinition),
	}
}

// Register stores an endpoint definition
func (er *EndpointRegistry) Register(method, pathPattern string, def *EndpointDefinition) {
	er.mu.Lock()
	defer er.mu.Unlock()

	key := er.makeKey(method, pathPattern)
	er.endpoints[key] = def
}

// Lookup finds an endpoint definition by matching the request method and path
// Handles path parameters like :id and *wildcard
func (er *EndpointRegistry) Lookup(method, requestPath string) (*EndpointDefinition, error) {
	er.mu.RLock()
	defer er.mu.RUnlock()

	// Try exact match first
	exactKey := er.makeKey(method, requestPath)
	if def, exists := er.endpoints[exactKey]; exists {
		return def, nil
	}

	// Try pattern matching for parameterized paths
	for key, def := range er.endpoints {
		if !strings.HasPrefix(key, method+":") {
			continue
		}

		if er.pathMatches(def.PathPattern, requestPath) {
			return def, nil
		}
	}

	return nil, fmt.Errorf("no endpoint registered for %s %s", method, requestPath)
}

// pathMatches checks if a request path matches a path pattern with parameters
// Examples:
//   - "/users/:id" matches "/users/123"
//   - "/files/*filepath" matches "/files/path/to/file.txt"
func (er *EndpointRegistry) pathMatches(pattern, path string) bool {
	// Convert Gin path pattern to regex
	// :param becomes ([^/]+)
	// *wildcard becomes (.*)
	regexPattern := "^" + regexp.QuoteMeta(pattern) + "$"
	regexPattern = strings.ReplaceAll(regexPattern, "\\:", ":")
	regexPattern = strings.ReplaceAll(regexPattern, "\\*", "*")

	// Replace :param with regex group
	regexPattern = regexp.MustCompile(`:([^/]+)`).ReplaceAllString(regexPattern, `([^/]+)`)
	// Replace *wildcard with regex group
	regexPattern = regexp.MustCompile(`\*([^/]+)`).ReplaceAllString(regexPattern, `(.*)`)

	matched, _ := regexp.MatchString(regexPattern, path)
	return matched
}

// makeKey creates a unique key for an endpoint
func (er *EndpointRegistry) makeKey(method, path string) string {
	return method + ":" + path
}

// GetAll returns all registered endpoints (for debugging/introspection)
func (er *EndpointRegistry) GetAll() map[string]*EndpointDefinition {
	er.mu.RLock()
	defer er.mu.RUnlock()

	// Return a copy to prevent external modification
	result := make(map[string]*EndpointDefinition, len(er.endpoints))
	for k, v := range er.endpoints {
		result[k] = v
	}
	return result
}

// AnalyzeStructFields recursively analyzes struct fields to discover nested types
// Returns a list of NestedTypeInfo describing all nested structs and arrays
func AnalyzeStructFields(t reflect.Type, prefix string, ancestors []reflect.Type) []NestedTypeInfo {
	var result []NestedTypeInfo

	// Handle nil or invalid types
	if t == nil {
		return result
	}

	// Dereference pointer types
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Only analyze structs
	if t.Kind() != reflect.Struct {
		return result
	}

	// Prevent infinite recursion for circular references
	// Check if this type is already in our ancestor chain (call stack)
	for _, ancestor := range ancestors {
		if ancestor == t {
			return result // Circular reference detected, stop recursion
		}
	}

	// Add current type to ancestors for recursive calls
	// Create a new slice to avoid modifying the caller's view
	currentAncestors := make([]reflect.Type, len(ancestors)+1)
	copy(currentAncestors, ancestors)
	currentAncestors[len(ancestors)] = t

	// Iterate through struct fields
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON field name
		jsonName := getJSONFieldName(field)
		if jsonName == "-" {
			continue // Skip fields marked with json:"-"
		}

		// Build the full path
		var path string
		if prefix == "" {
			path = jsonName
		} else {
			path = prefix + "." + jsonName
		}

		fieldType := field.Type

		// Dereference pointer types
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
		}

		switch fieldType.Kind() {
		case reflect.Struct:
			// Skip time.Time and other common non-nested types
			if isBuiltinType(fieldType) {
				continue
			}

			// Found a nested struct
			result = append(result, NestedTypeInfo{
				Path:    path,
				Type:    fieldType,
				IsArray: false,
			})

			// Recursively analyze nested struct
			// Pass currentAncestors - each recursive call gets its own view
			nestedResults := AnalyzeStructFields(fieldType, path, currentAncestors)
			result = append(result, nestedResults...)

		case reflect.Slice, reflect.Array:
			elemType := fieldType.Elem()

			// Dereference pointer element types
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}

			// Only care about slices of structs
			if elemType.Kind() == reflect.Struct && !isBuiltinType(elemType) {
				result = append(result, NestedTypeInfo{
					Path:    path,
					Type:    elemType,
					IsArray: true,
				})

				// Recursively analyze array element type for deeper nesting
				// Pass currentAncestors - each recursive call gets its own view
				nestedResults := AnalyzeStructFields(elemType, path, currentAncestors)
				result = append(result, nestedResults...)
			}
		}
	}

	return result
}

// getJSONFieldName extracts the JSON field name from a struct field
func getJSONFieldName(field reflect.StructField) string {
	jsonTag := field.Tag.Get("json")
	if jsonTag == "" {
		return field.Name
	}

	// Handle json:"fieldname,omitempty" format
	parts := strings.Split(jsonTag, ",")
	if parts[0] == "" {
		return field.Name
	}
	return parts[0]
}

// isBuiltinType checks if a type is a builtin type that shouldn't be recursed into
func isBuiltinType(t reflect.Type) bool {
	// Check for common types that look like structs but aren't "nested objects"
	typeName := t.String()
	builtins := []string{
		"time.Time",
		"json.RawMessage",
		"uuid.UUID",
		"decimal.Decimal",
	}
	for _, builtin := range builtins {
		if typeName == builtin {
			return true
		}
	}
	return false
}

// BuildNestedTypeMaps builds NestedArrays and NestedObjects maps from struct analysis
func BuildNestedTypeMaps(t reflect.Type) (nestedArrays, nestedObjects map[string]reflect.Type) {
	nestedArrays = make(map[string]reflect.Type)
	nestedObjects = make(map[string]reflect.Type)

	if t == nil {
		return
	}

	// Dereference pointer
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	// Handle slice/array types at top level
	if t.Kind() == reflect.Slice || t.Kind() == reflect.Array {
		// Top-level arrays don't need nested registration
		return
	}

	infos := AnalyzeStructFields(t, "", nil)
	for _, info := range infos {
		if info.IsArray {
			nestedArrays[info.Path] = info.Type
		} else {
			nestedObjects[info.Path] = info.Type
		}
	}

	return
}
