package cadwyn

import (
	"fmt"
	"net/http"
	"time"

	"github.com/isaacchung/cadwyn-go/pkg/middleware"
	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/router"
	"github.com/isaacchung/cadwyn-go/pkg/schema"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

// Cadwyn is the main application orchestrator that ties all components together
type Cadwyn struct {
	config            *Config
	versionBundle     *version.VersionBundle
	migrationChain    *migration.MigrationChain
	versionedRouter   *router.VersionedRouter
	versionMiddleware *middleware.VersionMiddleware
	schemaAnalyzer    *schema.SchemaAnalyzer
}

// Config holds configuration for the Cadwyn application
type Config struct {
	// Version configuration
	Versions       []*version.Version
	VersionChanges []migration.VersionChange

	// Version detection configuration
	VersionLocation      middleware.VersionLocation // "header", "query", "path"
	VersionParameterName string                     // e.g., "x-api-version", "version", "v"
	DefaultVersion       *version.Version

	// Application configuration
	EnableSchemaAnalysis  bool
	EnableDebugLogging    bool
	EnableRouteGeneration bool
}

// New creates a new Cadwyn application with the given configuration
func New(config Config) (*Cadwyn, error) {
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Apply defaults
	applyDefaults(&config)

	// Create version bundle
	versionBundle := version.NewVersionBundle(config.Versions)

	// Create migration chain
	migrationChain := migration.NewMigrationChain(config.VersionChanges)

	// Create versioned router
	versionedRouter := router.NewVersionedRouter(router.Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Create version middleware
	versionMiddleware := middleware.NewVersionMiddleware(middleware.Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
		ParameterName:  config.VersionParameterName,
		Location:       config.VersionLocation,
		DefaultVersion: config.DefaultVersion,
	})

	// Create schema analyzer (optional)
	var schemaAnalyzer *schema.SchemaAnalyzer
	if config.EnableSchemaAnalysis {
		schemaAnalyzer = schema.NewSchemaAnalyzer()
	}

	app := &Cadwyn{
		config:            &config,
		versionBundle:     versionBundle,
		migrationChain:    migrationChain,
		versionedRouter:   versionedRouter,
		versionMiddleware: versionMiddleware,
		schemaAnalyzer:    schemaAnalyzer,
	}

	// Generate versioned routes if enabled
	if config.EnableRouteGeneration {
		if err := app.generateVersionedRoutes(); err != nil {
			return nil, fmt.Errorf("failed to generate versioned routes: %w", err)
		}
	}

	return app, nil
}

// validateConfig validates the configuration
func validateConfig(config *Config) error {
	if len(config.Versions) == 0 {
		return fmt.Errorf("at least one version must be specified")
	}

	return nil
}

// applyDefaults applies default values to the configuration
func applyDefaults(config *Config) {
	if config.VersionLocation == "" {
		config.VersionLocation = middleware.VersionLocationHeader
	}

	if config.VersionParameterName == "" {
		switch config.VersionLocation {
		case middleware.VersionLocationHeader:
			config.VersionParameterName = "x-api-version"
		case middleware.VersionLocationQuery:
			config.VersionParameterName = "version"
		case middleware.VersionLocationPath:
			config.VersionParameterName = "v"
		}
	}

	if config.DefaultVersion == nil && len(config.Versions) > 0 {
		// Use the latest version as default
		config.DefaultVersion = config.Versions[len(config.Versions)-1]
	}

	// Enable route generation by default
	if !config.EnableRouteGeneration {
		config.EnableRouteGeneration = true
	}
}

// Router returns the versioned router for registering routes
func (c *Cadwyn) Router() *router.VersionedRouter {
	return c.versionedRouter
}

// Middleware returns the HTTP middleware that should be applied to your HTTP server
func (c *Cadwyn) Middleware() func(http.Handler) http.Handler {
	return c.versionMiddleware.Middleware()
}

// ServeHTTP implements http.Handler, allowing Cadwyn to be used directly as an HTTP handler
func (c *Cadwyn) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Apply version middleware and then route
	handler := c.Middleware()(c.versionedRouter)
	handler.ServeHTTP(w, r)
}

// GetVersions returns all configured versions
func (c *Cadwyn) GetVersions() []*version.Version {
	return c.versionBundle.GetVersions()
}

// GetHeadVersion returns the head (latest) version
func (c *Cadwyn) GetHeadVersion() *version.Version {
	return c.versionBundle.GetHeadVersion()
}

// GetVersionChanges returns all configured version changes
func (c *Cadwyn) GetVersionChanges() []migration.VersionChange {
	return c.migrationChain.GetChanges()
}

// AddVersionChange adds a new version change to the migration chain
func (c *Cadwyn) AddVersionChange(change migration.VersionChange) {
	c.migrationChain.AddChange(change)
}

// generateVersionedRoutes generates versioned routes based on registered routes
func (c *Cadwyn) generateVersionedRoutes() error {
	return c.versionedRouter.GenerateVersionedRoutes()
}

// GetRouteInfo returns information about all registered routes
func (c *Cadwyn) GetRouteInfo() []router.RouteInfo {
	return c.versionedRouter.GetRouteInfo()
}

