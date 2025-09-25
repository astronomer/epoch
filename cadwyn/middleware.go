package cadwyn

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

// VersionLocation defines where to look for version information
type VersionLocation string

const (
	VersionLocationHeader VersionLocation = "header"
	VersionLocationQuery  VersionLocation = "query"
	VersionLocationPath   VersionLocation = "path"
)

// VersionFormat defines the format of version values
type VersionFormat string

const (
	VersionFormatDate   VersionFormat = "date"
	VersionFormatSemver VersionFormat = "semver"
	VersionFormatString VersionFormat = "string"
)

// VersionManager handles version extraction from Gin contexts
type VersionManager interface {
	GetVersion(c *gin.Context) (string, error)
}

// HeaderVersionManager extracts version from HTTP headers
type HeaderVersionManager struct {
	headerName string
}

// NewHeaderVersionManager creates a new header-based version manager
func NewHeaderVersionManager(headerName string) *HeaderVersionManager {
	return &HeaderVersionManager{
		headerName: headerName,
	}
}

func (hvm *HeaderVersionManager) GetVersion(c *gin.Context) (string, error) {
	version := c.GetHeader(hvm.headerName)
	if version == "" {
		return "", nil // No version specified
	}
	return version, nil
}

// QueryVersionManager extracts version from query parameters
type QueryVersionManager struct {
	paramName string
}

// NewQueryVersionManager creates a new query-based version manager
func NewQueryVersionManager(paramName string) *QueryVersionManager {
	return &QueryVersionManager{
		paramName: paramName,
	}
}

func (qvm *QueryVersionManager) GetVersion(c *gin.Context) (string, error) {
	version := c.Query(qvm.paramName)
	return version, nil
}

// PathVersionManager extracts version from URL path
type PathVersionManager struct {
	versionRegex     *regexp.Regexp
	possibleVersions map[string]bool
}

// NewPathVersionManager creates a new path-based version manager
func NewPathVersionManager(possibleVersions []string) *PathVersionManager {
	versionMap := make(map[string]bool)
	for _, v := range possibleVersions {
		versionMap[v] = true
	}

	// Create regex to match any of the possible versions
	escapedVersions := make([]string, len(possibleVersions))
	for i, v := range possibleVersions {
		escapedVersions[i] = regexp.QuoteMeta(v)
	}

	pattern := fmt.Sprintf("/(%s)/", strings.Join(escapedVersions, "|"))
	regex := regexp.MustCompile(pattern)

	return &PathVersionManager{
		versionRegex:     regex,
		possibleVersions: versionMap,
	}
}

func (pvm *PathVersionManager) GetVersion(c *gin.Context) (string, error) {
	matches := pvm.versionRegex.FindStringSubmatch(c.Request.URL.Path)
	if len(matches) > 1 {
		return matches[1], nil
	}
	return "", nil
}

// VersionMiddleware handles version detection and context setting
type VersionMiddleware struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
	versionManager VersionManager
	defaultVersion *Version
	parameterName  string
	location       VersionLocation
	format         VersionFormat
}

// MiddlewareConfig holds configuration for version middleware
type MiddlewareConfig struct {
	VersionBundle  *VersionBundle
	MigrationChain *MigrationChain
	ParameterName  string
	Location       VersionLocation
	Format         VersionFormat
	DefaultVersion *Version
}

// NewVersionMiddleware creates a new version detection middleware
func NewVersionMiddleware(config MiddlewareConfig) *VersionMiddleware {
	var versionManager VersionManager

	switch config.Location {
	case VersionLocationHeader:
		versionManager = NewHeaderVersionManager(config.ParameterName)
	case VersionLocationQuery:
		versionManager = NewQueryVersionManager(config.ParameterName)
	case VersionLocationPath:
		versions := make([]string, len(config.VersionBundle.GetVersions()))
		for i, v := range config.VersionBundle.GetVersions() {
			versions[i] = v.String()
		}
		versionManager = NewPathVersionManager(versions)
	default:
		versionManager = NewHeaderVersionManager("X-API-Version")
	}

	return &VersionMiddleware{
		versionBundle:  config.VersionBundle,
		migrationChain: config.MigrationChain,
		versionManager: versionManager,
		defaultVersion: config.DefaultVersion,
		parameterName:  config.ParameterName,
		location:       config.Location,
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
				// Try waterfall logic - find closest older version
				requestedVersion = vm.findClosestOlderVersion(versionStr)
				if requestedVersion == nil {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("Unknown version: %s", versionStr)})
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

		// Add version to response header
		c.Header(vm.parameterName, requestedVersion.String())

		// Continue with the request
		c.Next()
	}
}

