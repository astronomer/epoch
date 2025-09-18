package migration

import (
	"context"
	"fmt"

	"github.com/isaacchung/cadwyn-go/pkg/version"
)

// VersionChange defines how to migrate data between two specific versions
type VersionChange interface {
	// Description returns a human-readable description of this change
	Description() string

	// FromVersion returns the source version for this change
	FromVersion() *version.Version

	// ToVersion returns the target version for this change
	ToVersion() *version.Version

	// MigrateRequest migrates a request from FromVersion to ToVersion
	MigrateRequest(ctx context.Context, data interface{}) (interface{}, error)

	// MigrateResponse migrates a response from ToVersion to FromVersion
	MigrateResponse(ctx context.Context, data interface{}) (interface{}, error)

	// AppliesTo returns true if this change can migrate between the given versions
	AppliesTo(from, to *version.Version) bool
}

// BaseVersionChange provides a basic implementation of VersionChange
type BaseVersionChange struct {
	description string
	fromVersion *version.Version
	toVersion   *version.Version
}

// NewBaseVersionChange creates a new base version change
func NewBaseVersionChange(description string, from, to *version.Version) *BaseVersionChange {
	return &BaseVersionChange{
		description: description,
		fromVersion: from,
		toVersion:   to,
	}
}

// Description returns the description of this change
func (bvc *BaseVersionChange) Description() string {
	return bvc.description
}

// FromVersion returns the source version
func (bvc *BaseVersionChange) FromVersion() *version.Version {
	return bvc.fromVersion
}

// ToVersion returns the target version
func (bvc *BaseVersionChange) ToVersion() *version.Version {
	return bvc.toVersion
}

// AppliesTo checks if this change applies to the given version migration
func (bvc *BaseVersionChange) AppliesTo(from, to *version.Version) bool {
	return from.Equal(bvc.fromVersion) && to.Equal(bvc.toVersion)
}

// MigrateRequest provides a default implementation that returns data unchanged
func (bvc *BaseVersionChange) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	return data, nil
}

// MigrateResponse provides a default implementation that returns data unchanged
func (bvc *BaseVersionChange) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	return data, nil
}

// FieldChange represents a change to a specific field
type FieldChange struct {
	// Field name to modify
	FieldName string
	// Operation type: "add", "remove", "rename", "transform"
	Operation string
	// New field name (for rename operations)
	NewFieldName string
	// Default value (for add operations)
	DefaultValue interface{}
	// Transform function (for transform operations)
	TransformFunc func(interface{}) interface{}
}

// StructVersionChange handles struct field transformations
type StructVersionChange struct {
	*BaseVersionChange
	// Changes to apply during migration
	RequestChanges  []FieldChange
	ResponseChanges []FieldChange
}

// NewStructVersionChange creates a new struct-based version change
func NewStructVersionChange(
	description string,
	from, to *version.Version,
	requestChanges, responseChanges []FieldChange,
) *StructVersionChange {
	return &StructVersionChange{
		BaseVersionChange: NewBaseVersionChange(description, from, to),
		RequestChanges:    requestChanges,
		ResponseChanges:   responseChanges,
	}
}

// MigrateRequest applies field changes to transform request data
func (svc *StructVersionChange) MigrateRequest(ctx context.Context, data interface{}) (interface{}, error) {
	return ApplyFieldChanges(data, svc.RequestChanges)
}

// MigrateResponse applies field changes to transform response data
func (svc *StructVersionChange) MigrateResponse(ctx context.Context, data interface{}) (interface{}, error) {
	// For response migration, we typically apply changes in reverse
	return ApplyFieldChanges(data, svc.ResponseChanges)
}

// MigrationChain manages a sequence of version changes
type MigrationChain struct {
	changes []VersionChange
}

// NewMigrationChain creates a new migration chain
func NewMigrationChain(changes []VersionChange) *MigrationChain {
	return &MigrationChain{
		changes: changes,
	}
}

// MigrateRequest applies all changes in the chain for request migration
func (mc *MigrationChain) MigrateRequest(ctx context.Context, data interface{}, from, to *version.Version) (interface{}, error) {
	current := data
	currentVersion := from

	for _, change := range mc.changes {
		if change.AppliesTo(currentVersion, to) ||
			(currentVersion.IsOlderThan(change.ToVersion()) && change.ToVersion().IsOlderThan(to)) ||
			(currentVersion.Equal(change.FromVersion()) && change.ToVersion().IsOlderThan(to)) {

			migrated, err := change.MigrateRequest(ctx, current)
			if err != nil {
				return nil, fmt.Errorf("migration failed at %s->%s: %w",
					change.FromVersion().String(), change.ToVersion().String(), err)
			}
			current = migrated
			currentVersion = change.ToVersion()

			// If we've reached the target version, stop
			if currentVersion.Equal(to) {
				break
			}
		}
	}

	return current, nil
}

// MigrateResponse applies all changes in reverse for response migration
func (mc *MigrationChain) MigrateResponse(ctx context.Context, data interface{}, from, to *version.Version) (interface{}, error) {
	current := data
	currentVersion := from

	// If we're going from head to an older version, apply changes in reverse
	if from.IsNewerThan(to) {
		for i := len(mc.changes) - 1; i >= 0; i-- {
			change := mc.changes[i]

			// Check if this change is relevant for the migration path
			if (change.ToVersion().Equal(currentVersion) || change.ToVersion().IsOlderThan(currentVersion)) &&
				(change.FromVersion().Equal(to) || change.FromVersion().IsNewerThan(to)) {

				migrated, err := change.MigrateResponse(ctx, current)
				if err != nil {
					return nil, fmt.Errorf("reverse migration failed at %s->%s: %w",
						change.ToVersion().String(), change.FromVersion().String(), err)
				}
				current = migrated
				currentVersion = change.FromVersion()

				// If we've reached the target version, stop
				if currentVersion.Equal(to) {
					break
				}
			}
		}
	}

	return current, nil
}

// FindChangesForMigration finds all changes needed to migrate from one version to another
func (mc *MigrationChain) FindChangesForMigration(from, to *version.Version) []VersionChange {
	var applicableChanges []VersionChange

	for _, change := range mc.changes {
		// For forward migration (from < to)
		if from.IsOlderThan(to) {
			if (change.FromVersion().Equal(from) || change.FromVersion().IsNewerThan(from)) &&
				(change.ToVersion().Equal(to) || change.ToVersion().IsOlderThan(to)) {
				applicableChanges = append(applicableChanges, change)
			}
		} else if to.IsOlderThan(from) {
			// For backward migration (from > to)
			if (change.ToVersion().Equal(from) || change.ToVersion().IsOlderThan(from)) &&
				(change.FromVersion().Equal(to) || change.FromVersion().IsNewerThan(to)) {
				applicableChanges = append(applicableChanges, change)
			}
		}
	}

	return applicableChanges
}

// AddChange adds a new version change to the chain
func (mc *MigrationChain) AddChange(change VersionChange) {
	mc.changes = append(mc.changes, change)
}

// GetChanges returns all changes in the chain
func (mc *MigrationChain) GetChanges() []VersionChange {
	return mc.changes
}
