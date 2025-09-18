package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

// VersionLocation defines where to look for the API version
type VersionLocation string

const (
	VersionLocationHeader VersionLocation = "header"
	VersionLocationQuery  VersionLocation = "query"
	VersionLocationPath   VersionLocation = "path"
)

// VersionMiddleware handles API version detection and migration
type VersionMiddleware struct {
	versionBundle  *version.VersionBundle
	migrationChain *migration.MigrationChain
	parameterName  string
	location       VersionLocation
	defaultVersion *version.Version
}

// Config holds configuration for the version middleware
type Config struct {
	VersionBundle  *version.VersionBundle
	MigrationChain *migration.MigrationChain
	ParameterName  string           // e.g., "x-api-version" for headers, "version" for query
	Location       VersionLocation  // where to look for version
	DefaultVersion *version.Version // fallback version if none specified
}

// NewVersionMiddleware creates a new version middleware
func NewVersionMiddleware(config Config) *VersionMiddleware {
	// Set defaults
	if config.ParameterName == "" {
		switch config.Location {
		case VersionLocationHeader:
			config.ParameterName = "x-api-version"
		case VersionLocationQuery:
			config.ParameterName = "version"
		case VersionLocationPath:
			config.ParameterName = "v"
		default:
			config.ParameterName = "x-api-version"
		}
	}

	if config.DefaultVersion == nil {
		config.DefaultVersion = config.VersionBundle.GetHeadVersion()
	}

	return &VersionMiddleware{
		versionBundle:  config.VersionBundle,
		migrationChain: config.MigrationChain,
		parameterName:  config.ParameterName,
		location:       config.Location,
		defaultVersion: config.DefaultVersion,
	}
}

// Middleware returns the HTTP middleware function
func (vm *VersionMiddleware) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Detect version from request
			requestedVersion := vm.detectVersion(r)

			// Add version to context
			ctx := context.WithValue(r.Context(), versionContextKey, requestedVersion)
			r = r.WithContext(ctx)

			// Create response wrapper to intercept and migrate responses
			rw := &responseWrapper{
				ResponseWriter:   w,
				requestedVersion: requestedVersion,
				headVersion:      vm.versionBundle.GetHeadVersion(),
				migrationChain:   vm.migrationChain,
				ctx:              ctx,
			}

			// Call next handler
			next.ServeHTTP(rw, r)
		})
	}
}

// detectVersion extracts the API version from the HTTP request
func (vm *VersionMiddleware) detectVersion(r *http.Request) *version.Version {
	var versionStr string

	switch vm.location {
	case VersionLocationHeader:
		versionStr = r.Header.Get(vm.parameterName)
	case VersionLocationQuery:
		versionStr = r.URL.Query().Get(vm.parameterName)
	case VersionLocationPath:
		versionStr = vm.extractVersionFromPath(r.URL.Path)
	}

	// If no version specified, use default
	if versionStr == "" {
		return vm.defaultVersion
	}

	// Try to parse the version
	parsedVersion, err := vm.versionBundle.ParseVersion(versionStr)
	if err != nil {
		// If parsing fails, use default version
		return vm.defaultVersion
	}

	return parsedVersion
}

// extractVersionFromPath extracts version from URL path like /v1/users or /2023-01-01/users
func (vm *VersionMiddleware) extractVersionFromPath(path string) string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) == 0 {
		return ""
	}

	firstPart := parts[0]

	// Handle /v1, /v2023-01-01 format
	if strings.HasPrefix(firstPart, vm.parameterName) {
		return strings.TrimPrefix(firstPart, vm.parameterName)
	}

	// Handle direct version format /2023-01-01, /1.0.0
	if vm.looksLikeVersion(firstPart) {
		return firstPart
	}

	return ""
}

// looksLikeVersion checks if a string looks like a version
func (vm *VersionMiddleware) looksLikeVersion(s string) bool {
	// Check if it looks like a date (YYYY-MM-DD)
	if len(s) == 10 && s[4] == '-' && s[7] == '-' {
		return true
	}

	// Check if it looks like semver (X.Y.Z)
	parts := strings.Split(s, ".")
	if len(parts) == 3 {
		return true
	}

	return false
}

// responseWrapper wraps http.ResponseWriter to intercept response writing
type responseWrapper struct {
	http.ResponseWriter
	requestedVersion *version.Version
	headVersion      *version.Version
	migrationChain   *migration.MigrationChain
	ctx              context.Context

	// Response capture
	statusCode int
	body       []byte
	headers    http.Header
	written    bool
}

