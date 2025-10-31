package epoch

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"sync"
)

// EndpointDefinition stores type information for a specific endpoint
type EndpointDefinition struct {
	Method       string
	PathPattern  string                  // e.g., "/users/:id"
	RequestType  reflect.Type            // Type for request body
	ResponseType reflect.Type            // Type for response body
	NestedArrays map[string]reflect.Type // field name → item type for nested arrays
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
