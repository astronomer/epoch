package epoch

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

	// Validate that the oldest version (first in array) has no changes
	// Exception: if there's only one version, it can have changes (to HEAD)
	if numVersions > 1 {
		oldestVersion := regularVersions[0]
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

	return nil, fmt.Errorf("unknown version '%s': available versions are %v", versionStr, vb.versionValues)
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