// Header returns the header map
func (rw *responseWrapper) Header() http.Header {
	if rw.headers == nil {
		rw.headers = make(http.Header)
	}
	return rw.headers
}

// WriteHeader captures the status code
func (rw *responseWrapper) WriteHeader(code int) {
	if rw.written {
		return
	}
	rw.statusCode = code
}

// Write captures the response body
func (rw *responseWrapper) Write(data []byte) (int, error) {
	if rw.written {
		return rw.ResponseWriter.Write(data)
	}

	rw.body = append(rw.body, data...)

	// If this is the final write, process the response
	if err := rw.processResponse(); err != nil {
		http.Error(rw.ResponseWriter, "Version migration error", http.StatusInternalServerError)
		return 0, err
	}

	rw.written = true
	return len(data), nil
}

// processResponse applies version migration to the captured response
func (rw *responseWrapper) processResponse() error {
	// If no migration needed, write directly
	if rw.requestedVersion.Equal(rw.headVersion) {
		return rw.writeResponse()
	}

	// Try to parse and migrate JSON responses
	contentType := rw.Header().Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		return rw.migrateJSONResponse()
	}

	// For non-JSON responses, write as-is
	return rw.writeResponse()
}

// migrateJSONResponse migrates a JSON response
func (rw *responseWrapper) migrateJSONResponse() error {
	if len(rw.body) == 0 {
		return rw.writeResponse()
	}

	// Parse JSON
	var responseData interface{}
	if err := json.Unmarshal(rw.body, &responseData); err != nil {
		// If JSON parsing fails, write as-is
		return rw.writeResponse()
	}

	// Apply migration
	migratedData, err := rw.migrationChain.MigrateResponse(
		rw.ctx,
		responseData,
		rw.headVersion,
		rw.requestedVersion,
	)
	if err != nil {
		return fmt.Errorf("response migration failed: %w", err)
	}

	// Re-serialize JSON
	migratedJSON, err := json.Marshal(migratedData)
	if err != nil {
		return fmt.Errorf("failed to marshal migrated response: %w", err)
	}

	// Update body
	rw.body = migratedJSON

	return rw.writeResponse()
}

// writeResponse writes the final response
func (rw *responseWrapper) writeResponse() error {
	// Copy headers
	for key, values := range rw.headers {
		for _, value := range values {
			rw.ResponseWriter.Header().Add(key, value)
		}
	}

	// Set status code
	if rw.statusCode == 0 {
		rw.statusCode = http.StatusOK
	}
	rw.ResponseWriter.WriteHeader(rw.statusCode)

	// Write body
	if len(rw.body) > 0 {
		_, err := rw.ResponseWriter.Write(rw.body)
		return err
	}

	return nil
}

// Context key for storing version information
type contextKey string

const versionContextKey contextKey = "cadwyn_version"

// GetVersionFromContext extracts the API version from a request context
func GetVersionFromContext(ctx context.Context) *version.Version {
	if v, ok := ctx.Value(versionContextKey).(*version.Version); ok {
		return v
	}
	return nil
}

// RequestMigrationMiddleware provides middleware specifically for request migration
func RequestMigrationMiddleware(versionBundle *version.VersionBundle, migrationChain *migration.MigrationChain) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestedVersion := GetVersionFromContext(r.Context())
			if requestedVersion == nil {
				next.ServeHTTP(w, r)
				return
			}

			headVersion := versionBundle.GetHeadVersion()
			if requestedVersion.Equal(headVersion) {
				next.ServeHTTP(w, r)
				return
			}

			// For POST/PUT/PATCH requests, migrate the request body
			if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodPatch {
				if err := migrateRequestBody(r, requestedVersion, headVersion, migrationChain); err != nil {
					http.Error(w, "Request migration failed", http.StatusBadRequest)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// migrateRequestBody migrates the request body from the requested version to head version
func migrateRequestBody(r *http.Request, from, to *version.Version, migrationChain *migration.MigrationChain) error {
	// This is a simplified implementation
	// In practice, you'd want to parse the request body, migrate it, and replace it
	// For now, we'll just add a header to indicate migration was attempted
	r.Header.Set("X-Cadwyn-Request-Migrated", "true")
	return nil
}
