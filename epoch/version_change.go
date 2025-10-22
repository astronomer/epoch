package epoch

import (
	"context"
	"fmt"
	"strings"
)

// AlterRequestInstruction defines how to modify a request during migration
type AlterRequestInstruction struct {
	Schemas     []interface{} // Types this instruction applies to
	Path        string        // Path this instruction applies to (if path-based)
	Methods     []string      // HTTP methods this applies to (if path-based)
	Transformer func(*RequestInfo) error
}

// AlterResponseInstruction defines how to modify a response during migration
type AlterResponseInstruction struct {
	Schemas           []interface{} // Types this instruction applies to
	Path              string        // Path this instruction applies to (if path-based)
	Methods           []string      // HTTP methods this applies to (if path-based)
	MigrateHTTPErrors bool          // Whether to migrate error responses
	Transformer       func(*ResponseInfo) error
}

// VersionChange defines a set of instructions for migrating between two API versions
type VersionChange struct {
	description                            string
	isHiddenFromChangelog                  bool
	instructionsToMigrateToPreviousVersion []interface{}

	// Path-based instruction containers (primary routing mechanism)
	alterRequestByPathInstructions  map[string][]*AlterRequestInstruction
	alterResponseByPathInstructions map[string][]*AlterResponseInstruction

	// Version information
	fromVersion *Version
	toVersion   *Version
}

// NewVersionChange creates a new version change with the given description and instructions
func NewVersionChange(description string, fromVersion, toVersion *Version, instructions ...interface{}) *VersionChange {
	vc := &VersionChange{
		description:                            description,
		fromVersion:                            fromVersion,
		toVersion:                              toVersion,
		instructionsToMigrateToPreviousVersion: instructions,
		alterRequestByPathInstructions:         make(map[string][]*AlterRequestInstruction),
		alterResponseByPathInstructions:        make(map[string][]*AlterResponseInstruction),
	}

	vc.extractInstructionsIntoContainers()
	return vc
}

// extractInstructionsIntoContainers organizes instructions by type
func (vc *VersionChange) extractInstructionsIntoContainers() {
	for _, instruction := range vc.instructionsToMigrateToPreviousVersion {
		switch inst := instruction.(type) {
		case *AlterRequestInstruction:
			if inst.Path != "" {
				vc.alterRequestByPathInstructions[inst.Path] = append(
					vc.alterRequestByPathInstructions[inst.Path], inst)
			}

		case *AlterResponseInstruction:
			if inst.Path != "" {
				vc.alterResponseByPathInstructions[inst.Path] = append(
					vc.alterResponseByPathInstructions[inst.Path], inst)
			}
		}
	}
}

// MigrateRequest applies request migrations for this version change
// Note: ctx parameter is currently unused but reserved for future use (timeouts, cancellation, tracing)
func (vc *VersionChange) MigrateRequest(ctx context.Context, requestInfo *RequestInfo) error {
	// Extract path and method from request context
	var requestPath, requestMethod string
	if requestInfo.GinContext != nil && requestInfo.GinContext.Request != nil {
		requestPath = requestInfo.GinContext.Request.URL.Path
		requestMethod = requestInfo.GinContext.Request.Method
	}

	// Apply path-based instructions that match this request
	for _, instructions := range vc.alterRequestByPathInstructions {
		for _, instruction := range instructions {
			// Check if this instruction applies to this path
			if instruction.Path != "" && matchesPath(requestPath, instruction.Path) {
				// Check if method matches (empty Methods means all methods)
				if len(instruction.Methods) == 0 || contains(instruction.Methods, requestMethod) {
					if err := instruction.Transformer(requestInfo); err != nil {
						return fmt.Errorf("request path migration failed for change '%s': %w", vc.description, err)
					}
				}
			}
		}
	}

	return nil
}

