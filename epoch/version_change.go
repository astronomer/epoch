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
	// NOTE: Runtime type matching has been removed. All endpoints must use EndpointRegistry
	// to explicitly declare their types at setup time. The matchedSchemaType is set by MigrateRequestForType.

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

	// NOTE: Runtime type matching has been removed. All endpoints must use EndpointRegistry
	// to explicitly declare their types at setup time. The matchedSchemaType is set by MigrateResponseForType.

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

// NOTE: Runtime schema matching has been removed entirely.
// The new architecture uses EndpointRegistry for explicit type registration at endpoint setup time.

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
// It also handles nested arrays explicitly when provided
func (mc *MigrationChain) MigrateResponseForType(
	ctx context.Context,
	responseInfo *ResponseInfo,
	knownType reflect.Type,
	nestedArrays map[string]reflect.Type,
	from, to *Version,
) error {
	// Set the known type directly - NO runtime matching needed
	responseInfo.schemaMatched = true
	responseInfo.matchedSchemaType = knownType

	// Apply nested array transformations FIRST if defined
	if len(nestedArrays) > 0 {
		for fieldName, itemType := range nestedArrays {
			if err := mc.transformNestedArrayItems(
				ctx, responseInfo, fieldName, itemType, from, to,
			); err != nil {
				return err
			}
		}
	}

	// Then apply top-level migrations using the existing logic
	return mc.MigrateResponse(ctx, responseInfo, from, to)
}

// transformNestedArrayItems applies migrations to items in a nested array field
// For example, transforms UserResponse items inside UsersListResponse.users array
func (mc *MigrationChain) transformNestedArrayItems(
	ctx context.Context,
	responseInfo *ResponseInfo,
	fieldName string,
	itemType reflect.Type,
	from, to *Version,
) error {
	// Get the array field
	if responseInfo.Body == nil {
		return nil
	}

	arrayField := responseInfo.Body.Get(fieldName)
	if arrayField == nil || !arrayField.Exists() {
		return nil // Field doesn't exist
	}

	if arrayField.TypeSafe() != ast.V_ARRAY {
		return nil // Field exists but isn't an array
	}

	// Transform each item in the array
	return responseInfo.TransformArrayField(fieldName, func(item *ast.Node) error {
		// Create temporary ResponseInfo for the array item
		itemInfo := &ResponseInfo{
			Body:              item,
			schemaMatched:     true,
			matchedSchemaType: itemType,
		}

		// Apply migrations for the item type
		return mc.MigrateResponse(ctx, itemInfo, from, to)
	})
}

// ============================================================================
// VERSION CHANGE BUILDER - Fluent API for building migrations
// ============================================================================

// versionChangeBuilder implements the flow-based fluent builder API
// Following the actual migration flow:
// - Requests: Client Version → HEAD Version (always forward)
// - Responses: HEAD Version → Client Version (always backward)
type versionChangeBuilder struct {
	description    string
	fromVersion    *Version
	toVersion      *Version
	typeOps        map[reflect.Type]*typeBuilder
	customRequest  func(*RequestInfo) error
	customResponse func(*ResponseInfo) error
}

// NewVersionChangeBuilder creates a new type-based version change builder
func NewVersionChangeBuilder(fromVersion, toVersion *Version) *versionChangeBuilder {
	return &versionChangeBuilder{
		fromVersion: fromVersion,
		toVersion:   toVersion,
		typeOps:     make(map[reflect.Type]*typeBuilder),
	}
}

// Description sets the human-readable description of the change
func (b *versionChangeBuilder) Description(desc string) *versionChangeBuilder {
	b.description = desc
	return b
}

// ForType starts building operations for specific types
// This allows targeting migrations to specific Go struct types (e.g., UserResponse)
// Types are explicitly declared at endpoint registration via WrapHandler().Returns()/.Accepts()
func (b *versionChangeBuilder) ForType(types ...interface{}) *typeBuilder {
	tb := &typeBuilder{
		parent:                       b,
		targetTypes:                  make([]reflect.Type, 0, len(types)),
		requestToNextVersionOps:      make(RequestToNextVersionOperationList, 0),
		responseToPreviousVersionOps: make(ResponseToPreviousVersionOperationList, 0),
	}

	// Convert types to reflect.Type
	for _, t := range types {
		reflectType := reflect.TypeOf(t)
		if reflectType.Kind() == reflect.Ptr {
			reflectType = reflectType.Elem()
		}
		tb.targetTypes = append(tb.targetTypes, reflectType)

		// Store by first type for retrieval
		if len(tb.targetTypes) == 1 {
			b.typeOps[reflectType] = tb
		}
	}

	return tb
}

