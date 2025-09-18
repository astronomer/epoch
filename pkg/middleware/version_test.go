package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

func TestVersionDetection(t *testing.T) {
	// Create test versions
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")
	head := version.NewHeadVersion()

	versionBundle := version.NewVersionBundle([]*version.Version{v1, v2, head})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	tests := []struct {
		name            string
		location        VersionLocation
		parameterName   string
		setupRequest    func(*http.Request)
		expectedVersion string
	}{
		{
			name:          "header version detection",
			location:      VersionLocationHeader,
			parameterName: "x-api-version",
			setupRequest: func(r *http.Request) {
				r.Header.Set("x-api-version", "2023-01-01")
			},
			expectedVersion: "2023-01-01",
		},
		{
			name:          "query parameter version detection",
			location:      VersionLocationQuery,
			parameterName: "version",
			setupRequest: func(r *http.Request) {
				r.URL.RawQuery = "version=2023-02-01"
			},
			expectedVersion: "2023-02-01",
		},
		{
			name:          "path version detection with v prefix",
			location:      VersionLocationPath,
			parameterName: "v",
			setupRequest: func(r *http.Request) {
				r.URL.Path = "/v2023-01-01/users"
			},
			expectedVersion: "2023-01-01",
		},
		{
			name:            "no version specified - uses default",
			location:        VersionLocationHeader,
			parameterName:   "x-api-version",
			setupRequest:    func(r *http.Request) {}, // no version set
			expectedVersion: "head",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			middleware := NewVersionMiddleware(Config{
				VersionBundle:  versionBundle,
				MigrationChain: migrationChain,
				ParameterName:  tt.parameterName,
				Location:       tt.location,
			})

			// Create test request
			req := httptest.NewRequest("GET", "/users", nil)
			tt.setupRequest(req)

			// Create response recorder
			rr := httptest.NewRecorder()

			// Create test handler that checks the version in context
			var detectedVersion *version.Version
			testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				detectedVersion = GetVersionFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			// Apply middleware
			handler := middleware.Middleware()(testHandler)
			handler.ServeHTTP(rr, req)

			// Check detected version
			if detectedVersion == nil {
				t.Errorf("no version detected in context")
				return
			}

			if detectedVersion.String() != tt.expectedVersion {
				t.Errorf("expected version '%s', got '%s'", tt.expectedVersion, detectedVersion.String())
			}
		})
	}
}

func TestVersionMiddlewareIntegration(t *testing.T) {
	// Create test versions and changes
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")
	head := version.NewHeadVersion()

	versionBundle := version.NewVersionBundle([]*version.Version{v1, v2, head})

	// Create version changes: v1->v2 (add email), v2->head
	change1 := migration.NewStructVersionChange(
		"Add email field",
		v1, v2,
		[]migration.FieldChange{migration.AddField("email", "default@example.com")},
		[]migration.FieldChange{migration.RemoveField("email")},
	)

	change2 := migration.NewStructVersionChange(
		"Upgrade to head",
		v2, head,
		[]migration.FieldChange{}, // no changes needed
		[]migration.FieldChange{}, // no changes needed
	)

	migrationChain := migration.NewMigrationChain([]migration.VersionChange{change1, change2})

	middleware := NewVersionMiddleware(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
		ParameterName:  "x-api-version",
		Location:       VersionLocationHeader,
	})

	// Test handler that returns JSON
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"id": 1, "name": "Alice", "email": "alice@example.com"}`))
	})

	// Test with v1 (should remove email field from response)
	req := httptest.NewRequest("GET", "/users", nil)
	req.Header.Set("x-api-version", "2023-01-01")

	rr := httptest.NewRecorder()
	handler := middleware.Middleware()(testHandler)
	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	// Check that response was migrated (email field should be removed)
	responseBody := rr.Body.String()
	if strings.Contains(responseBody, "email") {
		t.Errorf("email field should be removed for v1, got: %s", responseBody)
	}
}

func TestGetVersionFromContext(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")

	// Test with version in context
	ctx := context.WithValue(context.Background(), versionContextKey, v1)
	detectedVersion := GetVersionFromContext(ctx)

	if detectedVersion == nil {
		t.Errorf("should detect version from context")
	} else if !detectedVersion.Equal(v1) {
		t.Errorf("detected version should equal v1")
	}

	// Test with no version in context
	emptyCtx := context.Background()
	noVersion := GetVersionFromContext(emptyCtx)

	if noVersion != nil {
		t.Errorf("should return nil when no version in context")
	}
}

func TestExtractVersionFromPath(t *testing.T) {
	versionBundle := version.NewVersionBundle([]*version.Version{})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	middleware := NewVersionMiddleware(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
		ParameterName:  "v",
		Location:       VersionLocationPath,
	})

	tests := []struct {
		path     string
		expected string
	}{
		{"/v2023-01-01/users", "2023-01-01"},
		{"/v1.0.0/users", "1.0.0"},
		{"/2023-01-01/users", "2023-01-01"},
		{"/1.0.0/users", "1.0.0"},
		{"/users", ""},
		{"/invalid/users", ""},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := middleware.extractVersionFromPath(tt.path)
			if result != tt.expected {
				t.Errorf("path '%s': expected '%s', got '%s'", tt.path, tt.expected, result)
			}
		})
	}
}

func TestLooksLikeVersion(t *testing.T) {
	middleware := &VersionMiddleware{}

	tests := []struct {
		input    string
		expected bool
	}{
		{"2023-01-01", true},
		{"1.0.0", true},
		{"2.1.3", true},
		{"invalid", false},
		{"2023/01/01", false},
		{"1.0", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := middleware.looksLikeVersion(tt.input)
			if result != tt.expected {
				t.Errorf("input '%s': expected %t, got %t", tt.input, tt.expected, result)
			}
		})
	}
}

func TestResponseWrapper(t *testing.T) {
	// Create a simple response wrapper test
	rr := httptest.NewRecorder()

	v1, _ := version.NewDateVersion("2023-01-01")
	head := version.NewHeadVersion()
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	wrapper := &responseWrapper{
		ResponseWriter:   rr,
		requestedVersion: v1,
		headVersion:      head,
		migrationChain:   migrationChain,
		ctx:              context.Background(),
	}

	// Test header setting
	wrapper.Header().Set("Content-Type", "application/json")

	// Test writing
	wrapper.WriteHeader(http.StatusOK)
	wrapper.Write([]byte(`{"test": "value"}`))

	// Check that response was written
	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", rr.Code)
	}

	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type header to be set")
	}
}
