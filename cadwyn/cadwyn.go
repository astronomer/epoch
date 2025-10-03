package cadwyn

import (
	"fmt"
	"reflect"

	"github.com/gin-gonic/gin"
)

// Cadwyn provides API versioning capabilities for existing Gin applications
type Cadwyn struct {
	versionBundle   *VersionBundle
	migrationChain  *MigrationChain
	schemaGenerator *SchemaGenerator
	versionConfig   VersionConfig
}

// VersionConfig holds configuration for version detection and handling
type VersionConfig struct {
	VersionLocation      VersionLocation
	VersionParameterName string
	VersionFormat        VersionFormat
	DefaultVersion       *Version
}

// NewCadwyn creates a new Cadwyn instance for API versioning
func NewCadwyn() *CadwynBuilder {
	return &CadwynBuilder{
		versions: []*Version{},
		changes:  []*VersionChange{},
		types:    []reflect.Type{},
		versionConfig: VersionConfig{
			VersionLocation:      VersionLocationHeader,
			VersionParameterName: "X-API-Version",
			VersionFormat:        VersionFormatSemver,
		},
	}
}

// Middleware returns a Gin middleware that detects API versions from requests
func (c *Cadwyn) Middleware() gin.HandlerFunc {
	middleware := NewVersionMiddleware(MiddlewareConfig{
		VersionBundle:  c.versionBundle,
		MigrationChain: c.migrationChain,
		Location:       c.versionConfig.VersionLocation,
		ParameterName:  c.versionConfig.VersionParameterName,
		Format:         c.versionConfig.VersionFormat,
		DefaultVersion: c.versionConfig.DefaultVersion,
	})
	return middleware.Middleware()
}

// WrapHandler wraps a Gin handler to provide automatic request/response migration
func (c *Cadwyn) WrapHandler(handler gin.HandlerFunc) gin.HandlerFunc {
	versionAwareHandler := NewVersionAwareHandler(handler, c.versionBundle, c.migrationChain)
	return versionAwareHandler.HandlerFunc()
}

// GetVersionBundle returns the version bundle
func (c *Cadwyn) GetVersionBundle() *VersionBundle {
	return c.versionBundle
}

// GetMigrationChain returns the migration chain
func (c *Cadwyn) GetMigrationChain() *MigrationChain {
	return c.migrationChain
}

// GetSchemaGenerator returns the schema generator
func (c *Cadwyn) GetSchemaGenerator() *SchemaGenerator {
	return c.schemaGenerator
}

// GetVersions returns all configured versions
func (c *Cadwyn) GetVersions() []*Version {
	return c.versionBundle.GetVersions()
}

// GetHeadVersion returns the head (latest) version
func (c *Cadwyn) GetHeadVersion() *Version {
	return c.versionBundle.GetHeadVersion()
}

// ParseVersion parses a version string
func (c *Cadwyn) ParseVersion(versionStr string) (*Version, error) {
	return c.versionBundle.ParseVersion(versionStr)
}

// GenerateStructForVersion generates Go code for a struct at a specific version
func (c *Cadwyn) GenerateStructForVersion(structType interface{}, targetVersion string) (string, error) {
	if c.schemaGenerator == nil {
		return "", fmt.Errorf("schema generation is not enabled")
	}
	reflectType := reflect.TypeOf(structType)
	if reflectType.Kind() == reflect.Ptr {
		reflectType = reflectType.Elem()
	}
	return c.schemaGenerator.GenerateStruct(reflectType, targetVersion)
}

// CadwynBuilder provides a fluent API for building Cadwyn instances
type CadwynBuilder struct {
	versions      []*Version
	changes       []*VersionChange
	types         []reflect.Type
	versionConfig VersionConfig
}

// WithVersions sets the versions for the application
func (cb *CadwynBuilder) WithVersions(versions ...*Version) *CadwynBuilder {
	cb.versions = append(cb.versions, versions...)
	return cb
}

// WithDateVersions creates and adds date-based versions
func (cb *CadwynBuilder) WithDateVersions(dates ...string) *CadwynBuilder {
	for _, dateStr := range dates {
		if v, err := NewDateVersion(dateStr); err == nil {
			cb.versions = append(cb.versions, v)
		}
	}
	return cb
}