// CustomRequest adds a global custom request transformer
func (b *versionChangeBuilder) CustomRequest(fn func(*RequestInfo) error) *versionChangeBuilder {
	b.customRequest = fn
	return b
}

// CustomResponse adds a global custom response transformer
func (b *versionChangeBuilder) CustomResponse(fn func(*ResponseInfo) error) *versionChangeBuilder {
	b.customResponse = fn
	return b
}

// Build compiles all operations into a VersionChange
func (b *versionChangeBuilder) Build() *VersionChange {
	if b.description == "" {
		b.description = "Migration from " + b.fromVersion.String() + " to " + b.toVersion.String()
	}

	// Validate: require at least one type or custom transformer
	if len(b.typeOps) == 0 && b.customRequest == nil && b.customResponse == nil {
		panic("epoch: VersionChange must specify at least one type using ForType() or custom transformers")
	}

	var instructions []interface{}

	// Compile type-based operations into instructions
	for _, tb := range b.typeOps {
		if len(tb.requestToNextVersionOps) == 0 &&
			len(tb.responseToPreviousVersionOps) == 0 {
			continue
		}

		// CRITICAL: Create local copies to avoid closure variable capture bug
		// Without this, all closures would reference the same (last) tb
		tbCopy := tb

		// Get field mappings for error transformation
		fieldMappings := make(map[string]string)

		// Combine field mappings from both operation types
		for k, v := range tbCopy.requestToNextVersionOps.GetFieldMappings() {
			fieldMappings[k] = v
		}
		for k, v := range tbCopy.responseToPreviousVersionOps.GetFieldMappings() {
			fieldMappings[k] = v
		}

		// Create request instruction for each type
		for _, targetType := range tbCopy.targetTypes {
			// CRITICAL: Create local copy for closure
			targetTypeCopy := targetType
			requestOpsCopy := tbCopy.requestToNextVersionOps

			requestInst := &AlterRequestInstruction{
				Schemas: []interface{}{reflect.New(targetTypeCopy).Interface()},
				Transformer: func(req *RequestInfo) error {
					if req.Body == nil {
						return nil
					}

					// Request migration is always FROM client version TO HEAD version
					// Apply "to next version" operations (Client→HEAD)
					return requestOpsCopy.Apply(req.Body)
				},
			}
			instructions = append(instructions, requestInst)

			// Create response instruction
			responseOpsCopy := tbCopy.responseToPreviousVersionOps
			fieldMappingsCopy := make(map[string]string)
			for k, v := range fieldMappings {
				fieldMappingsCopy[k] = v
			}

			responseInst := &AlterResponseInstruction{
				Schemas:           []interface{}{reflect.New(targetTypeCopy).Interface()},
				MigrateHTTPErrors: true,
				Transformer: func(resp *ResponseInfo) error {
					if resp.Body != nil {
						// Handle arrays and objects separately
						if resp.Body.TypeSafe() == ast.V_ARRAY {
							// For arrays, apply operations to each item
							if err := resp.TransformArrayField("", func(node *ast.Node) error {
								// Response migration is always FROM HEAD version TO client version
								// Apply "to previous version" operations (HEAD→Client)
								return responseOpsCopy.Apply(node)
							}); err != nil {
								return err
							}
						} else {
							// For objects, apply operations to the object and handle nested arrays
							// Response migration is always FROM HEAD version TO client version
							if err := responseOpsCopy.Apply(resp.Body); err != nil {
								return err
							}

							// Handle nested arrays within objects
							if err := resp.TransformNestedArrays(func(node *ast.Node) error {
								return responseOpsCopy.Apply(node)
							}); err != nil {
								return err
							}
						}
					}

					// Additionally transform field names in error messages for validation errors
					if len(fieldMappingsCopy) > 0 {
						return transformErrorFieldNamesInResponse(resp, fieldMappingsCopy)
					}

					return nil
				},
			}
			instructions = append(instructions, responseInst)
		}
	}

	// Add custom transformers if provided
	if b.customRequest != nil {
		instructions = append(instructions, &AlterRequestInstruction{
			Schemas:     []interface{}{}, // Global
			Transformer: b.customRequest,
		})
	}

	if b.customResponse != nil {
		instructions = append(instructions, &AlterResponseInstruction{
			Schemas:           []interface{}{}, // Global
			MigrateHTTPErrors: true,
			Transformer:       b.customResponse,
		})
	}

	return NewVersionChange(b.description, b.fromVersion, b.toVersion, instructions...)
}