// MigrateResponse applies response migrations for this version change
// Note: ctx parameter is currently unused but reserved for future use (timeouts, cancellation, tracing)
func (vc *VersionChange) MigrateResponse(ctx context.Context, responseInfo *ResponseInfo) error {
	// Extract path and method from response context
	var responsePath, responseMethod string
	if responseInfo.GinContext != nil && responseInfo.GinContext.Request != nil {
		responsePath = responseInfo.GinContext.Request.URL.Path
		responseMethod = responseInfo.GinContext.Request.Method
	}

	// Apply path-based instructions that match this response
	for _, instructions := range vc.alterResponseByPathInstructions {
		for _, instruction := range instructions {
			// Check if this instruction applies to this path
			if instruction.Path != "" && matchesPath(responsePath, instruction.Path) {
				// Check if method matches (empty Methods means all methods)
				if len(instruction.Methods) == 0 || contains(instruction.Methods, responseMethod) {
					// Check if we should migrate error responses
					if responseInfo.StatusCode >= 400 && !instruction.MigrateHTTPErrors {
						continue
					}
					if err := instruction.Transformer(responseInfo); err != nil {
						return fmt.Errorf("response path migration failed for change '%s': %w", vc.description, err)
					}
				}
			}
		}
	}

	return nil
}

// matchesPath checks if a request path matches a pattern
// Supports exact matches and Gin-style path parameters (e.g., /users/:id)
func matchesPath(requestPath, pattern string) bool {
	// Exact match
	if requestPath == pattern {
		return true
	}

	// Gin-style parameter matching (e.g., /users/:id matches /users/123)
	patternParts := strings.Split(strings.Trim(pattern, "/"), "/")
	requestParts := strings.Split(strings.Trim(requestPath, "/"), "/")

	if len(patternParts) != len(requestParts) {
		return false
	}

	for i := range patternParts {
		// Check for parameter (starts with :)
		if strings.HasPrefix(patternParts[i], ":") {
			continue // Parameters match any value
		}

		// Must be exact match
		if patternParts[i] != requestParts[i] {
			return false
		}
	}

	return true
}

