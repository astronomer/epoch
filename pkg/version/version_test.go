package version

import (
	"testing"
)

func TestNewDateVersion(t *testing.T) {
	tests := []struct {
		name        string
		dateStr     string
		expectError bool
		expectedRaw string
	}{
		{
			name:        "valid date",
			dateStr:     "2023-01-01",
			expectError: false,
			expectedRaw: "2023-01-01",
		},
		{
			name:        "invalid date format",
			dateStr:     "2023/01/01",
			expectError: true,
		},
		{
			name:        "invalid date",
			dateStr:     "2023-13-01",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := NewDateVersion(tt.dateStr)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if version.Raw != tt.expectedRaw {
				t.Errorf("expected raw '%s', got '%s'", tt.expectedRaw, version.Raw)
			}

			if version.Type != VersionTypeDate {
				t.Errorf("expected type %v, got %v", VersionTypeDate, version.Type)
			}

			if version.IsHead {
				t.Errorf("date version should not be head")
			}
		})
	}
}

func TestNewSemverVersion(t *testing.T) {
	tests := []struct {
		name          string
		semverStr     string
		expectError   bool
		expectedMajor int
		expectedMinor int
		expectedPatch int
	}{
		{
			name:          "valid semver",
			semverStr:     "1.2.3",
			expectError:   false,
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 3,
		},
		{
			name:          "valid semver with v prefix",
			semverStr:     "v2.1.0",
			expectError:   false,
			expectedMajor: 2,
			expectedMinor: 1,
			expectedPatch: 0,
		},
		{
			name:          "valid major.minor format",
			semverStr:     "1.2",
			expectError:   false,
			expectedMajor: 1,
			expectedMinor: 2,
			expectedPatch: 0,
		},
		{
			name:          "valid major.minor with v prefix",
			semverStr:     "v3.4",
			expectError:   false,
			expectedMajor: 3,
			expectedMinor: 4,
			expectedPatch: 0,
		},
		{
			name:        "invalid semver format",
			semverStr:   "a.b.c",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := NewSemverVersion(tt.semverStr)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if version.Major != tt.expectedMajor {
				t.Errorf("expected major %d, got %d", tt.expectedMajor, version.Major)
			}

			if version.Minor != tt.expectedMinor {
				t.Errorf("expected minor %d, got %d", tt.expectedMinor, version.Minor)
			}

			if version.Patch != tt.expectedPatch {
				t.Errorf("expected patch %d, got %d", tt.expectedPatch, version.Patch)
			}

			if version.Type != VersionTypeSemver {
				t.Errorf("expected type %v, got %v", VersionTypeSemver, version.Type)
			}
		})
	}
}

func TestVersionComparison(t *testing.T) {
	// Create test versions
	v1, _ := NewDateVersion("2023-01-01")
	v2, _ := NewDateVersion("2023-02-01")
	v3, _ := NewSemverVersion("1.0.0")
	v4, _ := NewSemverVersion("1.1.0")
	head := NewHeadVersion()

	tests := []struct {
		name     string
		v1       *Version
		v2       *Version
		expected int
	}{
		{
			name:     "date v1 < v2",
			v1:       v1,
			v2:       v2,
			expected: -1,
		},
		{
			name:     "date v2 > v1",
			v1:       v2,
			v2:       v1,
			expected: 1,
		},
		{
			name:     "semver v3 < v4",
			v1:       v3,
			v2:       v4,
			expected: -1,
		},
		{
			name:     "head > any version",
			v1:       head,
			v2:       v2,
			expected: 1,
		},
		{
			name:     "any version < head",
			v1:       v1,
			v2:       head,
			expected: -1,
		},
		{
			name:     "date < semver (different types)",
			v1:       v1,
			v2:       v3,
			expected: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.v1.Compare(tt.v2)
			if result != tt.expected {
				t.Errorf("expected %d, got %d", tt.expected, result)
			}
		})
	}
}

func TestVersionBundle(t *testing.T) {
	// Create test versions
	v1, _ := NewDateVersion("2023-01-01")
	v2, _ := NewDateVersion("2023-02-01")
	v3, _ := NewSemverVersion("1.0.0")
	head := NewHeadVersion()

	versions := []*Version{v2, v1, v3, head} // Intentionally unsorted

	bundle := NewVersionBundle(versions)

	// Test that versions are sorted
	sortedVersions := bundle.GetVersions()
	if len(sortedVersions) != 4 {
		t.Errorf("expected 4 versions, got %d", len(sortedVersions))
	}

	// Check order (v1 < v3 < v2 < head)
	if !sortedVersions[0].Equal(v1) {
		t.Errorf("first version should be v1 (2023-01-01)")
	}

	// Test head version
	if !bundle.GetHeadVersion().IsHead {
		t.Errorf("head version should be marked as head")
	}

	// Test finding versions
	found := bundle.FindVersion("2023-01-01")
	if found == nil || !found.Equal(v1) {
		t.Errorf("should find version 2023-01-01")
	}

	foundHead := bundle.FindVersion("head")
	if foundHead == nil || !foundHead.IsHead {
		t.Errorf("should find head version")
	}
}

func TestParseVersion(t *testing.T) {
	v1, _ := NewDateVersion("2023-01-01")
	v2, _ := NewSemverVersion("1.0.0")
	head := NewHeadVersion()

	bundle := NewVersionBundle([]*Version{v1, v2, head})

	tests := []struct {
		name         string
		versionStr   string
		expectError  bool
		expectedType VersionType
	}{
		{
			name:         "parse existing date version",
			versionStr:   "2023-01-01",
			expectError:  false,
			expectedType: VersionTypeDate,
		},
		{
			name:         "parse existing semver version",
			versionStr:   "1.0.0",
			expectError:  false,
			expectedType: VersionTypeSemver,
		},
		{
			name:         "parse head version",
			versionStr:   "head",
			expectError:  false,
			expectedType: VersionTypeHead,
		},
		{
			name:         "parse new date version",
			versionStr:   "2023-03-01",
			expectError:  false,
			expectedType: VersionTypeDate,
		},
		{
			name:         "parse new semver version",
			versionStr:   "2.0.0",
			expectError:  false,
			expectedType: VersionTypeSemver,
		},
		{
			name:        "parse invalid version",
			versionStr:  "invalid-version",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version, err := bundle.ParseVersion(tt.versionStr)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if version.Type != tt.expectedType {
				t.Errorf("expected type %v, got %v", tt.expectedType, version.Type)
			}
		})
	}
}