// typeBuilder builds operations for specific types
type typeBuilder struct {
	parent                       *versionChangeBuilder
	targetTypes                  []reflect.Type
	requestToNextVersionOps      RequestToNextVersionOperationList
	responseToPreviousVersionOps ResponseToPreviousVersionOperationList
}

// RequestToNextVersion returns a builder for request operations (Client→HEAD)
// This is the ONLY direction requests flow
func (tb *typeBuilder) RequestToNextVersion() *requestToNextVersionBuilder {
	return &requestToNextVersionBuilder{parent: tb}
}

// ResponseToPreviousVersion returns a builder for response operations (HEAD→Client)
// This is the ONLY direction responses flow
func (tb *typeBuilder) ResponseToPreviousVersion() *responseToPreviousVersionBuilder {
	return &responseToPreviousVersionBuilder{parent: tb}
}

// ForType returns to the parent and starts a new type builder
func (tb *typeBuilder) ForType(types ...interface{}) *typeBuilder {
	return tb.parent.ForType(types...)
}

// Build is a convenience method that calls the parent's Build()
func (tb *typeBuilder) Build() *VersionChange {
	return tb.parent.Build()
}

type requestToNextVersionBuilder struct {
	parent *typeBuilder
}

// AddField adds a field when request migrates from client to HEAD
func (b *requestToNextVersionBuilder) AddField(name string, defaultValue interface{}) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestAddField{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// AddFieldWithDefault adds a field ONLY if missing (Cadwyn-style default handling)
func (b *requestToNextVersionBuilder) AddFieldWithDefault(name string, defaultValue interface{}) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestAddFieldWithDefault{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// RemoveField removes a field when request migrates from client to HEAD
func (b *requestToNextVersionBuilder) RemoveField(name string) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestRemoveField{
			Name: name,
		})
	return b
}

// RenameField renames a field when request migrates from client to HEAD
func (b *requestToNextVersionBuilder) RenameField(from, to string) *requestToNextVersionBuilder {
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestRenameField{
			From: from,
			To:   to,
		})
	return b
}

// Custom applies a custom transformation function to the request
func (b *requestToNextVersionBuilder) Custom(fn func(*RequestInfo) error) *requestToNextVersionBuilder {
	// Wrap RequestInfo function to work with ast.Node
	b.parent.requestToNextVersionOps = append(b.parent.requestToNextVersionOps,
		&RequestCustom{
			Fn: func(node *ast.Node) error {
				// Create a temporary RequestInfo wrapper
				req := &RequestInfo{Body: node}
				return fn(req)
			},
		})
	return b
}

// Back to response builder
func (b *requestToNextVersionBuilder) ResponseToPreviousVersion() *responseToPreviousVersionBuilder {
	return b.parent.ResponseToPreviousVersion()
}

// Back to type builder
func (b *requestToNextVersionBuilder) ForType(types ...interface{}) *typeBuilder {
	return b.parent.ForType(types...)
}

// Build completes the builder chain
func (b *requestToNextVersionBuilder) Build() *VersionChange {
	return b.parent.Build()
}

type responseToPreviousVersionBuilder struct {
	parent *typeBuilder
}

// AddField adds a field when response migrates from HEAD to client
func (b *responseToPreviousVersionBuilder) AddField(name string, defaultValue interface{}) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseAddField{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// RemoveField removes a field when response migrates from HEAD to client
func (b *responseToPreviousVersionBuilder) RemoveField(name string) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseRemoveField{
			Name: name,
		})
	return b
}

// RemoveFieldIfDefault removes a field ONLY if it equals the default value
func (b *responseToPreviousVersionBuilder) RemoveFieldIfDefault(name string, defaultValue interface{}) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseRemoveFieldIfDefault{
			Name:    name,
			Default: defaultValue,
		})
	return b
}

// RenameField renames a field when response migrates from HEAD to client
func (b *responseToPreviousVersionBuilder) RenameField(from, to string) *responseToPreviousVersionBuilder {
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseRenameField{
			From: from,
			To:   to,
		})
	return b
}