// contains checks if a string slice contains a specific value
func contains(slice []string, value string) bool {
	for _, item := range slice {
		if item == value {
			return true
		}
	}
	return false
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

// MigrationChain manages a sequence of version changes
type MigrationChain struct {
	changes []*VersionChange
}

// NewMigrationChain creates a new migration chain with cycle detection
func NewMigrationChain(changes []*VersionChange) (*MigrationChain, error) {
	mc := &MigrationChain{
		changes: changes,
	}

	// Detect cycles in the version graph
	if err := mc.detectCycles(); err != nil {
		return nil, err
	}

	return mc, nil
}

// MigrateRequest applies all changes in the chain for request migration
func (mc *MigrationChain) MigrateRequest(ctx context.Context, requestInfo *RequestInfo, from, to *Version) error {
	// Find the starting point in the version chain
	start := -1
	for i, change := range mc.changes {
		if change.FromVersion().Equal(from) || change.FromVersion().IsOlderThan(from) {
			start = i
			break
		}
	}

	if start == -1 {
		return fmt.Errorf("no migration path found from version %s to %s (available changes: %d)",
			from.String(), to.String(), len(mc.changes))
	}

	// Apply changes in sequence until we reach the target version
	for i := start; i < len(mc.changes); i++ {
		change := mc.changes[i]

		// Stop if we've reached the target version
		if change.ToVersion().Equal(to) {
			if err := change.MigrateRequest(ctx, requestInfo); err != nil {
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
			if err := change.MigrateRequest(ctx, requestInfo); err != nil {
				return fmt.Errorf("migration failed at %s->%s: %w",
					change.FromVersion().String(), change.ToVersion().String(), err)
			}
		}
	}

	return nil
}

// MigrateResponse applies all changes in reverse for response migration
func (mc *MigrationChain) MigrateResponse(ctx context.Context, responseInfo *ResponseInfo, from, to *Version) error {
	// If from and to are the same, no migration needed
	if from.Equal(to) {
		return nil
	}

	// First, validate that 'from' version exists in the migration chain
	foundFromVersion := false
	for _, change := range mc.changes {
		if change.ToVersion().Equal(from) || change.FromVersion().Equal(from) {
			foundFromVersion = true
			break
		}
	}
	if !foundFromVersion {
		return fmt.Errorf("no migration path found from version %s (version not in migration chain)",
			from.String())
	}

	// Collect ALL changes that need to be applied for this migration
	// When migrating from 'from' (e.g. HEAD/v3) to 'to' (e.g. v2):
	// - Apply ALL changes where FromVersion==to (e.g. all v2->v3 changes)
	// - These are applied in reverse (as v3->v2) to step back one version
	var changesToApply []*VersionChange

	for _, change := range mc.changes {
		// Apply change if FromVersion matches target AND ToVersion is in our path
		// This ensures we apply ALL migrations at the target version level
		if change.FromVersion().Equal(to) &&
			(change.ToVersion().Equal(from) || change.ToVersion().IsNewerThan(to)) {
			changesToApply = append(changesToApply, change)
		}
	}

	// If no changes found, return error
	if len(changesToApply) == 0 {
		return fmt.Errorf("no migration path found from version %s to %s (no applicable changes found)",
			from.String(), to.String())
	}

	// Apply all collected changes
	for _, change := range changesToApply {
		if err := change.MigrateResponse(ctx, responseInfo); err != nil {
			return fmt.Errorf("reverse migration failed at %s->%s: %w",
				change.ToVersion().String(), change.FromVersion().String(), err)
		}
	}

	return nil
}

// AddChange adds a new version change to the chain
func (mc *MigrationChain) AddChange(change *VersionChange) {
	mc.changes = append(mc.changes, change)
}

// detectCycles uses depth-first search to find cycles in the version graph
func (mc *MigrationChain) detectCycles() error {
	// Build adjacency list: version string -> list of target versions
	graph := make(map[string][]string)
	versionSet := make(map[string]bool)

	for _, change := range mc.changes {
		from := change.FromVersion().String()
		to := change.ToVersion().String()

		graph[from] = append(graph[from], to)
		versionSet[from] = true
		versionSet[to] = true
	}

	// Track visit states: 0=unvisited, 1=visiting, 2=visited
	visited := make(map[string]int)

	// Check each version for cycles
	for version := range versionSet {
		if visited[version] == 0 {
			if err := dfs(version, graph, visited, []string{}); err != nil {
				return err
			}
		}
	}

	return nil
}

// dfs performs depth-first search to detect cycles
func dfs(node string, graph map[string][]string, visited map[string]int, path []string) error {
	// Mark as visiting
	visited[node] = 1
	path = append(path, node)

	// Visit all neighbors
	for _, neighbor := range graph[node] {
		switch visited[neighbor] {
		case 0:
			// Unvisited, continue DFS
			if err := dfs(neighbor, graph, visited, path); err != nil {
				return err
			}
		case 1:
			// Currently visiting - cycle detected!
			cycleStart := -1
			for i, v := range path {
				if v == neighbor {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := append(path[cycleStart:], neighbor)
				return fmt.Errorf("cycle detected in version chain: %s", strings.Join(cycle, " -> "))
			}
			return fmt.Errorf("cycle detected in version chain involving: %s", neighbor)
		case 2:
			// Already visited, no cycle
			continue
		}
	}

	// Mark as fully visited
	visited[node] = 2
	return nil
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
			// A change is included if:
			// 1. Its FromVersion is >= from (Equal or IsNewerThan)
			// 2. Its ToVersion is <= to (IsOlderThan or Equal)
			if (change.FromVersion().Equal(from) || change.FromVersion().IsNewerThan(from)) &&
				(change.ToVersion().IsOlderThan(to) || change.ToVersion().Equal(to)) {
				path = append(path, change)
			}
		}
	} else {
		// If migrating backward (from newer to older)
		// We need to reverse the changes that got us from 'to' to 'from'
		for _, change := range mc.changes {
			// Include changes that are in the path from 'to' to 'from'
			// A change is included if:
			// 1. Its FromVersion is >= to (Equal or IsNewerThan)
			// 2. Its ToVersion is <= from (IsOlderThan or Equal)
			if (change.FromVersion().Equal(to) || change.FromVersion().IsNewerThan(to)) &&
				(change.ToVersion().IsOlderThan(from) || change.ToVersion().Equal(from)) {
				path = append(path, change)
			}
		}
	}

	return path
}
