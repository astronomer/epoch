package cadwyn

import (
	"context"
	"fmt"
	"reflect"
)

// VersionChange defines a set of instructions for migrating between two API versions
// This matches the Python Cadwyn VersionChange class structure
type VersionChange struct {
	description                            string
	isHiddenFromChangelog                  bool
	instructionsToMigrateToPreviousVersion []interface{}

	// Organized instruction containers (like Python Cadwyn)
	alterSchemaInstructions           []*SchemaInstruction
	alterEnumInstructions             []*EnumInstruction
	alterEndpointInstructions         []*EndpointInstruction
	alterRequestBySchemaInstructions  map[reflect.Type][]*AlterRequestInstruction
	alterRequestByPathInstructions    map[string][]*AlterRequestInstruction
	alterResponseBySchemaInstructions map[reflect.Type][]*AlterResponseInstruction
	alterResponseByPathInstructions   map[string][]*AlterResponseInstruction

	// Version information
	fromVersion *Version
	toVersion   *Version

	// Route mappings (like Python's _route_to_*_mapping)
	routeToRequestMigrationMapping  map[int][]*AlterRequestInstruction
	routeToResponseMigrationMapping map[int][]*AlterResponseInstruction
}

// NewVersionChange creates a new version change with the given description and instructions
func NewVersionChange(description string, fromVersion, toVersion *Version, instructions ...interface{}) *VersionChange {
	vc := &VersionChange{
		description:                            description,
		fromVersion:                            fromVersion,
		toVersion:                              toVersion,
		instructionsToMigrateToPreviousVersion: instructions,
		alterRequestBySchemaInstructions:       make(map[reflect.Type][]*AlterRequestInstruction),
		alterRequestByPathInstructions:         make(map[string][]*AlterRequestInstruction),
		alterResponseBySchemaInstructions:      make(map[reflect.Type][]*AlterResponseInstruction),
		alterResponseByPathInstructions:        make(map[string][]*AlterResponseInstruction),
		routeToRequestMigrationMapping:         make(map[int][]*AlterRequestInstruction),
		routeToResponseMigrationMapping:        make(map[int][]*AlterResponseInstruction),
	}

	vc.extractInstructionsIntoContainers()
	return vc
}

// extractInstructionsIntoContainers organizes instructions by type (like Python Cadwyn)
func (vc *VersionChange) extractInstructionsIntoContainers() {
	for _, instruction := range vc.instructionsToMigrateToPreviousVersion {
		switch inst := instruction.(type) {
		case *SchemaInstruction:
			vc.alterSchemaInstructions = append(vc.alterSchemaInstructions, inst)
		case *EnumInstruction:
			vc.alterEnumInstructions = append(vc.alterEnumInstructions, inst)
		case *EndpointInstruction:
			vc.alterEndpointInstructions = append(vc.alterEndpointInstructions, inst)
		case *AlterRequestInstruction:
			if inst.Path != "" {
				vc.alterRequestByPathInstructions[inst.Path] = append(
					vc.alterRequestByPathInstructions[inst.Path], inst)
			} else {
				for _, schema := range inst.Schemas {
					schemaType := reflect.TypeOf(schema)
					vc.alterRequestBySchemaInstructions[schemaType] = append(
						vc.alterRequestBySchemaInstructions[schemaType], inst)
				}
			}
		case *AlterResponseInstruction:
			if inst.Path != "" {
				vc.alterResponseByPathInstructions[inst.Path] = append(
					vc.alterResponseByPathInstructions[inst.Path], inst)
			} else {
				for _, schema := range inst.Schemas {
					schemaType := reflect.TypeOf(schema)
					vc.alterResponseBySchemaInstructions[schemaType] = append(
						vc.alterResponseBySchemaInstructions[schemaType], inst)
				}
			}
		}
	}
}

