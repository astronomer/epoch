package epoch

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	"github.com/bytedance/sonic/ast"
)

// AlterRequestInstruction defines how to modify a request during migration
type AlterRequestInstruction struct {
	Schemas     []interface{} // Types this instruction applies to (explicit type-based routing)
	Transformer func(*RequestInfo) error
}

// AlterResponseInstruction defines how to modify a response during migration
type AlterResponseInstruction struct {
	Schemas           []interface{} // Types this instruction applies to (explicit type-based routing)
	MigrateHTTPErrors bool          // Whether to migrate error responses
	Transformer       func(*ResponseInfo) error
}

// VersionChange defines a set of instructions for migrating between two API versions
// Uses explicit type-based routing where types are declared at endpoint registration
type VersionChange struct {
	description                            string
	isHiddenFromChangelog                  bool
	instructionsToMigrateToPreviousVersion []interface{}

	// Type-based instruction containers (explicit type routing from endpoint registry)
	alterRequestBySchemaInstructions  map[reflect.Type][]*AlterRequestInstruction
	alterResponseBySchemaInstructions map[reflect.Type][]*AlterResponseInstruction

	// Global instructions (apply to all types)
	globalRequestInstructions  []*AlterRequestInstruction
	globalResponseInstructions []*AlterResponseInstruction

	// Operation metadata for OpenAPI schema generation
	// These store the actual field operation lists (Add/Remove/Rename) for each type
	requestOperationsByType  map[reflect.Type]RequestToNextVersionOperationList
	responseOperationsByType map[reflect.Type]ResponseToPreviousVersionOperationList

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
		alterRequestBySchemaInstructions:       make(map[reflect.Type][]*AlterRequestInstruction),
		alterResponseBySchemaInstructions:      make(map[reflect.Type][]*AlterResponseInstruction),
		globalRequestInstructions:              make([]*AlterRequestInstruction, 0),
		globalResponseInstructions:             make([]*AlterResponseInstruction, 0),
		requestOperationsByType:                make(map[reflect.Type]RequestToNextVersionOperationList),
		responseOperationsByType:               make(map[reflect.Type]ResponseToPreviousVersionOperationList),
	}

	vc.extractInstructionsIntoContainers()
	return vc
}

// extractInstructionsIntoContainers organizes instructions by type
func (vc *VersionChange) extractInstructionsIntoContainers() {
	for _, instruction := range vc.instructionsToMigrateToPreviousVersion {
		switch inst := instruction.(type) {
		case *AlterRequestInstruction:
			if len(inst.Schemas) == 0 {
				// Global instruction (applies to all types)
				vc.globalRequestInstructions = append(vc.globalRequestInstructions, inst)
			} else {
				// Type-specific instruction
				for _, schema := range inst.Schemas {
					schemaType := reflect.TypeOf(schema)
					if schemaType.Kind() == reflect.Ptr {
						schemaType = schemaType.Elem()
					}

					vc.alterRequestBySchemaInstructions[schemaType] = append(
						vc.alterRequestBySchemaInstructions[schemaType], inst)
				}
			}

		case *AlterResponseInstruction:
			if len(inst.Schemas) == 0 {
				// Global instruction (applies to all types)
				vc.globalResponseInstructions = append(vc.globalResponseInstructions, inst)
			} else {
				// Type-specific instruction
				for _, schema := range inst.Schemas {
					schemaType := reflect.TypeOf(schema)
					if schemaType.Kind() == reflect.Ptr {
						schemaType = schemaType.Elem()
					}

					vc.alterResponseBySchemaInstructions[schemaType] = append(
						vc.alterResponseBySchemaInstructions[schemaType], inst)
				}
			}
		}
	}
}

// MigrateRequest applies request migrations for this version change using explicit types
func (vc *VersionChange) MigrateRequest(ctx context.Context, requestInfo *RequestInfo) error {
	if requestInfo.Body == nil {
		return nil // No body to migrate
	}

	// First, apply global instructions (apply to all requests)
	for _, instruction := range vc.globalRequestInstructions {
		if err := instruction.Transformer(requestInfo); err != nil {
			return fmt.Errorf("global request migration failed for change '%s': %w", vc.description, err)
		}
	}

	// Determine which type to use
	var matchedType reflect.Type
	if requestInfo.schemaMatched {
		// Use type from endpoint registry (set by MigrateRequestForType)
		matchedType = requestInfo.matchedSchemaType
	} else {
		// No type information available - endpoint not registered properly
		return nil
	}

	// Apply type-specific instructions using the matched type
	if matchedType != nil {
		if instructions, exists := vc.alterRequestBySchemaInstructions[matchedType]; exists {
			for _, instruction := range instructions {
				if err := instruction.Transformer(requestInfo); err != nil {
					return fmt.Errorf("type-based request migration failed for change '%s' (type: %s): %w",
						vc.description, matchedType.Name(), err)
				}
			}
		}
	}

	return nil
}

