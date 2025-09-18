package version

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Version represents a specific API version
type Version struct {
	// Raw string representation of the version
	Raw string
	// Parsed date (for date-based versions)
	Date *time.Time
	// Semantic version components (for semver)
	Major, Minor, Patch int
	// Type indicates the version format
	Type VersionType
	// IsHead indicates if this is the latest version
	IsHead bool
}

// VersionType represents the format of a version
type VersionType int

const (
	VersionTypeDate VersionType = iota
	VersionTypeSemver
	VersionTypeHead
)

// String returns the string representation of the version type
func (vt VersionType) String() string {
	switch vt {
	case VersionTypeDate:
		return "date"
	case VersionTypeSemver:
		return "semver"
	case VersionTypeHead:
		return "head"
	default:
		return "unknown"
	}
}

// NewDateVersion creates a new date-based version
func NewDateVersion(dateStr string) (*Version, error) {
	date, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		return nil, fmt.Errorf("invalid date format '%s': %w", dateStr, err)
	}

	return &Version{
		Raw:    dateStr,
		Date:   &date,
		Type:   VersionTypeDate,
		IsHead: false,
	}, nil
}

// NewSemverVersion creates a new semantic version
func NewSemverVersion(semverStr string) (*Version, error) {
	// Remove 'v' prefix if present
	semverStr = strings.TrimPrefix(semverStr, "v")

	var major, minor, patch int
	n, err := fmt.Sscanf(semverStr, "%d.%d.%d", &major, &minor, &patch)
	if err != nil || n != 3 {
		return nil, fmt.Errorf("invalid semver format '%s'", semverStr)
	}

	return &Version{
		Raw:    semverStr,
		Major:  major,
		Minor:  minor,
		Patch:  patch,
		Type:   VersionTypeSemver,
		IsHead: false,
	}, nil
}

// NewHeadVersion creates a head (latest) version
func NewHeadVersion() *Version {
	return &Version{
		Raw:    "head",
		Type:   VersionTypeHead,
		IsHead: true,
	}
}

// String returns the string representation of the version
func (v *Version) String() string {
	if v.IsHead {
		return "head"
	}
	return v.Raw
}

// Compare compares this version with another version
// Returns: -1 if v < other, 0 if v == other, 1 if v > other
func (v *Version) Compare(other *Version) int {
	// Head version is always the latest
	if v.IsHead && !other.IsHead {
		return 1
	}
	if !v.IsHead && other.IsHead {
		return -1
	}
	if v.IsHead && other.IsHead {
		return 0
	}

	// Compare same types
	if v.Type == other.Type {
		switch v.Type {
		case VersionTypeDate:
			return v.compareDates(other)
		case VersionTypeSemver:
			return v.compareSemver(other)
		}
	}

	// Different types - date versions are considered older than semver
	if v.Type == VersionTypeDate && other.Type == VersionTypeSemver {
		return -1
	}
	if v.Type == VersionTypeSemver && other.Type == VersionTypeDate {
		return 1
	}

	// Fallback to string comparison
	if v.Raw < other.Raw {
		return -1
	}
	if v.Raw > other.Raw {
		return 1
	}
	return 0
}

// compareDates compares two date-based versions
func (v *Version) compareDates(other *Version) int {
	if v.Date == nil || other.Date == nil {
		return 0
	}

	if v.Date.Before(*other.Date) {
		return -1
	}
	if v.Date.After(*other.Date) {
		return 1
	}
	return 0
}

// compareSemver compares two semantic versions
func (v *Version) compareSemver(other *Version) int {
	if v.Major != other.Major {
		if v.Major < other.Major {
			return -1
		}
		return 1
	}

	if v.Minor != other.Minor {
		if v.Minor < other.Minor {
			return -1
		}
		return 1
	}

	if v.Patch != other.Patch {
		if v.Patch < other.Patch {
			return -1
		}
		return 1
	}

	return 0
}

// IsOlderThan returns true if this version is older than the other
func (v *Version) IsOlderThan(other *Version) bool {
	return v.Compare(other) < 0
}

// IsNewerThan returns true if this version is newer than the other
func (v *Version) IsNewerThan(other *Version) bool {
	return v.Compare(other) > 0
}

// Equal returns true if this version equals the other
func (v *Version) Equal(other *Version) bool {
	return v.Compare(other) == 0
}

// VersionBundle manages a collection of versions
type VersionBundle struct {
	versions    []*Version
	headVersion *Version
}

// NewVersionBundle creates a new version bundle
func NewVersionBundle(versions []*Version) *VersionBundle {
	// Sort versions in ascending order (oldest to newest)
	sortedVersions := make([]*Version, len(versions))
	copy(sortedVersions, versions)

	sort.Slice(sortedVersions, func(i, j int) bool {
		return sortedVersions[i].Compare(sortedVersions[j]) < 0
	})

	// Find or create head version
	var headVersion *Version
	for _, v := range sortedVersions {
		if v.IsHead {
			headVersion = v
			break
		}
	}

	// If no explicit head version, create one
	if headVersion == nil {
		headVersion = NewHeadVersion()
	}

	return &VersionBundle{
		versions:    sortedVersions,
		headVersion: headVersion,
	}
}

// GetVersions returns all versions in ascending order
func (vb *VersionBundle) GetVersions() []*Version {
	return vb.versions
}

// GetHeadVersion returns the head (latest) version
func (vb *VersionBundle) GetHeadVersion() *Version {
	return vb.headVersion
}

// GetLatestNonHeadVersion returns the latest non-head version
func (vb *VersionBundle) GetLatestNonHeadVersion() *Version {
	for i := len(vb.versions) - 1; i >= 0; i-- {
		if !vb.versions[i].IsHead {
			return vb.versions[i]
		}
	}
	return nil
}

// FindVersion finds a version by its string representation
func (vb *VersionBundle) FindVersion(versionStr string) *Version {
	if versionStr == "head" || versionStr == "" {
		return vb.headVersion
	}

	for _, v := range vb.versions {
		if v.Raw == versionStr {
			return v
		}
	}

	return nil
}

// ParseVersion attempts to parse a version string into a Version object
func (vb *VersionBundle) ParseVersion(versionStr string) (*Version, error) {
	if versionStr == "head" || versionStr == "" {
		return vb.headVersion, nil
	}

	// Try existing version first
	if existing := vb.FindVersion(versionStr); existing != nil {
		return existing, nil
	}

	// Try parsing as date
	if dateVersion, err := NewDateVersion(versionStr); err == nil {
		return dateVersion, nil
	}

	// Try parsing as semver
	if semverVersion, err := NewSemverVersion(versionStr); err == nil {
		return semverVersion, nil
	}

	return nil, fmt.Errorf("unable to parse version: %s", versionStr)
}

// AddVersion adds a new version to the bundle
func (vb *VersionBundle) AddVersion(version *Version) {
	vb.versions = append(vb.versions, version)

	// Re-sort versions
	sort.Slice(vb.versions, func(i, j int) bool {
		return vb.versions[i].Compare(vb.versions[j]) < 0
	})
}

// GetVersionsBetween returns all versions between from and to (inclusive)
func (vb *VersionBundle) GetVersionsBetween(from, to *Version) []*Version {
	var result []*Version

	for _, v := range vb.versions {
		if (v.Equal(from) || v.IsNewerThan(from)) &&
			(v.Equal(to) || v.IsOlderThan(to)) {
			result = append(result, v)
		}
	}

	return result
}
