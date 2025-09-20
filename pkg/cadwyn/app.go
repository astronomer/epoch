package cadwyn

import (
	"fmt"

	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

// Cadwyn is the main application that manages API versioning
type Cadwyn struct {
	versionBundle  *version.VersionBundle
	migrationChain *migration.MigrationChain
}

// Config holds configuration for the Cadwyn application
type Config struct {
	Versions []*version.Version
	Changes  []*migration.VersionChange
}

// New creates a new Cadwyn application with the given configuration
func New(config Config) (*Cadwyn, error) {
	if len(config.Versions) == 0 {
		return nil, fmt.Errorf("at least one version must be specified")
	}

	// Create version bundle
	versionBundle := version.NewVersionBundle(config.Versions)

	// Create migration chain
	migrationChain := migration.NewMigrationChain(config.Changes)

	return &Cadwyn{
		versionBundle:  versionBundle,
		migrationChain: migrationChain,
	}, nil
}

// GetVersionBundle returns the version bundle
func (c *Cadwyn) GetVersionBundle() *version.VersionBundle {
	return c.versionBundle
}

// GetMigrationChain returns the migration chain
func (c *Cadwyn) GetMigrationChain() *migration.MigrationChain {
	return c.migrationChain
}

// GetVersions returns all configured versions
func (c *Cadwyn) GetVersions() []*version.Version {
	return c.versionBundle.GetVersions()
}

// GetHeadVersion returns the head (latest) version
func (c *Cadwyn) GetHeadVersion() *version.Version {
	return c.versionBundle.GetHeadVersion()
}

// ParseVersion parses a version string
func (c *Cadwyn) ParseVersion(versionStr string) (*version.Version, error) {
	return c.versionBundle.ParseVersion(versionStr)
}

// Builder provides a fluent API for building Cadwyn applications
type Builder struct {
	versions []*version.Version
	changes  []*migration.VersionChange
}

// NewBuilder creates a new Cadwyn builder
func NewBuilder() *Builder {
	return &Builder{
		versions: []*version.Version{},
		changes:  []*migration.VersionChange{},
	}
}

// WithVersions sets the versions for the application
func (b *Builder) WithVersions(versions ...*version.Version) *Builder {
	b.versions = versions
	return b
}

// WithDateVersions creates and adds date-based versions
func (b *Builder) WithDateVersions(dates ...string) *Builder {
	for _, dateStr := range dates {
		if v, err := version.NewDateVersion(dateStr); err == nil {
			b.versions = append(b.versions, v)
		}
	}
	return b
}

// WithSemverVersions creates and adds semantic versions
func (b *Builder) WithSemverVersions(semvers ...string) *Builder {
	for _, semverStr := range semvers {
		if v, err := version.NewSemverVersion(semverStr); err == nil {
			b.versions = append(b.versions, v)
		}
	}
	return b
}

// WithHeadVersion adds a head version
func (b *Builder) WithHeadVersion() *Builder {
	b.versions = append(b.versions, version.NewHeadVersion())
	return b
}

// WithVersionChanges sets the version changes for the application
func (b *Builder) WithVersionChanges(changes ...*migration.VersionChange) *Builder {
	b.changes = changes
	return b
}

// Build creates the Cadwyn application
func (b *Builder) Build() (*Cadwyn, error) {
	return New(Config{
		Versions: b.versions,
		Changes:  b.changes,
	})
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
