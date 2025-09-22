package cadwyn

import (
	"fmt"
	"net/http"
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

		// TODO: Implement request migration
		// This would involve:
		// 1. Reading the request body
		// 2. Migrating it from requested version to head version
		// 3. Calling the handler with migrated request
		// 4. Migrating the response from head version back to requested version

		// For now, just call the handler
		vah.handler(c)
	}
}

// WrapHandler wraps any Gin handler with version-aware migration
func (vm *VersionMiddleware) WrapHandler(handler gin.HandlerFunc) gin.HandlerFunc {
	versionAwareHandler := NewVersionAwareHandler(handler, vm.versionBundle, vm.migrationChain)
	return versionAwareHandler.HandlerFunc()
}
