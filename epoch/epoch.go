package epoch

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
)

// Epoch provides API versioning capabilities for existing Gin applications
type Epoch struct {
	versionBundle    *VersionBundle
	migrationChain   *MigrationChain
	versionConfig    VersionConfig
	endpointRegistry *EndpointRegistry
}

// VersionConfig holds configuration for version detection and handling
type VersionConfig struct {
	VersionParameterName string
	VersionFormat        VersionFormat
	DefaultVersion       *Version
}

// NewEpoch creates a new Epoch instance for API versioning
func NewEpoch() *EpochBuilder {
	return &EpochBuilder{
		versions: []*Version{},
		changes:  []*VersionChange{},
		types:    []reflect.Type{},
		versionConfig: VersionConfig{
			VersionParameterName: "X-API-Version",
			VersionFormat:        VersionFormatSemver,
		},
	}
}

// Middleware returns a Gin middleware that detects API versions from requests
func (c *Epoch) Middleware() gin.HandlerFunc {
	middleware := NewVersionMiddleware(MiddlewareConfig{
		VersionBundle:  c.versionBundle,
		MigrationChain: c.migrationChain,
		ParameterName:  c.versionConfig.VersionParameterName,
		Format:         c.versionConfig.VersionFormat,
		DefaultVersion: c.versionConfig.DefaultVersion,
	})
	return middleware.Middleware()
}

// VersionBundle returns the version bundle (for OpenAPI schema generation)
func (c *Epoch) VersionBundle() *VersionBundle {
	return c.versionBundle
}

// EndpointRegistry returns the endpoint registry (for OpenAPI schema generation)
func (c *Epoch) EndpointRegistry() *EndpointRegistry {
	return c.endpointRegistry
}

// HandlerWrapper wraps a handler and collects type information for endpoint registration
type HandlerWrapper struct {
	epoch        *Epoch
	handler      gin.HandlerFunc
	request      interface{}
	response     interface{}
	nestedArrays map[string]interface{}
}

// WrapHandler wraps a Gin handler to provide automatic request/response migration
// Returns a HandlerWrapper that allows type registration via builder pattern
func (c *Epoch) WrapHandler(handler gin.HandlerFunc) *HandlerWrapper {
	return &HandlerWrapper{
		epoch:        c,
		handler:      handler,
		nestedArrays: make(map[string]interface{}),
	}
}

// Accepts registers the request type for this endpoint
func (hw *HandlerWrapper) Accepts(reqType interface{}) *HandlerWrapper {
	hw.request = reqType
	return hw
}

// Returns registers the response type for this endpoint
func (hw *HandlerWrapper) Returns(respType interface{}) *HandlerWrapper {
	hw.response = respType
	return hw
}

// WithArrayItems registers a nested array field and its item type
func (hw *HandlerWrapper) WithArrayItems(fieldName string, itemType interface{}) *HandlerWrapper {
	hw.nestedArrays[fieldName] = itemType
	return hw
}

// buildEndpointDefinition creates an EndpointDefinition from the wrapper's state
func (hw *HandlerWrapper) buildEndpointDefinition(method, pathPattern string) *EndpointDefinition {
	def := &EndpointDefinition{
		Method:      method,
		PathPattern: pathPattern,
	}

	if hw.request != nil {
		def.RequestType = reflect.TypeOf(hw.request)
		if def.RequestType.Kind() == reflect.Ptr {
			def.RequestType = def.RequestType.Elem()
		}
	}

	if hw.response != nil {
		def.ResponseType = reflect.TypeOf(hw.response)
		if def.ResponseType.Kind() == reflect.Ptr {
			def.ResponseType = def.ResponseType.Elem()
		}
	}

	if len(hw.nestedArrays) > 0 {
		def.NestedArrays = make(map[string]reflect.Type)
		for field, itemType := range hw.nestedArrays {
			itemReflectType := reflect.TypeOf(itemType)
			if itemReflectType.Kind() == reflect.Ptr {
				itemReflectType = itemReflectType.Elem()
			}
			def.NestedArrays[field] = itemReflectType
		}
	}

	return def
}

