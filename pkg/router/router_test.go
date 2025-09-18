package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

func TestVersionedRouter(t *testing.T) {
	// Create test versions
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")
	head := version.NewHeadVersion()

	versionBundle := version.NewVersionBundle([]*version.Version{v1, v2, head})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Register a test handler
	testHandler := func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello World"))
	}

	router.GET("/users", testHandler)

	// Test the route
	req := httptest.NewRequest("GET", "/users", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", rr.Body.String())
	}
}

func TestHTTPMethods(t *testing.T) {
	versionBundle := version.NewVersionBundle([]*version.Version{})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Register handlers for different methods
	router.GET("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("GET"))
	})

	router.POST("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("POST"))
	})

	router.PUT("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("PUT"))
	})

	router.DELETE("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("DELETE"))
	})

	router.PATCH("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("PATCH"))
	})

	// Test each method
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH"}

	for _, method := range methods {
		req := httptest.NewRequest(method, "/test", nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Method %s: expected status 200, got %d", method, rr.Code)
		}

		if rr.Body.String() != method {
			t.Errorf("Method %s: expected '%s', got '%s'", method, method, rr.Body.String())
		}
	}
}

func TestRouteMiddleware(t *testing.T) {
	versionBundle := version.NewVersionBundle([]*version.Version{})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Add middleware
	middleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "applied")
			next.ServeHTTP(w, r)
		})
	}

	router.Use("GET", "/test", middleware)

	router.GET("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Header().Get("X-Middleware") != "applied" {
		t.Errorf("Middleware was not applied")
	}
}

func TestVersionSpecificRoutes(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")
	head := version.NewHeadVersion()

	versionBundle := version.NewVersionBundle([]*version.Version{v1, v2, head})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Register route with specific versions
	router.GET("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users"))
	}, v1, v2)

	// Generate versioned routes
	err := router.GenerateVersionedRoutes()
	if err != nil {
		t.Fatalf("Failed to generate versioned routes: %v", err)
	}

	// Test version-specific paths
	tests := []struct {
		path         string
		expectStatus int
	}{
		{"/users", http.StatusOK},             // Original path
		{"/v2023-01-01/users", http.StatusOK}, // Version-specific path
		{"/v2023-02-01/users", http.StatusOK}, // Version-specific path
	}

	for _, tt := range tests {
		req := httptest.NewRequest("GET", tt.path, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != tt.expectStatus {
			t.Errorf("Path %s: expected status %d, got %d", tt.path, tt.expectStatus, rr.Code)
		}
	}
}

func TestRouteGroup(t *testing.T) {
	versionBundle := version.NewVersionBundle([]*version.Version{})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Create a route group
	api := router.Group("/api")

	// Add group middleware
	api.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-API-Group", "true")
			next.ServeHTTP(w, r)
		})
	})

	// Register routes in the group
	api.GET("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users"))
	})

	api.POST("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("create user"))
	})

	// Test group routes
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{"GET", "/api/users", "users"},
		{"POST", "/api/users", "create user"},
	}

	for _, tt := range tests {
		req := httptest.NewRequest(tt.method, tt.path, nil)
		rr := httptest.NewRecorder()

		router.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("%s %s: expected status 200, got %d", tt.method, tt.path, rr.Code)
		}

		if rr.Body.String() != tt.body {
			t.Errorf("%s %s: expected '%s', got '%s'", tt.method, tt.path, tt.body, rr.Body.String())
		}

		if rr.Header().Get("X-API-Group") != "true" {
			t.Errorf("%s %s: group middleware was not applied", tt.method, tt.path)
		}
	}
}

func TestVersionAwareHandler(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")

	versionBundle := version.NewVersionBundle([]*version.Version{v1})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Register version-aware handler
	versionHandler := WrapVersionAware(func(w http.ResponseWriter, r *http.Request, v *version.Version) {
		if v != nil {
			w.Write([]byte("Version: " + v.String()))
		} else {
			w.Write([]byte("No version"))
		}
	})

	router.GET("/version", versionHandler)

	// Test without version in context
	req := httptest.NewRequest("GET", "/version", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Body.String() != "No version" {
		t.Errorf("Expected 'No version', got '%s'", rr.Body.String())
	}

	// Test with version in context
	ctx := context.WithValue(context.Background(), "cadwyn_version", v1)
	req = httptest.NewRequest("GET", "/version", nil)
	req = req.WithContext(ctx)
	rr = httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Body.String() != "Version: 2023-01-01" {
		t.Errorf("Expected 'Version: 2023-01-01', got '%s'", rr.Body.String())
	}
}

func TestGetRouteInfo(t *testing.T) {
	v1, _ := version.NewDateVersion("2023-01-01")
	v2, _ := version.NewDateVersion("2023-02-01")

	versionBundle := version.NewVersionBundle([]*version.Version{v1, v2})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	router.GET("/users", func(w http.ResponseWriter, r *http.Request) {}, v1, v2)
	router.POST("/users", func(w http.ResponseWriter, r *http.Request) {}, v1)

	info := router.GetRouteInfo()

	if len(info) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(info))
	}

	// Check that route info contains expected data
	for _, routeInfo := range info {
		if routeInfo.Pattern == "/users" && routeInfo.Method == "GET" {
			if len(routeInfo.Versions) != 2 {
				t.Errorf("GET /users should have 2 versions, got %d", len(routeInfo.Versions))
			}
		}
		if routeInfo.Pattern == "/users" && routeInfo.Method == "POST" {
			if len(routeInfo.Versions) != 1 {
				t.Errorf("POST /users should have 1 version, got %d", len(routeInfo.Versions))
			}
		}
	}
}

func TestGenerateVersionedPath(t *testing.T) {
	versionBundle := version.NewVersionBundle([]*version.Version{})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	v1, _ := version.NewDateVersion("2023-01-01")
	head := version.NewHeadVersion()

	tests := []struct {
		pattern  string
		version  *version.Version
		expected string
	}{
		{"/users", v1, "/v2023-01-01/users"},
		{"/users", head, "/users"},
		{"/api/users", v1, "/v2023-01-01/api/users"},
	}

	for _, tt := range tests {
		result := router.generateVersionedPath(tt.pattern, tt.version)
		if result != tt.expected {
			t.Errorf("Pattern '%s' with version '%s': expected '%s', got '%s'",
				tt.pattern, tt.version.String(), tt.expected, result)
		}
	}
}

func TestMethodNotAllowed(t *testing.T) {
	versionBundle := version.NewVersionBundle([]*version.Version{})
	migrationChain := migration.NewMigrationChain([]migration.VersionChange{})

	router := NewVersionedRouter(Config{
		VersionBundle:  versionBundle,
		MigrationChain: migrationChain,
	})

	// Register only GET handler
	router.GET("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("GET"))
	})

	// Try to access with POST method
	req := httptest.NewRequest("POST", "/test", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if rr.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", rr.Code)
	}

	body := strings.TrimSpace(rr.Body.String())
	if !strings.Contains(body, "Method not allowed") {
		t.Errorf("Expected 'Method not allowed' in response, got '%s'", body)
	}
}