// MigrateRequest applies request migrations for this version change
func (vc *VersionChange) MigrateRequest(ctx context.Context, requestInfo *RequestInfo, bodyType reflect.Type, routeID int) error {
	// Apply schema-based request migrations
	if bodyType != nil {
		if instructions, exists := vc.alterRequestBySchemaInstructions[bodyType]; exists {
			for _, instruction := range instructions {
				if err := instruction.Transformer(requestInfo); err != nil {
					return fmt.Errorf("request schema migration failed: %w", err)
				}
			}
		}
	}

	// Apply path-based request migrations
	if instructions, exists := vc.routeToRequestMigrationMapping[routeID]; exists {
		for _, instruction := range instructions {
			if err := instruction.Transformer(requestInfo); err != nil {
				return fmt.Errorf("request path migration failed: %w", err)
			}
		}
	}

	return nil
}

// MigrateResponse applies response migrations for this version change
func (vc *VersionChange) MigrateResponse(ctx context.Context, responseInfo *ResponseInfo, responseType reflect.Type, routeID int) error {
	// Apply schema-based response migrations
	if responseType != nil {
		if instructions, exists := vc.alterResponseBySchemaInstructions[responseType]; exists {
			for _, instruction := range instructions {
				// Check if we should migrate error responses
				if responseInfo.StatusCode >= 300 && !instruction.MigrateHTTPErrors {
					continue
				}
				if err := instruction.Transformer(responseInfo); err != nil {
					return fmt.Errorf("response schema migration failed: %w", err)
				}
			}
		}
	}

	// Apply path-based response migrations
	if instructions, exists := vc.routeToResponseMigrationMapping[routeID]; exists {
		for _, instruction := range instructions {
			// Check if we should migrate error responses
			if responseInfo.StatusCode >= 300 && !instruction.MigrateHTTPErrors {
				continue
			}
			if err := instruction.Transformer(responseInfo); err != nil {
				return fmt.Errorf("response path migration failed: %w", err)
			}
		}
	}

	return nil
}

// FromVersion returns the version this change migrates from
func (vc *VersionChange) FromVersion() *Version {
	return vc.fromVersion
}

// ToVersion returns the version this change migrates to
func (vc *VersionChange) ToVersion() *Version {
	return vc.toVersion
}

// Description returns a human-readable description of this change
func (vc *VersionChange) Description() string {
	return vc.description
}

// IsHiddenFromChangelog returns whether this change should be hidden from changelogs
func (vc *VersionChange) IsHiddenFromChangelog() bool {
	return vc.isHiddenFromChangelog
}

// SetHiddenFromChangelog sets whether this change should be hidden from changelogs
func (vc *VersionChange) SetHiddenFromChangelog(hidden bool) {
	vc.isHiddenFromChangelog = hidden
}

// BindRouteToRequestMigrations binds path-based request migrations to a specific route ID
func (vc *VersionChange) BindRouteToRequestMigrations(routeID int, path string) {
	if instructions, exists := vc.alterRequestByPathInstructions[path]; exists {
		vc.routeToRequestMigrationMapping[routeID] = instructions
	}
}

// BindRouteToResponseMigrations binds path-based response migrations to a specific route ID
func (vc *VersionChange) BindRouteToResponseMigrations(routeID int, path string) {
	if instructions, exists := vc.alterResponseByPathInstructions[path]; exists {
		vc.routeToResponseMigrationMapping[routeID] = instructions
	}
}

// GetSchemaInstructions returns all schema instructions
func (vc *VersionChange) GetSchemaInstructions() []*SchemaInstruction {
	return vc.alterSchemaInstructions
}

// GetEndpointInstructions returns all endpoint instructions
func (vc *VersionChange) GetEndpointInstructions() []*EndpointInstruction {
	return vc.alterEndpointInstructions
}

// GetEnumInstructions returns all enum instructions
func (vc *VersionChange) GetEnumInstructions() []*EnumInstruction {
	return vc.alterEnumInstructions
}

// FieldChange represents a change to a specific field (kept for backward compatibility)
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

// MigrationChain manages a sequence of version changes
type MigrationChain struct {
	changes []*VersionChange
}

// NewMigrationChain creates a new migration chain
func NewMigrationChain(changes []*VersionChange) *MigrationChain {
	return &MigrationChain{
		changes: changes,
	}
}

