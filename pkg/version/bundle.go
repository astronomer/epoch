package version

import (
	"context"
	"fmt"
	"reflect"
)

// VersionBundle manages a collection of versions and their changes
// This matches the Python Cadwyn VersionBundle class
type VersionBundle struct {
	headVersion           *Version
	versions              []*Version
	reversedVersions      []*Version
	versionValues         []string
	reversedVersionValues []string

	// Context variable for API version (like Python's api_version_var)
	apiVersionVar context.Context

	// All versions including head
	allVersions []*Version

	// Mapping from version changes to versions
	versionChangesToVersionMapping map[interface{}]string

	// Set of version values for quick lookup
	versionValuesSet map[string]bool

	// Versioned schemas and enums tracking
	versionedSchemas map[string]reflect.Type
	versionedEnums   map[string]reflect.Type
}

// NewVersionBundle creates a new version bundle
// If the first version is a head version, it becomes the head, otherwise a new head is created
func NewVersionBundle(versions []*Version) *VersionBundle {
	if len(versions) == 0 {
		panic("You must define at least one version in a VersionBundle")
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

	// Reverse the versions for easier processing
	reversedVersions := make([]*Version, len(regularVersions))
	for i, v := range regularVersions {
		reversedVersions[len(regularVersions)-1-i] = v
	}

	// Extract version values
	versionValues := make([]string, len(regularVersions))
	reversedVersionValues := make([]string, len(regularVersions))
	versionValuesSet := make(map[string]bool)

	for i, v := range regularVersions {
		versionValues[i] = v.String()
		reversedVersionValues[len(regularVersions)-1-i] = v.String()

		if versionValuesSet[v.String()] {
			panic(fmt.Sprintf("You tried to define two versions with the same value: '%s'", v.String()))
		}
		versionValuesSet[v.String()] = true
	}

	// Validate that the first version has no changes
	if len(regularVersions) > 0 && len(regularVersions[len(regularVersions)-1].Changes) > 0 {
		panic(fmt.Sprintf("The first version \"%s\" cannot have any version changes",
			regularVersions[len(regularVersions)-1].String()))
	}

	// Create all versions slice
	allVersions := make([]*Version, 0, len(regularVersions)+1)
	allVersions = append(allVersions, headVersion)
	allVersions = append(allVersions, regularVersions...)

	// Create version changes mapping
	versionChangesToVersionMapping := make(map[interface{}]string)
	for _, v := range regularVersions {
		for _, change := range v.Changes {
			versionChangesToVersionMapping[change] = v.String()
		}
	}

	return &VersionBundle{
		headVersion:                    headVersion,
		versions:                       regularVersions,
		reversedVersions:               reversedVersions,
		versionValues:                  versionValues,
		reversedVersionValues:          reversedVersionValues,
		allVersions:                    allVersions,
		versionChangesToVersionMapping: versionChangesToVersionMapping,
		versionValuesSet:               versionValuesSet,
		versionedSchemas:               make(map[string]reflect.Type),
		versionedEnums:                 make(map[string]reflect.Type),
	}
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

// GetClosestLesserVersion finds the closest version that is less than or equal to the given version
func (vb *VersionBundle) GetClosestLesserVersion(targetVersion string) (string, error) {
	for _, definedVersion := range vb.versionValues {
		if definedVersion <= targetVersion {
			return definedVersion, nil
		}
	}
	return "", fmt.Errorf("no version found that is earlier than or equal to %s", targetVersion)
}

// GetVersionedSchemas returns a map of versioned schemas
func (vb *VersionBundle) GetVersionedSchemas() map[string]reflect.Type {
	if len(vb.versionedSchemas) == 0 {
		vb.buildVersionedSchemas()
	}
	return vb.versionedSchemas
}

// GetVersionedEnums returns a map of versioned enums
func (vb *VersionBundle) GetVersionedEnums() map[string]reflect.Type {
	if len(vb.versionedEnums) == 0 {
		vb.buildVersionedEnums()
	}
	return vb.versionedEnums
}

// buildVersionedSchemas builds the map of versioned schemas from all version changes
func (vb *VersionBundle) buildVersionedSchemas() {
	for _, v := range vb.allVersions {
		for _, change := range v.Changes {
			// Extract schemas from schema instructions
			// TODO: Implement proper schema instruction extraction
			// This is a placeholder for the new architecture
			_ = change.GetSchemaInstructions()
		}
	}
}

// buildVersionedEnums builds the map of versioned enums from all version changes
func (vb *VersionBundle) buildVersionedEnums() {
	for _, v := range vb.allVersions {
		for _, change := range v.Changes {
			// Extract enums from enum instructions
			// TODO: Implement proper enum instruction extraction
			// This is a placeholder for the new architecture
			_ = change.GetEnumInstructions()
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