// ToHandlerFunc converts the wrapper into a gin.HandlerFunc
// Registers types immediately for build-time schema generation
// Example: r.POST("/users", epochInstance.WrapHandler(createUser).Returns(UserResponse{}).ToHandlerFunc("POST", "/users"))
func (hw *HandlerWrapper) ToHandlerFunc(method, pathPattern string) gin.HandlerFunc {
	// Build and register endpoint definition immediately
	def := hw.buildEndpointDefinition(method, pathPattern)
	hw.epoch.endpointRegistry.Register(method, pathPattern, def)

	// Return handler that uses version-aware processing
	return func(c *gin.Context) {
		versionAwareHandler := NewVersionAwareHandler(
			hw.handler,
			hw.epoch.versionBundle,
			hw.epoch.migrationChain,
			hw.epoch.endpointRegistry,
		)
		versionAwareHandler.HandlerFunc()(c)
	}
}

// GetVersionBundle returns the version bundle
func (c *Epoch) GetVersionBundle() *VersionBundle {
	return c.versionBundle
}

// GetMigrationChain returns the migration chain
func (c *Epoch) GetMigrationChain() *MigrationChain {
	return c.migrationChain
}

// GetVersions returns all configured versions
func (c *Epoch) GetVersions() []*Version {
	return c.versionBundle.GetVersions()
}

// GetHeadVersion returns the head (latest) version
func (c *Epoch) GetHeadVersion() *Version {
	return c.versionBundle.GetHeadVersion()
}

// ParseVersion parses a version string
func (c *Epoch) ParseVersion(versionStr string) (*Version, error) {
	return c.versionBundle.ParseVersion(versionStr)
}

// EpochBuilder provides a fluent API for building Epoch instances
type EpochBuilder struct {
	versions      []*Version
	changes       []*VersionChange
	types         []reflect.Type
	versionConfig VersionConfig
	errors        []error // Accumulated errors during building
}

// WithVersions sets the versions for the application
func (cb *EpochBuilder) WithVersions(versions ...*Version) *EpochBuilder {
	cb.versions = append(cb.versions, versions...)
	return cb
}

// WithDateVersions creates and adds date-based versions
// Invalid date strings are collected and will cause Build() to fail with a detailed error.
func (cb *EpochBuilder) WithDateVersions(dates ...string) *EpochBuilder {
	for _, dateStr := range dates {
		v, err := NewDateVersion(dateStr)
		if err != nil {
			cb.errors = append(cb.errors, fmt.Errorf("invalid date version '%s': %w", dateStr, err))
			continue
		}
		cb.versions = append(cb.versions, v)
	}
	return cb
}

// WithSemverVersions creates and adds semantic versions (supports both major.minor.patch and major.minor)
// Invalid semver strings are collected and will cause Build() to fail with a detailed error.
func (cb *EpochBuilder) WithSemverVersions(semvers ...string) *EpochBuilder {
	for _, semverStr := range semvers {
		v, err := NewSemverVersion(semverStr)
		if err != nil {
			cb.errors = append(cb.errors, fmt.Errorf("invalid semver version '%s': %w", semverStr, err))
			continue
		}
		cb.versions = append(cb.versions, v)
	}
	return cb
}

// WithStringVersions creates and adds string-based versions
func (cb *EpochBuilder) WithStringVersions(versions ...string) *EpochBuilder {
	for _, versionStr := range versions {
		v := NewStringVersion(versionStr)
		cb.versions = append(cb.versions, v)
	}
	return cb
}

// WithHeadVersion adds a head version
func (cb *EpochBuilder) WithHeadVersion() *EpochBuilder {
	cb.versions = append(cb.versions, NewHeadVersion())
	return cb
}

