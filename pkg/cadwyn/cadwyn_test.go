package cadwyn

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/isaacchung/cadwyn-go/pkg/middleware"
	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

func TestNew(t *testing.T) {
	v1 := DateVersion("2023-01-01")
	v2 := DateVersion("2023-02-01")
	head := HeadVersion()

	config := Config{
		Versions:       []*version.Version{v1, v2, head},
		VersionChanges: []migration.VersionChange{},
	}

	app, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create Cadwyn app: %v", err)
	}

	if app == nil {
		t.Fatal("App should not be nil")
	}

	// Test that components are initialized
	if app.Router() == nil {
		t.Error("Router should be initialized")
	}

	if app.Middleware() == nil {
		t.Error("Middleware should be initialized")
	}

	// Test version access
	versions := app.GetVersions()
	if len(versions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(versions))
	}

	headVersion := app.GetHeadVersion()
	if !headVersion.IsHead {
		t.Error("Head version should be marked as head")
	}
}

func TestConfigValidation(t *testing.T) {
	// Test empty versions
	config := Config{
		Versions: []*version.Version{},
	}

	_, err := New(config)
	if err == nil {
		t.Error("Expected error for empty versions")
	}
}

func TestDefaults(t *testing.T) {
	config := Config{
		Versions: []*version.Version{DateVersion("2023-01-01")},
	}

	app, err := New(config)
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Check that defaults were applied
	if app.config.VersionLocation != middleware.VersionLocationHeader {
		t.Error("Default version location should be header")
	}

	if app.config.VersionParameterName != "x-api-version" {
		t.Error("Default version parameter should be x-api-version")
	}

	if app.config.DefaultVersion == nil {
		t.Error("Default version should be set")
	}
}

func TestBuilder(t *testing.T) {
	app, err := NewBuilder().
		WithDateVersions("2023-01-01", "2023-02-01").
		WithHeadVersion().
		WithVersionLocation(middleware.VersionLocationQuery).
		WithVersionParameter("api_version").
		WithSchemaAnalysis().
		WithDebugLogging().
		Build()

	if err != nil {
		t.Fatalf("Failed to build app: %v", err)
	}

	versions := app.GetVersions()
	if len(versions) != 3 {
		t.Errorf("Expected 3 versions, got %d", len(versions))
	}

	if app.config.VersionLocation != middleware.VersionLocationQuery {
		t.Error("Version location should be query")
	}

	if app.config.VersionParameterName != "api_version" {
		t.Error("Version parameter should be api_version")
	}

	if !app.config.EnableSchemaAnalysis {
		t.Error("Schema analysis should be enabled")
	}

	if !app.config.EnableDebugLogging {
		t.Error("Debug logging should be enabled")
	}
}

func TestQuickSetupFunctions(t *testing.T) {
	// Test NewWithDateVersions
	app1, err := NewWithDateVersions("2023-01-01", "2023-02-01")
	if err != nil {
		t.Fatalf("Failed to create app with date versions: %v", err)
	}

	if len(app1.GetVersions()) != 2 {
		t.Error("Should have 2 versions")
	}

	// Test NewWithSemverVersions
	app2, err := NewWithSemverVersions("1.0.0", "1.1.0")
	if err != nil {
		t.Fatalf("Failed to create app with semver versions: %v", err)
	}

	if len(app2.GetVersions()) != 2 {
		t.Error("Should have 2 versions")
	}

	// Test NewSimple
	app3, err := NewSimple()
	if err != nil {
		t.Fatalf("Failed to create simple app: %v", err)
	}

	if len(app3.GetVersions()) != 1 {
		t.Error("Should have 1 version")
	}

	if !app3.GetHeadVersion().IsHead {
		t.Error("Should have head version")
	}
}

func TestMustFunctions(t *testing.T) {
	// Test MustNew (should not panic with valid config)
	config := Config{
		Versions: []*version.Version{DateVersion("2023-01-01")},
	}

	app1 := MustNew(config)
	if app1 == nil {
		t.Error("MustNew should return valid app")
	}

	// Test MustNewWithDateVersions
	app2 := MustNewWithDateVersions("2023-01-01", "2023-02-01")
	if app2 == nil {
		t.Error("MustNewWithDateVersions should return valid app")
	}
}

