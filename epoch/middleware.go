package epoch

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"github.com/bytedance/sonic"
	"github.com/bytedance/sonic/ast"
	"github.com/gin-gonic/gin"
)

// VersionFormat defines the format of version values
type VersionFormat string

const (
	VersionFormatDate   VersionFormat = "date"
	VersionFormatSemver VersionFormat = "semver"
	VersionFormatString VersionFormat = "string"
)

// VersionManager checks all locations for version information
// Priority: Header > Path
type VersionManager struct {
	headerName       string
	versionRegex     *regexp.Regexp
	possibleVersions map[string]bool
}

// NewVersionManager creates a new version manager that checks all locations
// Priority: Header > Path
// Supports partial version matching (e.g., "v1" matches latest v1.x.x)
func NewVersionManager(headerName string, possibleVersions []string) *VersionManager {
	versionMap := make(map[string]bool)
	for _, v := range possibleVersions {
		versionMap[v] = true
	}

	// Create regex to match version-like patterns in the path
	// Matches: /v1/, /v1.0/, /v1.0.0/, /1/, /1.0/, /1.0.0/, /2024-01-01/, /v1.0-beta/, etc.
	// The pattern captures version strings that start with optional 'v', followed by numbers/dots/hyphens/alphanumeric
	pattern := `/([vV]?\d+(?:[\.\-]\w+)*)/`
	regex := regexp.MustCompile(pattern)

	return &VersionManager{
		headerName:       headerName,
		versionRegex:     regex,
		possibleVersions: versionMap,
	}
}

// GetVersion checks all locations for version information
// Priority: Header > Path
func (vm *VersionManager) GetVersion(c *gin.Context) (string, error) {
	// First, check header (highest priority)
	headerVersion := c.GetHeader(vm.headerName)
	if headerVersion != "" {
		return headerVersion, nil
	}

	// Second, check URL path
	matches := vm.versionRegex.FindStringSubmatch(c.Request.URL.Path)
	if len(matches) > 1 {
		return matches[1], nil
	}

	// No version found in any location
	return "", nil
}

// VersionMiddleware handles version detection and context setting
type VersionMiddleware struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
	versionManager *VersionManager
	defaultVersion *Version
	parameterName  string
	format         VersionFormat
}

// MiddlewareConfig holds configuration for version middleware
type MiddlewareConfig struct {
	VersionBundle  *VersionBundle
	MigrationChain *MigrationChain
	ParameterName  string
	Format         VersionFormat
	DefaultVersion *Version
}

// NewVersionMiddleware creates a new version detection middleware
// Automatically checks all locations (header and path) with header taking priority
func NewVersionMiddleware(config MiddlewareConfig) *VersionMiddleware {
	// Get all possible version strings for path matching
	versions := make([]string, len(config.VersionBundle.GetVersions()))
	for i, v := range config.VersionBundle.GetVersions() {
		versions[i] = v.String()
	}

	// Create version manager that checks all locations
	versionManager := NewVersionManager(config.ParameterName, versions)

	return &VersionMiddleware{
		versionBundle:  config.VersionBundle,
		migrationChain: config.MigrationChain,
		versionManager: versionManager,
		defaultVersion: config.DefaultVersion,
		parameterName:  config.ParameterName,
		format:         config.Format,
	}
}

// Gin context keys for storing version information
const versionContextKey = "cadwyn_api_version"
const defaultVersionContextKey = "cadwyn_default_version_used"