// WithChanges sets the version changes for the application
func (cb *EpochBuilder) WithChanges(changes ...*VersionChange) *EpochBuilder {
	cb.changes = append(cb.changes, changes...)
	return cb
}

// WithVersionParameter sets the parameter name for version detection
func (cb *EpochBuilder) WithVersionParameter(name string) *EpochBuilder {
	cb.versionConfig.VersionParameterName = name
	return cb
}

// WithVersionFormat sets the version format
func (cb *EpochBuilder) WithVersionFormat(format VersionFormat) *EpochBuilder {
	cb.versionConfig.VersionFormat = format
	return cb
}

// WithDefaultVersion sets the default version
func (cb *EpochBuilder) WithDefaultVersion(v *Version) *EpochBuilder {
	cb.versionConfig.DefaultVersion = v
	return cb
}

// WithTypes registers multiple types for schema generation
func (cb *EpochBuilder) WithTypes(types ...interface{}) *EpochBuilder {
	for _, t := range types {
		reflectType := reflect.TypeOf(t)
		if reflectType.Kind() == reflect.Ptr {
			reflectType = reflectType.Elem()
		}
		cb.types = append(cb.types, reflectType)
	}
	return cb
}

// Build creates the Epoch instance
func (cb *EpochBuilder) Build() (*Epoch, error) {
	// Check for accumulated errors from builder methods
	if len(cb.errors) > 0 {
		// Return first error with context about all errors
		if len(cb.errors) == 1 {
			return nil, fmt.Errorf("builder validation failed: %w", cb.errors[0])
		}
		errMsg := fmt.Sprintf("builder validation failed with %d errors:", len(cb.errors))
		for i, err := range cb.errors {
			errMsg += fmt.Sprintf("\n  %d. %v", i+1, err)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	if len(cb.versions) == 0 {
		return nil, fmt.Errorf("at least one version must be specified")
	}

	// Create version bundle (without changes associated yet to avoid validation errors)
	versionBundle, err := NewVersionBundle(cb.versions)
	if err != nil {
		return nil, fmt.Errorf("failed to create version bundle: %w", err)
	}

	// Create migration chain with cycle detection
	migrationChain, err := NewMigrationChain(cb.changes)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration chain: %w", err)
	}

	// Associate changes with their from-versions AFTER validation and cycle detection
	// This is needed for schema generation to find applicable changes
	for _, change := range cb.changes {
		// Find the version that this change migrates from
		for _, version := range cb.versions {
			if version.Equal(change.FromVersion()) {
				version.Changes = append(version.Changes, change)
				break
			}
		}
	}

	return &Epoch{
		versionBundle:    versionBundle,
		migrationChain:   migrationChain,
		versionConfig:    cb.versionConfig,
		endpointRegistry: NewEndpointRegistry(),
	}, nil
}

// Convenience functions for common setups

// QuickStart creates an Epoch instance with date versions and head version
func QuickStart(dates ...string) (*Epoch, error) {
	builder := NewEpoch().WithDateVersions(dates...).WithHeadVersion()
	return builder.Build()
}

// WithSemver creates an Epoch instance with semantic versions and head version
func WithSemver(semvers ...string) (*Epoch, error) {
	builder := NewEpoch().WithSemverVersions(semvers...).WithHeadVersion()
	return builder.Build()
}

// WithStringVersions creates an Epoch instance with string versions and head version
func WithStrings(versions ...string) (*Epoch, error) {
	builder := NewEpoch().WithStringVersions(versions...).WithHeadVersion()
	return builder.Build()
}

// Simple creates an Epoch instance with just a head version
func Simple() (*Epoch, error) {
	builder := NewEpoch().WithHeadVersion()
	return builder.Build()
}

// StringVersion creates a string version (convenience wrapper)
func StringVersion(version string) *Version {
	return NewStringVersion(version)
}

// HeadVersion creates a head version
func HeadVersion() *Version {
	return NewHeadVersion()
}
