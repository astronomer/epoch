package cadwyn

import (
	"fmt"
)

// VersionBundle manages a collection of versions and their changes
type VersionBundle struct {
	headVersion   *Version
	versions      []*Version
	versionValues []string

	// All versions including head
	allVersions []*Version

	// Mapping from version changes to versions
	versionChangesToVersionMapping map[interface{}]string

	// Set of version values for quick lookup
	versionValuesSet map[string]bool

	// Versioned schemas and enums tracking
	versionedSchemas map[string]interface{}
	versionedEnums   map[string]interface{}
}

// NewVersionBundle creates a new version bundle
// If the first version is a head version, it becomes the head, otherwise a new head is created
// Returns an error if no versions are provided or if validation fails
func NewVersionBundle(versions []*Version) (*VersionBundle, error) {
	if len(versions) == 0 {
		return nil, fmt.Errorf("at least one version must be defined in a VersionBundle")
	}

	var headVersion *Version
	var regularVersions []*Version

	// Check if first version is head
	if versions[0].IsHead {
		headVersion = versions[0]
		regularVersions = versions[1:]
	} else {
		headVersion = NewHeadVersion()
		regularVersions = versions
	}

	// Single pass: collect values, check duplicates, and build mappings
	numVersions := len(regularVersions)
	versionValues := make([]string, numVersions)
	versionValuesSet := make(map[string]bool)
	versionChangesToVersionMapping := make(map[interface{}]string)

	for i, v := range regularVersions {
		versionStr := v.String()

		// Check for duplicates
		if versionValuesSet[versionStr] {
			return nil, fmt.Errorf("duplicate version detected: '%s' (versions must be unique)", versionStr)
		}
		versionValuesSet[versionStr] = true

		// Store values
		versionValues[i] = versionStr

		// Build version changes mapping
		for _, change := range v.Changes {
			versionChangesToVersionMapping[change] = versionStr
		}
	}

	// Validate that the oldest version (last in array) has no changes
	if numVersions > 0 {
		oldestVersion := regularVersions[numVersions-1]
		if len(oldestVersion.Changes) > 0 {
			return nil, fmt.Errorf("the oldest version '%s' cannot have version changes (it's the baseline with nothing to migrate from)",
				oldestVersion.String())
		}
	}

	// Create all versions slice
	allVersions := make([]*Version, 0, numVersions+1)
	allVersions = append(allVersions, headVersion)
	allVersions = append(allVersions, regularVersions...)

	vb := &VersionBundle{
		headVersion:                    headVersion,
		versions:                       regularVersions,
		versionValues:                  versionValues,
		allVersions:                    allVersions,
		versionChangesToVersionMapping: versionChangesToVersionMapping,
		versionValuesSet:               versionValuesSet,
		versionedSchemas:               make(map[string]interface{}),
		versionedEnums:                 make(map[string]interface{}),
	}

	return vb, nil
}

// GetHeadVersion returns the head version
func (vb *VersionBundle) GetHeadVersion() *Version {
	return vb.headVersion
}

// GetVersions returns all regular versions (excluding head)
func (vb *VersionBundle) GetVersions() []*Version {
	return vb.versions
}

// GetVersionValues returns all version values as strings
func (vb *VersionBundle) GetVersionValues() []string {
	return vb.versionValues
}

// ParseVersion parses a version string and returns the corresponding Version
func (vb *VersionBundle) ParseVersion(versionStr string) (*Version, error) {
	// Check for head version
	if versionStr == "head" || versionStr == "" {
		return vb.headVersion, nil
	}

	// Look for matching version
	for _, v := range vb.versions {
		if v.String() == versionStr {
			return v, nil
		}
	}

	return nil, fmt.Errorf("unknown version: %s", versionStr)
}

// GetVersionedSchemas returns a map of versioned schemas
func (vb *VersionBundle) GetVersionedSchemas() map[string]interface{} {
	if len(vb.versionedSchemas) == 0 {
		vb.buildVersionedSchemas()
	}
	return vb.versionedSchemas
}

// GetVersionedEnums returns a map of versioned enums
func (vb *VersionBundle) GetVersionedEnums() map[string]interface{} {
	if len(vb.versionedEnums) == 0 {
		vb.buildVersionedEnums()
	}
	return vb.versionedEnums
}

// buildVersionedSchemas builds the map of versioned schemas from all version changes
func (vb *VersionBundle) buildVersionedSchemas() {
	vb.versionedSchemas = make(map[string]interface{})

	for _, v := range vb.allVersions {
		versionKey := v.String()
		schemaMap := make(map[string]interface{})

		for _, change := range v.Changes {
			// Extract schemas from schema instructions
			schemaInstructions := change.GetSchemaInstructions()
			if schemaInstructions == nil {
				continue
			}

			// Type assert to slice of SchemaInstruction
			if instructions, ok := schemaInstructions.([]*SchemaInstruction); ok {
				for _, instruction := range instructions {
					// Store schema information for this version
					schemaMap[instruction.Name] = map[string]interface{}{
						"type":       instruction.Type,
						"attributes": instruction.Attributes,
						"version":    versionKey,
					}
				}
			}
		}

		if len(schemaMap) > 0 {
			vb.versionedSchemas[versionKey] = schemaMap
		}
	}
}

// buildVersionedEnums builds the map of versioned enums from all version changes
func (vb *VersionBundle) buildVersionedEnums() {
	vb.versionedEnums = make(map[string]interface{})

	for _, v := range vb.allVersions {
		versionKey := v.String()
		enumMap := make(map[string]interface{})

		for _, change := range v.Changes {
			// Extract enums from enum instructions
			enumInstructions := change.GetEnumInstructions()
			if enumInstructions == nil {
				continue
			}

			// Type assert to slice of EnumInstruction
			if instructions, ok := enumInstructions.([]*EnumInstruction); ok {
				for _, instruction := range instructions {
					// Store enum information for this version
					enumKey := fmt.Sprintf("%v", instruction.Enum)
					enumMap[enumKey] = map[string]interface{}{
						"type":      instruction.Type,
						"members":   instruction.Members,
						"is_hidden": instruction.IsHidden,
						"version":   versionKey,
					}
				}
			}
		}

		if len(enumMap) > 0 {
			vb.versionedEnums[versionKey] = enumMap
		}
	}
}

// IsVersionDefined checks if a version is defined in this bundle
func (vb *VersionBundle) IsVersionDefined(versionStr string) bool {
	if versionStr == "head" {
		return true
	}
	return vb.versionValuesSet[versionStr]
}

// Iterator returns an iterator over all versions
func (vb *VersionBundle) Iterator() []*Version {
	return vb.versions
}

// GetClosestLesserVersion finds the closest version that is less than the given version string
func (vb *VersionBundle) GetClosestLesserVersion(versionStr string) (string, error) {
	// Parse the target version for comparison
	targetVersion, err := NewVersion(versionStr, nil)
	if err != nil {
		return "", fmt.Errorf("invalid version string '%s': %w (available versions: %v)",
			versionStr, err, vb.GetVersionValues())
	}

	var closestVersion *Version

	// Check all versions (excluding head)
	for _, v := range vb.versions {
		if v.IsOlderThan(targetVersion) {
			if closestVersion == nil || v.IsNewerThan(closestVersion) {
				closestVersion = v
			}
		}
	}

	if closestVersion == nil {
		return "", fmt.Errorf("no version found that is less than '%s' (available versions: %v, all are >= requested)",
			versionStr, vb.GetVersionValues())
	}

	return closestVersion.String(), nil
}