func TestVersionHelpers(t *testing.T) {
	// Test DateVersion
	v1 := DateVersion("2023-01-01")
	if v1.String() != "2023-01-01" {
		t.Error("DateVersion should create valid date version")
	}

	// Test SemverVersion
	v2 := SemverVersion("1.0.0")
	if v2.String() != "1.0.0" {
		t.Error("SemverVersion should create valid semver version")
	}

	// Test HeadVersion
	head := HeadVersion()
	if !head.IsHead {
		t.Error("HeadVersion should create head version")
	}

	// Test Today (just check it doesn't panic)
	today := Today()
	if today == nil {
		t.Error("Today should return valid version")
	}
}

func TestHTTPIntegration(t *testing.T) {
	// Create app with versions
	app, err := NewWithDateVersions("2023-01-01", "2023-02-01")
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Register a test route
	app.Router().GET("/test", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World"))
	})

	// Test direct ServeHTTP
	req := httptest.NewRequest("GET", "/test", nil)
	rr := httptest.NewRecorder()

	app.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}

	if rr.Body.String() != "Hello World" {
		t.Errorf("Expected 'Hello World', got '%s'", rr.Body.String())
	}
}

func TestMiddlewareIntegration(t *testing.T) {
	app, err := NewWithDateVersions("2023-01-01")
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Test that middleware can be used with existing server
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Wrap with Cadwyn middleware
	wrappedHandler := app.Middleware()(testHandler)

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("x-api-version", "2023-01-01")
	rr := httptest.NewRecorder()

	wrappedHandler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rr.Code)
	}
}

func TestSchemaAnalysis(t *testing.T) {
	app, err := NewBuilder().
		WithDateVersions("2023-01-01").
		WithSchemaAnalysis().
		Build()

	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	type TestStruct struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	schema, err := app.AnalyzeSchema(TestStruct{})
	if err != nil {
		t.Fatalf("Failed to analyze schema: %v", err)
	}

	if schema.Name != "TestStruct" {
		t.Errorf("Expected schema name 'TestStruct', got '%s'", schema.Name)
	}

	if len(schema.Fields) != 2 {
		t.Errorf("Expected 2 fields, got %d", len(schema.Fields))
	}
}

func TestSchemaAnalysisDisabled(t *testing.T) {
	app, err := NewWithDateVersions("2023-01-01")
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	type TestStruct struct {
		ID int `json:"id"`
	}

	_, err = app.AnalyzeSchema(TestStruct{})
	if err == nil {
		t.Error("Expected error when schema analysis is disabled")
	}
}

func TestVersionChangeManagement(t *testing.T) {
	v1 := DateVersion("2023-01-01")
	v2 := DateVersion("2023-02-01")

	change := migration.NewBaseVersionChange("Test change", v1, v2)

	app, err := NewBuilder().
		WithVersions(v1, v2).
		WithVersionChanges(change).
		Build()

	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	changes := app.GetVersionChanges()
	if len(changes) != 1 {
		t.Errorf("Expected 1 version change, got %d", len(changes))
	}

	// Test adding version change
	newChange := migration.NewBaseVersionChange("Another change", v2, v1)
	app.AddVersionChange(newChange)

	updatedChanges := app.GetVersionChanges()
	if len(updatedChanges) != 2 {
		t.Errorf("Expected 2 version changes after adding, got %d", len(updatedChanges))
	}
}

func TestRouteInfo(t *testing.T) {
	app, err := NewWithDateVersions("2023-01-01")
	if err != nil {
		t.Fatalf("Failed to create app: %v", err)
	}

	// Register some routes
	app.Router().GET("/users", func(w http.ResponseWriter, r *http.Request) {})
	app.Router().POST("/users", func(w http.ResponseWriter, r *http.Request) {})

	routeInfo := app.GetRouteInfo()
	if len(routeInfo) != 2 {
		t.Errorf("Expected 2 routes, got %d", len(routeInfo))
	}

	// Test PrintRoutes (should not panic)
	app.PrintRoutes()
}
