package cadwyn

import (
	"fmt"
)

// Cadwyn is the main application that manages API versioning
type Cadwyn struct {
	versionBundle   *VersionBundle
	migrationChain  *MigrationChain
	schemaGenerator *SchemaGenerator
	ginApp          *Application
	router          *VersionedRouter
}

// Config holds configuration for the Cadwyn application
type Config struct {
	Versions []*Version
	Changes  []*VersionChange

	// Gin configuration
	EnableGinServer      bool
	VersionLocation      VersionLocation
	VersionParameterName string
	VersionFormat        VersionFormat
	DefaultVersion       *Version
	GinMode              string // "debug", "release", "test"

	// Features
	EnableSchemaGeneration bool
	EnableChangelog        bool
	EnableDebugLogging     bool

	// Server configuration
	Title       string
	Description string
	Version     string
}

// New creates a new Cadwyn application with the given configuration
func New(config Config) (*Cadwyn, error) {
	if len(config.Versions) == 0 {
		return nil, fmt.Errorf("at least one version must be specified")
	}

	// Apply defaults
	applyDefaults(&config)

	// Create version bundle
	versionBundle := NewVersionBundle(config.Versions)

	// Create migration chain
	migrationChain := NewMigrationChain(config.Changes)

	// Create schema generator
	var schemaGenerator *SchemaGenerator
	if config.EnableSchemaGeneration {
		schemaGenerator = NewSchemaGenerator(versionBundle, migrationChain)
	}

	// Create router
	router := NewVersionedRouter(RouterConfig{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Create Gin application if enabled
	var ginApp *Application
	if config.EnableGinServer {
		ginConfig := &ApplicationConfig{
			VersionBundle:          versionBundle,
			MigrationChain:         migrationChain,
			VersionLocation:        config.VersionLocation,
			VersionParameterName:   config.VersionParameterName,
			VersionFormat:          config.VersionFormat,
			DefaultVersion:         config.DefaultVersion,
			Title:                  config.Title,
			Description:            config.Description,
			Version:                config.Version,
			EnableSchemaGeneration: config.EnableSchemaGeneration,
			EnableChangelog:        config.EnableChangelog,
			EnableDebugLogging:     config.EnableDebugLogging,
			GinMode:                config.GinMode,
		}

		var err error
		ginApp, err = NewApplication(ginConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to create Gin application: %w", err)
		}
	}

	return &Cadwyn{
		versionBundle:   versionBundle,
		migrationChain:  migrationChain,
		schemaGenerator: schemaGenerator,
		ginApp:          ginApp,
		router:          router,
	}, nil
}

// applyDefaults applies default configuration values
func applyDefaults(config *Config) {
	if config.VersionLocation == "" {
		config.VersionLocation = VersionLocationHeader
	}

	if config.VersionParameterName == "" {
		config.VersionParameterName = "X-API-Version"
	}

	if config.VersionFormat == "" {
		config.VersionFormat = VersionFormatSemver
	}

	if config.Title == "" {
		config.Title = "Cadwyn API"
	}

	if config.Version == "" {
		config.Version = "1.0.0"
	}

	if config.Description == "" {
		config.Description = "API with automatic versioning"
	}
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

// GetGinApp returns the Gin application
func (c *Cadwyn) GetGinApp() *Application {
	return c.ginApp
}

// GetRouter returns the versioned router
func (c *Cadwyn) GetRouter() *VersionedRouter {
	return c.router
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

// Builder provides a fluent API for building Cadwyn applications
type Builder struct {
	config Config
}

// NewBuilder creates a new Cadwyn builder
func NewBuilder() *Builder {
	return &Builder{
		config: Config{
			Versions:               []*Version{},
			Changes:                []*VersionChange{},
			EnableGinServer:        true,
			EnableSchemaGeneration: true,
			EnableChangelog:        true,
			EnableDebugLogging:     false,
			GinMode:                "release",
		},
	}
}

// WithVersions sets the versions for the application
func (b *Builder) WithVersions(versions ...*Version) *Builder {
	b.config.Versions = versions
	return b
}

// WithDateVersions creates and adds date-based versions
func (b *Builder) WithDateVersions(dates ...string) *Builder {
	for _, dateStr := range dates {
		if v, err := NewDateVersion(dateStr); err == nil {
			b.config.Versions = append(b.config.Versions, v)
		}
	}
	return b
}

// WithSemverVersions creates and adds semantic versions
func (b *Builder) WithSemverVersions(semvers ...string) *Builder {
	for _, semverStr := range semvers {
		if v, err := NewSemverVersion(semverStr); err == nil {
			b.config.Versions = append(b.config.Versions, v)
		}
	}
	return b
}

// WithHeadVersion adds a head version
func (b *Builder) WithHeadVersion() *Builder {
	b.config.Versions = append(b.config.Versions, NewHeadVersion())
	return b
}

// WithVersionChanges sets the version changes for the application
func (b *Builder) WithVersionChanges(changes ...*VersionChange) *Builder {
	b.config.Changes = changes
	return b
}

// WithGinServer enables/disables Gin server functionality
func (b *Builder) WithGinServer(enabled bool) *Builder {
	b.config.EnableGinServer = enabled
	return b
}

// WithGinMode sets the Gin mode (debug, release, test)
func (b *Builder) WithGinMode(mode string) *Builder {
	b.config.GinMode = mode
	return b
}

// WithVersionLocation sets where to look for version information
func (b *Builder) WithVersionLocation(location VersionLocation) *Builder {
	b.config.VersionLocation = location
	return b
}

// WithVersionParameter sets the parameter name for version detection
func (b *Builder) WithVersionParameter(name string) *Builder {
	b.config.VersionParameterName = name
	return b
}

// WithVersionFormat sets the version format
func (b *Builder) WithVersionFormat(format VersionFormat) *Builder {
	b.config.VersionFormat = format
	return b
}

// WithDefaultVersion sets the default version
func (b *Builder) WithDefaultVersion(v *Version) *Builder {
	b.config.DefaultVersion = v
	return b
}

// WithSchemaGeneration enables/disables schema generation
func (b *Builder) WithSchemaGeneration(enabled bool) *Builder {
	b.config.EnableSchemaGeneration = enabled
	return b
}

// WithChangelog enables/disables changelog generation
func (b *Builder) WithChangelog(enabled bool) *Builder {
	b.config.EnableChangelog = enabled
	return b
}

// WithDebugLogging enables/disables debug logging
func (b *Builder) WithDebugLogging(enabled bool) *Builder {
	b.config.EnableDebugLogging = enabled
	return b
}

// WithTitle sets the API title
func (b *Builder) WithTitle(title string) *Builder {
	b.config.Title = title
	return b
}

// WithDescription sets the API description
func (b *Builder) WithDescription(description string) *Builder {
	b.config.Description = description
	return b
}

// Build creates the Cadwyn application
func (b *Builder) Build() (*Cadwyn, error) {
	return New(b.config)
}

// Quick setup functions for common use cases

// NewWithDateVersions creates a Cadwyn app with date-based versions
func NewWithDateVersions(dates ...string) (*Cadwyn, error) {
	builder := NewBuilder().WithDateVersions(dates...)
	return builder.Build()
}

// NewWithSemverVersions creates a Cadwyn app with semantic versions
func NewWithSemverVersions(semvers ...string) (*Cadwyn, error) {
	builder := NewBuilder().WithSemverVersions(semvers...)
	return builder.Build()
}

// NewSimple creates a simple Cadwyn app with just a head version
func NewSimple() (*Cadwyn, error) {
	builder := NewBuilder().WithHeadVersion()
	return builder.Build()
}

// Version creation helpers

// DateVersion creates a date-based version
func DateVersion(date string) *Version {
	v, err := NewDateVersion(date)
	if err != nil {
		panic(fmt.Sprintf("invalid date version '%s': %v", date, err))
	}
	return v
}

// SemverVersion creates a semantic version
func SemverVersion(semver string) *Version {
	v, err := NewSemverVersion(semver)
	if err != nil {
		panic(fmt.Sprintf("invalid semver version '%s': %v", semver, err))
	}
	return v
}

// HeadVersion creates a head version
func HeadVersion() *Version {
	return NewHeadVersion()
}