// WithSemverVersions creates and adds semantic versions (supports both major.minor.patch and major.minor)
func (cb *CadwynBuilder) WithSemverVersions(semvers ...string) *CadwynBuilder {
	for _, semverStr := range semvers {
		if v, err := NewSemverVersion(semverStr); err == nil {
			cb.versions = append(cb.versions, v)
		}
	}
	return cb
}

// WithStringVersions creates and adds string-based versions
func (cb *CadwynBuilder) WithStringVersions(versions ...string) *CadwynBuilder {
	for _, versionStr := range versions {
		v := NewStringVersion(versionStr)
		cb.versions = append(cb.versions, v)
	}
	return cb
}

// WithHeadVersion adds a head version
func (cb *CadwynBuilder) WithHeadVersion() *CadwynBuilder {
	cb.versions = append(cb.versions, NewHeadVersion())
	return cb
}

// WithChanges sets the version changes for the application
func (cb *CadwynBuilder) WithChanges(changes ...*VersionChange) *CadwynBuilder {
	cb.changes = append(cb.changes, changes...)
	return cb
}

// WithVersionLocation sets where to look for version information
func (cb *CadwynBuilder) WithVersionLocation(location VersionLocation) *CadwynBuilder {
	cb.versionConfig.VersionLocation = location
	return cb
}

// WithVersionParameter sets the parameter name for version detection
func (cb *CadwynBuilder) WithVersionParameter(name string) *CadwynBuilder {
	cb.versionConfig.VersionParameterName = name
	return cb
}

// WithVersionFormat sets the version format
func (cb *CadwynBuilder) WithVersionFormat(format VersionFormat) *CadwynBuilder {
	cb.versionConfig.VersionFormat = format
	return cb
}

// WithDefaultVersion sets the default version
func (cb *CadwynBuilder) WithDefaultVersion(v *Version) *CadwynBuilder {
	cb.versionConfig.DefaultVersion = v
	return cb
}

// WithTypes registers multiple types for schema generation
func (cb *CadwynBuilder) WithTypes(types ...interface{}) *CadwynBuilder {
	for _, t := range types {
		reflectType := reflect.TypeOf(t)
		if reflectType.Kind() == reflect.Ptr {
			reflectType = reflectType.Elem()
		}
		cb.types = append(cb.types, reflectType)
	}
	return cb
}

// Build creates the Cadwyn instance
func (cb *CadwynBuilder) Build() (*Cadwyn, error) {
	if len(cb.versions) == 0 {
		return nil, fmt.Errorf("at least one version must be specified")
	}

	// Create version bundle
	versionBundle, err := NewVersionBundle(cb.versions)
	if err != nil {
		return nil, fmt.Errorf("failed to create version bundle: %w", err)
	}

	// Create migration chain
	migrationChain := NewMigrationChain(cb.changes)

	// Create schema generator
	schemaGenerator := NewSchemaGenerator(versionBundle, migrationChain)

	// Register types with schema generator
	registeredTypes := make([]string, 0)
	for _, t := range cb.types {
		if err := schemaGenerator.RegisterType(t); err != nil {
			return nil, fmt.Errorf("failed to register type '%s': %w (already registered types: %v)",
				t.Name(), err, registeredTypes)
		}
		registeredTypes = append(registeredTypes, t.Name())
	}

	return &Cadwyn{
		versionBundle:   versionBundle,
		migrationChain:  migrationChain,
		schemaGenerator: schemaGenerator,
		versionConfig:   cb.versionConfig,
	}, nil
}

// Convenience functions for common setups

// QuickStart creates a Cadwyn instance with date versions and head version
func QuickStart(dates ...string) (*Cadwyn, error) {
	builder := NewCadwyn().WithDateVersions(dates...).WithHeadVersion()
	return builder.Build()
}

// WithSemver creates a Cadwyn instance with semantic versions and head version
func WithSemver(semvers ...string) (*Cadwyn, error) {
	builder := NewCadwyn().WithSemverVersions(semvers...).WithHeadVersion()
	return builder.Build()
}

// WithStringVersions creates a Cadwyn instance with string versions and head version
func WithStrings(versions ...string) (*Cadwyn, error) {
	builder := NewCadwyn().WithStringVersions(versions...).WithHeadVersion()
	return builder.Build()
}

// Simple creates a Cadwyn instance with just a head version
func Simple() (*Cadwyn, error) {
	builder := NewCadwyn().WithHeadVersion()
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
