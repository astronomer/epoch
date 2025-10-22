package epoch

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
)

// Epoch provides API versioning capabilities for existing Gin applications
type Epoch struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
	versionConfig  VersionConfig
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

// WrapHandler wraps a Gin handler to provide automatic request/response migration
func (c *Epoch) WrapHandler(handler gin.HandlerFunc) gin.HandlerFunc {
	versionAwareHandler := NewVersionAwareHandler(handler, c.versionBundle, c.migrationChain)
	return versionAwareHandler.HandlerFunc()
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

// GenerateStructForVersion generates Go code for a struct at a specific version
// Note: Schema generation has been removed and will be rebuilt in the future
// using the new declarative operation framework. This function is kept for
// backward compatibility but will return an error.
func (c *Epoch) GenerateStructForVersion(structType interface{}, targetVersion string) (string, error) {
	return "", fmt.Errorf("schema generation has been temporarily removed and will be rebuilt using the new declarative API")
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

	// Create version bundle
	versionBundle, err := NewVersionBundle(cb.versions)
	if err != nil {
		return nil, fmt.Errorf("failed to create version bundle: %w", err)
	}

	// Create migration chain with cycle detection
	migrationChain, err := NewMigrationChain(cb.changes)
	if err != nil {
		return nil, fmt.Errorf("failed to create migration chain: %w", err)
	}

	return &Epoch{
		versionBundle:  versionBundle,
		migrationChain: migrationChain,
		versionConfig:  cb.versionConfig,
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