// PrintRoutes prints all registered routes (useful for debugging)
func (c *Cadwyn) PrintRoutes() {
	if c.config.EnableDebugLogging {
		c.versionedRouter.PrintRoutes()
	}
}

// AnalyzeSchema analyzes a struct schema (if schema analysis is enabled)
func (c *Cadwyn) AnalyzeSchema(v interface{}) (*schema.Schema, error) {
	if c.schemaAnalyzer == nil {
		return nil, fmt.Errorf("schema analysis is not enabled")
	}
	return c.schemaAnalyzer.AnalyzeStruct(v)
}

// GenerateMigrationChanges generates migration changes between two structs (if schema analysis is enabled)
func (c *Cadwyn) GenerateMigrationChanges(oldStruct, newStruct interface{}) ([]migration.FieldChange, error) {
	if c.schemaAnalyzer == nil {
		return nil, fmt.Errorf("schema analysis is not enabled")
	}

	transformer := schema.NewStructTransformer()
	return transformer.GenerateMigrationChanges(oldStruct, newStruct)
}

// Builder provides a fluent API for building Cadwyn applications
type Builder struct {
	config Config
}

// NewBuilder creates a new Cadwyn builder
func NewBuilder() *Builder {
	return &Builder{
		config: Config{
			Versions:              []*version.Version{},
			VersionChanges:        []migration.VersionChange{},
			EnableSchemaAnalysis:  false,
			EnableDebugLogging:    false,
			EnableRouteGeneration: true,
		},
	}
}

// WithVersions sets the versions for the application
func (b *Builder) WithVersions(versions ...*version.Version) *Builder {
	b.config.Versions = versions
	return b
}

// WithDateVersions creates and adds date-based versions
func (b *Builder) WithDateVersions(dates ...string) *Builder {
	for _, dateStr := range dates {
		if v, err := version.NewDateVersion(dateStr); err == nil {
			b.config.Versions = append(b.config.Versions, v)
		}
	}
	return b
}

// WithSemverVersions creates and adds semantic versions
func (b *Builder) WithSemverVersions(semvers ...string) *Builder {
	for _, semverStr := range semvers {
		if v, err := version.NewSemverVersion(semverStr); err == nil {
			b.config.Versions = append(b.config.Versions, v)
		}
	}
	return b
}

// WithHeadVersion adds a head version
func (b *Builder) WithHeadVersion() *Builder {
	b.config.Versions = append(b.config.Versions, version.NewHeadVersion())
	return b
}

// WithVersionChanges sets the version changes for the application
func (b *Builder) WithVersionChanges(changes ...migration.VersionChange) *Builder {
	b.config.VersionChanges = changes
	return b
}

// WithVersionLocation sets where to look for version information
func (b *Builder) WithVersionLocation(location middleware.VersionLocation) *Builder {
	b.config.VersionLocation = location
	return b
}

// WithVersionParameter sets the parameter name for version detection
func (b *Builder) WithVersionParameter(name string) *Builder {
	b.config.VersionParameterName = name
	return b
}

// WithDefaultVersion sets the default version
func (b *Builder) WithDefaultVersion(v *version.Version) *Builder {
	b.config.DefaultVersion = v
	return b
}

// WithSchemaAnalysis enables schema analysis
func (b *Builder) WithSchemaAnalysis() *Builder {
	b.config.EnableSchemaAnalysis = true
	return b
}

// WithDebugLogging enables debug logging
func (b *Builder) WithDebugLogging() *Builder {
	b.config.EnableDebugLogging = true
	return b
}

// WithoutRouteGeneration disables automatic route generation
func (b *Builder) WithoutRouteGeneration() *Builder {
	b.config.EnableRouteGeneration = false
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

// Utility functions

// MustNew creates a Cadwyn app and panics on error (useful for examples)
func MustNew(config Config) *Cadwyn {
	app, err := New(config)
	if err != nil {
		panic(fmt.Sprintf("failed to create Cadwyn app: %v", err))
	}
	return app
}

// MustNewWithDateVersions creates a Cadwyn app with date versions and panics on error
func MustNewWithDateVersions(dates ...string) *Cadwyn {
	app, err := NewWithDateVersions(dates...)
	if err != nil {
		panic(fmt.Sprintf("failed to create Cadwyn app: %v", err))
	}
	return app
}

// Version creation helpers

// DateVersion creates a date-based version
func DateVersion(date string) *version.Version {
	v, err := version.NewDateVersion(date)
	if err != nil {
		panic(fmt.Sprintf("invalid date version '%s': %v", date, err))
	}
	return v
}

// SemverVersion creates a semantic version
func SemverVersion(semver string) *version.Version {
	v, err := version.NewSemverVersion(semver)
	if err != nil {
		panic(fmt.Sprintf("invalid semver version '%s': %v", semver, err))
	}
	return v
}

// HeadVersion creates a head version
func HeadVersion() *version.Version {
	return version.NewHeadVersion()
}

// Today creates a version for today's date
func Today() *version.Version {
	today := time.Now().Format("2006-01-02")
	return DateVersion(today)
}