// Custom applies a custom transformation function to the response
func (b *responseToPreviousVersionBuilder) Custom(fn func(*ResponseInfo) error) *responseToPreviousVersionBuilder {
	// Wrap ResponseInfo function to work with ast.Node
	b.parent.responseToPreviousVersionOps = append(b.parent.responseToPreviousVersionOps,
		&ResponseCustom{
			Fn: func(node *ast.Node) error {
				// Create a temporary ResponseInfo wrapper
				resp := &ResponseInfo{Body: node}
				return fn(resp)
			},
		})
	return b
}

// Back to request builder
func (b *responseToPreviousVersionBuilder) RequestToNextVersion() *requestToNextVersionBuilder {
	return b.parent.RequestToNextVersion()
}

// Back to type builder
func (b *responseToPreviousVersionBuilder) ForType(types ...interface{}) *typeBuilder {
	return b.parent.ForType(types...)
}

// Build completes the builder chain
func (b *responseToPreviousVersionBuilder) Build() *VersionChange {
	return b.parent.Build()
}

// ============================================================================
// ERROR FIELD NAME TRANSFORMATION HELPERS
// ============================================================================

// transformErrorFieldNamesInResponse transforms field names in error messages
func transformErrorFieldNamesInResponse(resp *ResponseInfo, fieldMapping map[string]string) error {
	// Only transform validation errors (400 Bad Request)
	if resp.StatusCode != 400 || resp.Body == nil {
		return nil
	}

	errorNode := resp.Body.Get("error")
	if errorNode == nil || !errorNode.Exists() {
		return nil
	}

	// Handle simple string errors
	if errorNode.TypeSafe() == ast.V_STRING {
		errorStr, _ := errorNode.String()
		transformedError := replaceFieldNamesInErrorString(errorStr, fieldMapping)
		resp.SetField("error", transformedError)
		return nil
	}

	// Handle structured errors with message
	if errorNode.TypeSafe() == ast.V_OBJECT {
		messageNode := errorNode.Get("message")
		if messageNode != nil && messageNode.Exists() {
			messageStr, _ := messageNode.String()
			transformedMessage := replaceFieldNamesInErrorString(messageStr, fieldMapping)

			// Reconstruct error object
			errorObj := map[string]interface{}{
				"message": transformedMessage,
			}

			// Preserve code if it exists
			codeNode := errorNode.Get("code")
			if codeNode != nil && codeNode.Exists() {
				code, _ := codeNode.String()
				errorObj["code"] = code
			}

			resp.SetField("error", errorObj)
		}
	}

	return nil
}

// replaceFieldNamesInErrorString replaces field names in error messages
func replaceFieldNamesInErrorString(errorMsg string, fieldMapping map[string]string) string {
	result := errorMsg

	for newField, oldField := range fieldMapping {
		// Replace various formats
		patterns := []struct {
			old string
			new string
		}{
			{newField, oldField},
			{toPascalCaseString(newField), toPascalCaseString(oldField)},
			{"'" + newField + "'", "'" + oldField + "'"},
			{"\"" + newField + "\"", "\"" + oldField + "\""},
			{"Key: 'User." + toPascalCaseString(newField) + "'", "Key: 'User." + toPascalCaseString(oldField) + "'"},
		}

		for _, p := range patterns {
			result = strings.ReplaceAll(result, p.old, p.new)
		}
	}

	return result
}

// toPascalCaseString converts snake_case to PascalCase
func toPascalCaseString(s string) string {
	if s == "" {
		return ""
	}

	// Handle common API naming conventions
	s = strings.Replace(s, "ID", "Id", -1)
	s = strings.Replace(s, "URL", "Url", -1)
	s = strings.Replace(s, "HTTP", "Http", -1)
	s = strings.Replace(s, "API", "Api", -1)

	// Split by underscores for snake_case
	parts := strings.Split(s, "_")

	var result strings.Builder
	result.Grow(len(s))

	for _, part := range parts {
		if part == "" {
			continue
		}

		// Capitalize first character
		runes := []rune(part)
		if len(runes) > 0 {
			result.WriteString(strings.ToUpper(string(runes[0])))
			if len(runes) > 1 {
				result.WriteString(string(runes[1:]))
			}
		}
	}

	return result.String()
}