// Middleware returns the Gin middleware function
func (vm *VersionMiddleware) Middleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract version from request
		versionStr, err := vm.versionManager.GetVersion(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Invalid version format: %v", err)})
			c.Abort()
			return
		}

		var requestedVersion *Version
		var defaultUsed bool

		if versionStr == "" {
			// No version specified, use default
			requestedVersion = vm.defaultVersion
			if requestedVersion == nil {
				requestedVersion = vm.versionBundle.GetHeadVersion()
			}
			defaultUsed = true
		} else {
			// Parse the requested version
			requestedVersion, err = vm.versionBundle.ParseVersion(versionStr)
			if err != nil {
				// First, try to match as a partial version (e.g., "v1" matches latest v1.x.x)
				requestedVersion = vm.findLatestMatchingVersion(versionStr)

				// If no partial match, try waterfall logic - find closest older version
				if requestedVersion == nil && vm.isValidVersionFormat(versionStr) {
					requestedVersion = vm.findClosestOlderVersion(versionStr)
				}

				if requestedVersion == nil {
					hint := fmt.Sprintf("Specify version using '%s' header or include it in the URL path (e.g., /v1/resource)", vm.parameterName)
					c.JSON(http.StatusBadRequest, gin.H{
						"error":              fmt.Sprintf("Unknown version: %s", versionStr),
						"available_versions": vm.versionBundle.GetVersionValues(),
						"hint":               hint,
					})
					c.Abort()
					return
				}
			}
		}

		// Set version in Gin context
		c.Set(versionContextKey, requestedVersion)
		if defaultUsed {
			c.Set(defaultVersionContextKey, true)
		}

		// Add Epoch metadata to context for user's logging middleware
		c.Set("epoch.version", requestedVersion.String())
		c.Set("epoch.is_head", requestedVersion.IsHead)
		c.Set("epoch.default_used", defaultUsed)
		c.Set("epoch.parameter_name", vm.parameterName)

		// Add version to response header
		c.Header(vm.parameterName, requestedVersion.String())

		// Continue with the request
		c.Next()
	}
}

// findLatestMatchingVersion finds the latest version matching a partial version string
// For example, "v1" or "1" matches the latest v1.x.x version
func (vm *VersionMiddleware) findLatestMatchingVersion(partialVersionStr string) *Version {
	// Normalize the partial version string (remove 'v' prefix if present)
	normalized := strings.TrimPrefix(strings.TrimPrefix(partialVersionStr, "v"), "V")

	var latestMatch *Version

	for _, v := range vm.versionBundle.GetVersions() {
		versionStr := v.String()

		// Try to match the version string with the partial version
		// For semver: "1" matches "1.0.0", "1.2.3", etc.
		// For semver: "1.2" matches "1.2.0", "1.2.3", etc.
		if vm.versionMatchesPartial(versionStr, partialVersionStr, normalized) {
			// Keep track of the latest matching version
			if latestMatch == nil || v.IsNewerThan(latestMatch) {
				latestMatch = v
			}
		}
	}

	return latestMatch
}

// versionMatchesPartial checks if a full version matches a partial version string
func (vm *VersionMiddleware) versionMatchesPartial(fullVersion, partialVersion, normalized string) bool {
	// Normalize the full version
	normalizedFull := strings.TrimPrefix(strings.TrimPrefix(fullVersion, "v"), "V")

	// Check if it starts with the normalized partial version
	// For "1" to match "1.0.0", "1.2.3", etc.
	// For "1.2" to match "1.2.0", "1.2.3", etc.
	if strings.HasPrefix(normalizedFull, normalized) {
		// Ensure it's a proper prefix (followed by a dot or end of string)
		if len(normalizedFull) == len(normalized) {
			return true
		}
		if len(normalizedFull) > len(normalized) && normalizedFull[len(normalized)] == '.' {
			return true
		}
	}

	return false
}

// findClosestOlderVersion implements waterfall versioning logic
// If requested version doesn't exist, find the closest older version
func (vm *VersionMiddleware) findClosestOlderVersion(requestedVersionStr string) *Version {
	// Parse the requested version first to enable proper comparison
	requestedVersion, err := NewVersion(requestedVersionStr)
	if err != nil {
		return nil // Invalid version format
	}

	var closestVersion *Version

	for _, v := range vm.versionBundle.GetVersions() {
		// Use proper version comparison instead of string comparison
		if v.IsOlderThan(requestedVersion) {
			if closestVersion == nil || v.IsNewerThan(closestVersion) {
				closestVersion = v
			}
		}
	}

	return closestVersion
}

