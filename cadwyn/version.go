package cadwyn

import (
	"fmt"
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
	// Changes associated with this version (like Python Cadwyn)
	Changes []VersionChangeInterface
}

// VersionChangeInterface defines the interface for version changes
// This will be implemented by our migration.VersionChange struct
type VersionChangeInterface interface {
	Description() string
	FromVersion() *Version
	ToVersion() *Version
	GetSchemaInstructions() interface{} // Returns schema instructions
	GetEnumInstructions() interface{}   // Returns enum instructions
}

// VersionType represents the format of a version
type VersionType int

const (
	VersionTypeDate VersionType = iota
	VersionTypeSemver
	VersionTypeString
	VersionTypeHead
)

// String returns the string representation of the version type
func (vt VersionType) String() string {
	switch vt {
	case VersionTypeDate:
		return "date"
	case VersionTypeSemver:
		return "semver"
	case VersionTypeString:
		return "string"
	case VersionTypeHead:
		return "head"
	default:
		return "unknown"
	}
}

// NewVersion creates a new version with associated changes (like Python Cadwyn)
func NewVersion(value string, changes ...VersionChangeInterface) (*Version, error) {
	// Try to parse as date first
	if date, err := time.Parse("2006-01-02", value); err == nil {
		return &Version{
			Raw:     value,
			Date:    &date,
			Type:    VersionTypeDate,
			IsHead:  false,
			Changes: changes,
		}, nil
	}

	// Try to parse as semver
	if version, err := NewSemverVersion(value); err == nil {
		version.Changes = changes
		return version, nil
	}

	// Fallback to string version
	version := NewStringVersion(value, changes...)
	return version, nil
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
// Supports both major.minor.patch and major.minor formats
func NewSemverVersion(semverStr string) (*Version, error) {
	// Remove 'v' prefix if present
	semverStr = strings.TrimPrefix(semverStr, "v")

	var major, minor, patch int

	// Try major.minor.patch format first
	n, err := fmt.Sscanf(semverStr, "%d.%d.%d", &major, &minor, &patch)
	if err == nil && n == 3 {
		return &Version{
			Raw:    semverStr,
			Major:  major,
			Minor:  minor,
			Patch:  patch,
			Type:   VersionTypeSemver,
			IsHead: false,
		}, nil
	}

	// Try major.minor format (patch defaults to 0)
	n, err = fmt.Sscanf(semverStr, "%d.%d", &major, &minor)
	if err == nil && n == 2 {
		return &Version{
			Raw:    semverStr,
			Major:  major,
			Minor:  minor,
			Patch:  0, // Default patch to 0 for major.minor format
			Type:   VersionTypeSemver,
			IsHead: false,
		}, nil
	}

	return nil, fmt.Errorf("invalid semver format '%s': expected major.minor.patch or major.minor", semverStr)
}

// NewStringVersion creates a new string-based version
func NewStringVersion(versionStr string, changes ...VersionChangeInterface) *Version {
	return &Version{
		Raw:     versionStr,
		Type:    VersionTypeString,
		IsHead:  false,
		Changes: changes,
	}
}

// NewHeadVersion creates a head (latest) version
func NewHeadVersion(changes ...VersionChangeInterface) *Version {
	return &Version{
		Raw:     "head",
		Type:    VersionTypeHead,
		IsHead:  true,
		Changes: changes,
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