// findClosestOlderVersion implements waterfall versioning logic
// If requested version doesn't exist, find the closest older version
func (vm *VersionMiddleware) findClosestOlderVersion(requestedVersion string) *Version {
	var closestVersion *Version

	for _, v := range vm.versionBundle.GetVersions() {
		// For date-based versions, we can do string comparison
		// For semantic versions, we'd need more sophisticated comparison
		if v.String() <= requestedVersion {
			if closestVersion == nil || v.String() > closestVersion.String() {
				closestVersion = v
			}
		}
	}

	return closestVersion
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

// WrapHandler wraps any Gin handler with version-aware migration
func (vm *VersionMiddleware) WrapHandler(handler gin.HandlerFunc) gin.HandlerFunc {
	versionAwareHandler := NewVersionAwareHandler(handler, vm.versionBundle, vm.migrationChain)
	return versionAwareHandler.HandlerFunc()
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

	// Read the request body
	bodyBytes, err := c.GetRawData()
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	if len(bodyBytes) == 0 {
		return nil // No body to migrate
	}

	// Parse JSON body
	var bodyData interface{}
	if err := c.ShouldBindJSON(&bodyData); err != nil {
		// If JSON parsing fails, leave body as-is
		return nil
	}

	// Create RequestInfo for migration
	requestInfo := NewRequestInfo(c, bodyData)

	// Find migration chain from requested version to head
	headVersion := vah.versionBundle.GetHeadVersion()
	migrationChain := vah.migrationChain.GetMigrationPath(fromVersion, headVersion)

	// Apply migrations in forward direction
	for _, change := range migrationChain {
		if err := change.MigrateRequest(c.Request.Context(), requestInfo, reflect.TypeOf(requestInfo.Body), 0); err != nil {
			return fmt.Errorf("failed to migrate request with change %s: %w", change.Description(), err)
		}
	}

	// Update the request context with migrated data
	c.Set("migratedRequestBody", requestInfo.Body)

	return nil
}

// migrateResponse migrates response data from head version back to requested version
func (vah *VersionAwareHandler) migrateResponse(c *gin.Context, toVersion *Version, responseCapture *ResponseCapture) error {
	// Parse captured response body
	var responseData interface{}
	if len(responseCapture.body) > 0 {
		if err := c.ShouldBindJSON(&responseData); err != nil {
			// If JSON parsing fails, write original response
			c.Writer = responseCapture.ResponseWriter
			c.Writer.WriteHeader(responseCapture.statusCode)
			c.Writer.Write(responseCapture.body)
			return nil
		}
	}

	// Create ResponseInfo for migration
	responseInfo := NewResponseInfo(c, responseData)
	responseInfo.StatusCode = responseCapture.statusCode

	// Find migration chain from head version back to requested version
	headVersion := vah.versionBundle.GetHeadVersion()
	migrationChain := vah.migrationChain.GetMigrationPath(headVersion, toVersion)

	// Apply migrations in reverse direction
	for i := len(migrationChain) - 1; i >= 0; i-- {
		change := migrationChain[i]
		if err := change.MigrateResponse(c.Request.Context(), responseInfo, reflect.TypeOf(responseInfo.Body), 0); err != nil {
			return fmt.Errorf("failed to migrate response with change %s: %w", change.Description(), err)
		}
	}

	// Write the migrated response
	c.Writer = responseCapture.ResponseWriter
	c.Writer.WriteHeader(responseInfo.StatusCode)

	if responseInfo.Body != nil {
		c.JSON(responseInfo.StatusCode, responseInfo.Body)
	}

	return nil
}