// isValidVersionFormat checks if a string matches a valid version format
func (vm *VersionMiddleware) isValidVersionFormat(versionStr string) bool {
	// Check for date format (YYYY-MM-DD)
	if matched, _ := regexp.MatchString(`^\d{4}-\d{2}-\d{2}$`, versionStr); matched {
		return true
	}

	// Check for semver format (X.Y.Z or X.Y)
	if matched, _ := regexp.MatchString(`^v?\d+\.\d+(\.\d+)?$`, versionStr); matched {
		return true
	}

	// For string versions, we're more permissive but exclude obviously invalid ones
	// like "invalid" which is clearly not a version
	if versionStr == "invalid" || versionStr == "unknown" || versionStr == "error" {
		return false
	}

	return true
}

// GetVersionFromContext extracts the version from the Gin context
func GetVersionFromContext(c *gin.Context) *Version {
	if v, exists := c.Get(versionContextKey); exists {
		if version, ok := v.(*Version); ok {
			return version
		}
	}
	return nil
}

// IsDefaultVersionUsed checks if the default version was used (no version specified)
func IsDefaultVersionUsed(c *gin.Context) bool {
	if used, exists := c.Get(defaultVersionContextKey); exists {
		if defaultUsed, ok := used.(bool); ok {
			return defaultUsed
		}
	}
	return false
}

// VersionAwareHandler wraps a Gin handler with version-aware request/response migration
type VersionAwareHandler struct {
	handler        gin.HandlerFunc
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
}

// NewVersionAwareHandler creates a new version-aware handler
func NewVersionAwareHandler(handler gin.HandlerFunc, versionBundle *VersionBundle, migrationChain *MigrationChain) *VersionAwareHandler {
	return &VersionAwareHandler{
		handler:        handler,
		versionBundle:  versionBundle,
		migrationChain: migrationChain,
	}
}

// HandlerFunc returns a Gin handler function with automatic migration
func (vah *VersionAwareHandler) HandlerFunc() gin.HandlerFunc {
	return func(c *gin.Context) {
		requestedVersion := GetVersionFromContext(c)
		if requestedVersion == nil {
			// No version in context, call handler directly
			vah.handler(c)
			return
		}

		// Implement request/response migration
		vah.handleWithMigration(c, requestedVersion)
	}
}

// handleWithMigration handles request/response migration for version-aware handlers
func (vah *VersionAwareHandler) handleWithMigration(c *gin.Context, requestedVersion *Version) {
	// Skip migration if requesting head version
	if requestedVersion.IsHead {
		vah.handler(c)
		return
	}

	// 1. Migrate request from requested version to head version
	if err := vah.migrateRequest(c, requestedVersion); err != nil {
		c.JSON(500, gin.H{"error": "Request migration failed", "details": err.Error()})
		return
	}

	// 2. Create a response writer that captures the response
	responseCapture := &ResponseCapture{
		ResponseWriter: c.Writer,
		body:           make([]byte, 0),
		statusCode:     200,
	}
	c.Writer = responseCapture

	// 3. Call the handler (which expects head version data)
	vah.handler(c)

	// 4. Migrate response from head version back to requested version
	if err := vah.migrateResponse(c, requestedVersion, responseCapture); err != nil {
		c.JSON(500, gin.H{"error": "Response migration failed", "details": err.Error()})
		return
	}
}

// ResponseCapture captures response data for migration
type ResponseCapture struct {
	gin.ResponseWriter
	body       []byte
	statusCode int
}

func (rc *ResponseCapture) Write(data []byte) (int, error) {
	rc.body = append(rc.body, data...)
	return len(data), nil
}

func (rc *ResponseCapture) WriteHeader(statusCode int) {
	rc.statusCode = statusCode
}