// MigrateResponse applies response migrations for this version change using explicit types
func (vc *VersionChange) MigrateResponse(ctx context.Context, responseInfo *ResponseInfo) error {
	// First, apply global instructions (apply to all responses)
	for _, instruction := range vc.globalResponseInstructions {
		// Check if we should migrate error responses
		if responseInfo.StatusCode >= 400 && !instruction.MigrateHTTPErrors {
			continue
		}
		if err := instruction.Transformer(responseInfo); err != nil {
			return fmt.Errorf("global response migration failed for change '%s': %w", vc.description, err)
		}
	}

	if responseInfo.Body == nil {
		return nil // No body to migrate
	}

	// Determine which type to use
	var matchedType reflect.Type
	if responseInfo.schemaMatched {
		// Use type from endpoint registry (set by MigrateResponseForType)
		matchedType = responseInfo.matchedSchemaType
	} else {
		// No type information available - endpoint not registered properly
		return nil
	}

	// Apply type-specific instructions using the matched type
	if matchedType != nil {
		if instructions, exists := vc.alterResponseBySchemaInstructions[matchedType]; exists {
			for _, instruction := range instructions {
				// Check if we should migrate error responses
				if responseInfo.StatusCode >= 400 && !instruction.MigrateHTTPErrors {
					continue
				}
				if err := instruction.Transformer(responseInfo); err != nil {
					return fmt.Errorf("type-based response migration failed for change '%s' (type: %s): %w",
						vc.description, matchedType.Name(), err)
				}
			}
		}

		if len(responseInfo.nestedObjectTypes) > 0 {
			for fieldPath, objectType := range responseInfo.nestedObjectTypes {
				if err := vc.transformNestedObjectForSingleStep(
					ctx, responseInfo, fieldPath, objectType,
				); err != nil {
					return fmt.Errorf("nested object migration failed for change '%s' (field: %s): %w",
						vc.description, fieldPath, err)
				}
			}
		}

		// After type-specific transformations, also transform nested arrays if defined
		if len(responseInfo.nestedArrayTypes) > 0 {
			// For each registered nested array, transform its items with type info
			for fieldPath, itemType := range responseInfo.nestedArrayTypes {
				if err := vc.transformNestedArrayItemsForSingleStep(
					ctx, responseInfo, fieldPath, itemType,
				); err != nil {
					return fmt.Errorf("nested array migration failed for change '%s' (field: %s): %w",
						vc.description, fieldPath, err)
				}
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

// GetRequestOperationsByType returns the request operations for a specific type
// This is used by OpenAPI schema generation to apply field operations to schemas
func (vc *VersionChange) GetRequestOperationsByType(targetType reflect.Type) (RequestToNextVersionOperationList, bool) {
	ops, exists := vc.requestOperationsByType[targetType]
	return ops, exists
}

// GetResponseOperationsByType returns the response operations for a specific type
// This is used by OpenAPI schema generation to apply field operations to schemas
func (vc *VersionChange) GetResponseOperationsByType(targetType reflect.Type) (ResponseToPreviousVersionOperationList, bool) {
	ops, exists := vc.responseOperationsByType[targetType]
	return ops, exists
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
	// If from and to are the same, no migration needed
	if from.Equal(to) {
		return nil
	}

	// Type information is set by MigrateRequestForType from the EndpointRegistry

	// If 'to' is HEAD, treat it as the latest non-HEAD version
	// since migrations are defined between numbered versions, not to/from HEAD
	targetVersion := to

	if targetVersion.IsHead {
		// Find the latest non-HEAD version
		var latestVersion *Version
		for _, change := range mc.changes {
			if change.ToVersion() != nil && !change.ToVersion().IsHead {
				if latestVersion == nil || change.ToVersion().IsNewerThan(latestVersion) {
					latestVersion = change.ToVersion()
				}
			}
		}
		if latestVersion != nil {
			targetVersion = latestVersion
		} else {
			// No migrations defined - HEAD equals the from version
			return nil
		}
	}

	// Early return if from equals target after HEAD resolution
	if from.Equal(targetVersion) {
		return nil
	}

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

		// Stop if this change would take us past the target
		if change.ToVersion().IsNewerThan(targetVersion) {
			break
		}

		// Apply this change if it's part of the migration path
		if (change.ToVersion().Equal(targetVersion) || change.ToVersion().IsOlderThan(targetVersion)) &&
			(change.FromVersion().IsOlderThan(targetVersion) || change.FromVersion().Equal(targetVersion)) {
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

	// Type information is set by MigrateResponseForType from the EndpointRegistry

	// If 'from' is HEAD, treat it as the latest non-HEAD version
	// since migrations are defined between numbered versions, not to/from HEAD
	currentVersion := from

	if currentVersion.IsHead {
		// Find the latest non-HEAD version
		var latestVersion *Version
		for _, change := range mc.changes {
			if change.ToVersion() != nil && !change.ToVersion().IsHead {
				if latestVersion == nil || change.ToVersion().IsNewerThan(latestVersion) {
					latestVersion = change.ToVersion()
				}
			}
		}
		if latestVersion != nil {
			currentVersion = latestVersion
		} else {
			return nil
		}
	}

	// Build the migration path from 'from' to 'to' (going backward through versions)
	// For example, from v3 to v1: v3→v2→v1
	// We need to apply changes at each step in reverse

	iterationCount := 0
	maxIterations := 10 // Safety limit

	for !currentVersion.Equal(to) {
		iterationCount++
		if iterationCount > maxIterations {
			return fmt.Errorf("migration loop exceeded max iterations (%d), possible cycle from %s to %s",
				maxIterations, from.String(), to.String())
		}

		// Find all changes that go FROM the next older version TO current version
		// We apply these in reverse (as current→older)
		var stepChanges []*VersionChange
		var nextVersion *Version

		for _, change := range mc.changes {
			// We want changes like v2→v3 to step back from v3→v2
			if change.ToVersion().Equal(currentVersion) && change.FromVersion().IsOlderThan(currentVersion) {
				// Pick the change that gets us closer to target 'to'
				if nextVersion == nil || change.FromVersion().IsOlderThan(nextVersion) {
					if change.FromVersion().Equal(to) || change.FromVersion().IsNewerThan(to) {
						nextVersion = change.FromVersion()
					}
				}
			}
		}

		if nextVersion == nil {
			return fmt.Errorf("no migration path found from version %s to %s (stuck at %s)",
				from.String(), to.String(), currentVersion.String())
		}

		// Collect ALL changes at this level (from nextVersion to currentVersion)
		for _, change := range mc.changes {
			if change.FromVersion().Equal(nextVersion) && change.ToVersion().Equal(currentVersion) {
				stepChanges = append(stepChanges, change)
			}
		}

		// Apply all changes at this level in reverse
		for _, change := range stepChanges {
			if err := change.MigrateResponse(ctx, responseInfo); err != nil {
				return fmt.Errorf("reverse migration failed at %s->%s: %w",
					change.ToVersion().String(), change.FromVersion().String(), err)
			}
		}

		// Move to next version level
		currentVersion = nextVersion
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

// MigrateRequestForType applies request migrations for a known type (NO runtime matching)
// This is used by the endpoint registry system where types are explicitly declared at setup time
func (mc *MigrationChain) MigrateRequestForType(
	ctx context.Context,
	requestInfo *RequestInfo,
	knownType reflect.Type,
	from, to *Version,
) error {
	// Set the known type directly - NO runtime matching needed
	requestInfo.schemaMatched = true
	requestInfo.matchedSchemaType = knownType

	// Apply all migrations in sequence using the existing logic
	return mc.MigrateRequest(ctx, requestInfo, from, to)
}

// MigrateResponseForType applies response migrations for a known type (NO runtime matching)
// This is used by the endpoint registry system where types are explicitly declared at setup time
// It also handles nested arrays and nested objects explicitly when provided
func (mc *MigrationChain) MigrateResponseForType(
	ctx context.Context,
	responseInfo *ResponseInfo,
	knownType reflect.Type,
	nestedArrays map[string]reflect.Type,
	from, to *Version,
) error {
	// Delegate to the extended version with nil nestedObjects for backward compatibility
	return mc.MigrateResponseForTypeWithNestedObjects(ctx, responseInfo, knownType, nestedArrays, nil, from, to)
}

// MigrateResponseForTypeWithNestedObjects applies response migrations for a known type
// with full support for nested arrays and nested objects
func (mc *MigrationChain) MigrateResponseForTypeWithNestedObjects(
	ctx context.Context,
	responseInfo *ResponseInfo,
	knownType reflect.Type,
	nestedArrays map[string]reflect.Type,
	nestedObjects map[string]reflect.Type,
	from, to *Version,
) error {
	// Check if the response type is a top-level array (e.g., []User)
	if knownType.Kind() == reflect.Slice || knownType.Kind() == reflect.Array {
		// Extract the element type from the array
		elementType := knownType.Elem()

		// Apply migrations to each array item
		return mc.transformTopLevelArrayItems(ctx, responseInfo, elementType, from, to)
	}

	// Set the known type and nested type information - NO runtime matching needed
	responseInfo.schemaMatched = true
	responseInfo.matchedSchemaType = knownType
	responseInfo.nestedArrayTypes = nestedArrays
	responseInfo.nestedObjectTypes = nestedObjects

	// Apply top-level migrations - nested types will be transformed at each step
	return mc.MigrateResponse(ctx, responseInfo, from, to)
}

// transformTopLevelArrayItems applies migrations to items in a top-level array response
// For example, when the handler returns []User directly
func (mc *MigrationChain) transformTopLevelArrayItems(
	ctx context.Context,
	responseInfo *ResponseInfo,
	itemType reflect.Type,
	from, to *Version,
) error {
	// Check if responseInfo.Body is an array
	if responseInfo.Body == nil {
		return nil
	}

	if responseInfo.Body.TypeSafe() != ast.V_ARRAY {
		return fmt.Errorf("expected array response but got %v", responseInfo.Body.TypeSafe())
	}

	// Get all array items
	arrayLen, err := responseInfo.Body.Len()
	if err != nil {
		return fmt.Errorf("failed to get array length: %w", err)
	}

	// Transform each item
	for i := 0; i < arrayLen; i++ {
		item := responseInfo.Body.Index(i)
		if item == nil {
			continue
		}

		// Create temporary ResponseInfo for the array item
		itemInfo := &ResponseInfo{
			Body:              item,
			schemaMatched:     true,
			matchedSchemaType: itemType,
		}

		// Apply migrations for the item type
		if err := mc.MigrateResponse(ctx, itemInfo, from, to); err != nil {
			return fmt.Errorf("failed to migrate array item %d: %w", i, err)
		}
	}

	return nil
}

// getNodeAtPath navigates to a nested node using dot-notation path
// Returns nil if any part of the path doesn't exist
func getNodeAtPath(root *ast.Node, path string) *ast.Node {
	if root == nil || path == "" {
		return root
	}

	parts := strings.Split(path, ".")
	current := root

	for _, part := range parts {
		if current == nil {
			return nil
		}
		current = current.Get(part)
		if current == nil || !current.Exists() {
			return nil
		}
	}

	return current
}

// transformNestedArrayItemsForSingleStep applies THIS version change's migrations
// to items in a nested array field (single step, not multi-step)
// Supports dot-notation paths for arrays inside nested objects (e.g., "profile.skills")
func (vc *VersionChange) transformNestedArrayItemsForSingleStep(
	ctx context.Context,
	responseInfo *ResponseInfo,
	fieldPath string,
	itemType reflect.Type,
) error {
	if responseInfo.Body == nil {
		return nil
	}

	// Navigate to the array field using dot-notation path
	arrayField := getNodeAtPath(responseInfo.Body, fieldPath)
	if arrayField == nil || !arrayField.Exists() || arrayField.TypeSafe() != ast.V_ARRAY {
		return nil
	}

	// Get instructions for the item type
	instructions, exists := vc.alterResponseBySchemaInstructions[itemType]
	if !exists {
		return nil // No migrations defined for this item type in this version change
	}

	// Transform each array item with this version change's operations
	length, err := arrayField.Len()
	if err != nil {
		return err
	}

	for i := 0; i < length; i++ {
		item := arrayField.Index(i)
		if item == nil {
			continue
		}

		itemInfo := &ResponseInfo{
			Body:              item,
			schemaMatched:     true,
			matchedSchemaType: itemType,
		}

		// Apply only THIS version change's instructions (single step)
		for _, instruction := range instructions {
			if responseInfo.StatusCode >= 400 && !instruction.MigrateHTTPErrors {
				continue
			}
			if err := instruction.Transformer(itemInfo); err != nil {
				return err
			}
		}
	}
	return nil
}

// transformNestedObjectForSingleStep applies THIS version change's migrations
// to a nested object field (single step, not multi-step)
// Supports dot-notation paths for deeply nested objects (e.g., "user.profile.address")
func (vc *VersionChange) transformNestedObjectForSingleStep(
	ctx context.Context,
	responseInfo *ResponseInfo,
	fieldPath string,
	objectType reflect.Type,
) error {
	if responseInfo.Body == nil {
		return nil
	}

	// Navigate to the object field using dot-notation path
	objectField := getNodeAtPath(responseInfo.Body, fieldPath)
	if objectField == nil || !objectField.Exists() || objectField.TypeSafe() != ast.V_OBJECT {
		return nil
	}

	// Get instructions for the object type
	instructions, exists := vc.alterResponseBySchemaInstructions[objectType]
	if !exists {
		return nil // No migrations defined for this object type in this version change
	}

	// Create ResponseInfo wrapper for the nested object
	objectInfo := &ResponseInfo{
		Body:              objectField,
		StatusCode:        responseInfo.StatusCode,
		schemaMatched:     true,
		matchedSchemaType: objectType,
	}

	// Apply only THIS version change's instructions (single step)
	for _, instruction := range instructions {
		if responseInfo.StatusCode >= 400 && !instruction.MigrateHTTPErrors {
			continue
		}
		if err := instruction.Transformer(objectInfo); err != nil {
			return err
		}
	}

	return nil
}