// MigrateRequest applies all changes in the chain for request migration
func (mc *MigrationChain) MigrateRequest(ctx context.Context, requestInfo *RequestInfo, from, to *Version, bodyType reflect.Type, routeID int) error {
	// Find the starting point in the version chain
	start := -1
	for i, change := range mc.changes {
		if change.FromVersion().Equal(from) || change.FromVersion().IsOlderThan(from) {
			start = i
			break
		}
	}

	if start == -1 {
		return fmt.Errorf("no migration path found from version %s (available changes: %d)",
			from.String(), len(mc.changes))
	}

	// Apply changes in sequence until we reach the target version
	for i := start; i < len(mc.changes); i++ {
		change := mc.changes[i]

		// Stop if we've reached the target version
		if change.ToVersion().Equal(to) {
			if err := change.MigrateRequest(ctx, requestInfo, bodyType, routeID); err != nil {
				return fmt.Errorf("migration failed at %s->%s: %w",
					change.FromVersion().String(), change.ToVersion().String(), err)
			}
			break
		}

		// Stop if this change would take us past the target
		if change.ToVersion().IsNewerThan(to) {
			break
		}

		// Apply this change if it's part of the migration path
		if change.FromVersion().IsOlderThan(to) || change.FromVersion().Equal(to) {
			if err := change.MigrateRequest(ctx, requestInfo, bodyType, routeID); err != nil {
				return fmt.Errorf("migration failed at %s->%s: %w",
					change.FromVersion().String(), change.ToVersion().String(), err)
			}
		}
	}

	return nil
}

// MigrateResponse applies all changes in reverse for response migration
func (mc *MigrationChain) MigrateResponse(ctx context.Context, responseInfo *ResponseInfo, from, to *Version, responseType reflect.Type, routeID int) error {
	// Find the ending point in the version chain
	end := -1
	for i, change := range mc.changes {
		if change.ToVersion().Equal(from) || change.ToVersion().IsNewerThan(from) {
			end = i
			break
		}
	}

	if end == -1 {
		return fmt.Errorf("no migration path found from version %s (available changes: %d)",
			from.String(), len(mc.changes))
	}

	// Apply changes in reverse until we reach the target version
	for i := end; i >= 0; i-- {
		change := mc.changes[i]

		// Stop if we've reached the target version
		if change.FromVersion().Equal(to) {
			if err := change.MigrateResponse(ctx, responseInfo, responseType, routeID); err != nil {
				return fmt.Errorf("reverse migration failed at %s->%s: %w",
					change.ToVersion().String(), change.FromVersion().String(), err)
			}
			break
		}

		// Stop if this change would take us past the target (going backwards)
		if change.FromVersion().IsOlderThan(to) {
			break
		}

		// Apply this change if it's part of the migration path
		if change.ToVersion().IsNewerThan(to) || change.ToVersion().Equal(to) {
			if err := change.MigrateResponse(ctx, responseInfo, responseType, routeID); err != nil {
				return fmt.Errorf("reverse migration failed at %s->%s: %w",
					change.ToVersion().String(), change.FromVersion().String(), err)
			}
		}
	}

	return nil
}

// AddChange adds a new version change to the chain
func (mc *MigrationChain) AddChange(change *VersionChange) {
	mc.changes = append(mc.changes, change)
}

// GetChanges returns all changes in the chain
func (mc *MigrationChain) GetChanges() []*VersionChange {
	return mc.changes
}

// GetMigrationPath returns the changes needed to migrate from one version to another
func (mc *MigrationChain) GetMigrationPath(from, to *Version) []*VersionChange {
	if from.Equal(to) {
		return []*VersionChange{}
	}

	var path []*VersionChange

	// If migrating forward (from older to newer)
	if from.IsOlderThan(to) {
		for _, change := range mc.changes {
			// Include changes that are in the path from 'from' to 'to'
			if (change.FromVersion().Equal(from) || change.FromVersion().IsNewerThan(from)) &&
				(change.ToVersion().Equal(to) || change.ToVersion().IsOlderThan(to)) {
				path = append(path, change)
			}
		}
	} else {
		// If migrating backward (from newer to older)
		// We need to reverse the changes that got us from 'to' to 'from'
		for _, change := range mc.changes {
			// Include changes that are in the path from 'to' to 'from'
			if (change.FromVersion().Equal(to) || change.FromVersion().IsNewerThan(to)) &&
				(change.ToVersion().Equal(from) || change.ToVersion().IsOlderThan(from)) {
				path = append(path, change)
			}
		}
	}

	return path
}