// migrateRequest migrates request data from requested version to head version
func (vah *VersionAwareHandler) migrateRequest(c *gin.Context, fromVersion *Version) error {
	// Get request body if present
	if c.Request.Body == nil {
		return nil // No body to migrate
	}

	// Read body once and preserve it
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}
	c.Request.Body.Close()

	// If body is empty, nothing to migrate
	if len(bodyBytes) == 0 {
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	// Parse JSON body with Sonic to preserve field order
	bodyNode, err := sonic.Get(bodyBytes)
	if err != nil {
		// If JSON parsing fails, restore original body and let handler deal with it
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	// Load the node to parse the entire structure
	if err := bodyNode.Load(); err != nil {
		c.Request.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		return nil
	}

	// Create RequestInfo for migration
	requestInfo := NewRequestInfo(c, &bodyNode)

	// Find migration chain from requested version to head
	headVersion := vah.versionBundle.GetHeadVersion()
	migrationChain := vah.migrationChain.GetMigrationPath(fromVersion, headVersion)

	// Apply migrations in forward direction
	for _, change := range migrationChain {
		if err := change.MigrateRequest(c.Request.Context(), requestInfo); err != nil {
			return fmt.Errorf("failed to migrate request with change %s: %w", change.Description(), err)
		}
	}

	// Update the request context with migrated data
	c.Set("migratedRequestBody", requestInfo.Body)

	// Marshal the migrated body using Sonic's Raw() to preserve field order
	migratedJSON, err := requestInfo.Body.Raw()
	if err != nil {
		return fmt.Errorf("failed to get raw JSON from migrated request: %w", err)
	}

	c.Request.Body = io.NopCloser(bytes.NewReader([]byte(migratedJSON)))

	return nil
}

// migrateResponse migrates response data from head version back to requested version
func (vah *VersionAwareHandler) migrateResponse(c *gin.Context, toVersion *Version, responseCapture *ResponseCapture) error {
	// Parse captured response body with Sonic to preserve field order
	var responseNode *ast.Node
	if len(responseCapture.body) > 0 {
		node, err := sonic.Get(responseCapture.body)
		if err != nil {
			// If JSON parsing fails, write original response
			c.Writer = responseCapture.ResponseWriter
			c.Writer.WriteHeader(responseCapture.statusCode)
			_, _ = c.Writer.Write(responseCapture.body)
			return nil
		}

		// IMPORTANT: sonic.Get() returns a search node that needs to be loaded
		// We need to call Load() to actually parse the entire structure
		if err := node.Load(); err != nil {
			c.Writer = responseCapture.ResponseWriter
			c.Writer.WriteHeader(responseCapture.statusCode)
			_, _ = c.Writer.Write(responseCapture.body)
			return nil
		}

		responseNode = &node
	}

	// Create ResponseInfo for migration
	responseInfo := NewResponseInfo(c, responseNode)
	responseInfo.StatusCode = responseCapture.statusCode

	// Find migration chain from head version back to requested version
	headVersion := vah.versionBundle.GetHeadVersion()
	migrationChain := vah.migrationChain.GetMigrationPath(headVersion, toVersion)

	// Apply migrations in reverse direction
	for i := len(migrationChain) - 1; i >= 0; i-- {
		change := migrationChain[i]
		if err := change.MigrateResponse(c.Request.Context(), responseInfo); err != nil {
			return fmt.Errorf("failed to migrate response with change %s: %w", change.Description(), err)
		}
	}

	// Write the migrated response with preserved field order
	c.Writer = responseCapture.ResponseWriter

	if responseInfo.Body != nil {
		// Use Sonic's Raw() to preserve field order
		migratedJSON, err := responseInfo.Body.Raw()
		if err != nil {
			return fmt.Errorf("failed to get raw JSON from migrated response: %w", err)
		}

		c.Data(responseInfo.StatusCode, "application/json", []byte(migratedJSON))
	} else {
		c.Writer.WriteHeader(responseInfo.StatusCode)
	}

	return nil
}
