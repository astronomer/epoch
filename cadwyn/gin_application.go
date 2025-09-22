package cadwyn

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Application represents a Cadwyn Gin application
type Application struct {
	engine            *gin.Engine
	versionBundle     *VersionBundle
	migrationChain    *MigrationChain
	schemaGenerator   *SchemaGenerator
	versionMiddleware *VersionMiddleware
	versionedRouter   *VersionedRouter

	// Configuration
	config *ApplicationConfig
}

// ApplicationConfig holds configuration for the Cadwyn Gin application
type ApplicationConfig struct {
	// Version configuration
	VersionBundle  *VersionBundle
	MigrationChain *MigrationChain

	// Version detection configuration
	VersionLocation      VersionLocation
	VersionParameterName string
	VersionFormat        VersionFormat
	DefaultVersion       *Version

	// Application configuration
	Title       string
	Description string
	Version     string

	// API documentation
	DocsURL    string
	OpenAPIURL string

	// Features
	EnableSchemaGeneration bool
	EnableChangelog        bool
	EnableDebugLogging     bool

	// Gin configuration
	GinMode string // "debug", "release", "test"
}

// NewApplication creates a new Cadwyn Gin application
func NewApplication(config *ApplicationConfig) (*Application, error) {
	if config.VersionBundle == nil {
		return nil, fmt.Errorf("version bundle is required")
	}

	// Apply defaults
	applyApplicationDefaults(config)

	// Set Gin mode
	gin.SetMode(config.GinMode)

	// Create Gin engine
	engine := gin.New()

	// Add default middleware
	engine.Use(gin.Logger())
	engine.Use(gin.Recovery())

	// Create schema generator
	var schemaGenerator *SchemaGenerator
	if config.EnableSchemaGeneration {
		schemaGenerator = NewSchemaGenerator(config.VersionBundle, config.MigrationChain)
	}

	// Create version middleware
	middlewareConfig := MiddlewareConfig{
		VersionBundle:  config.VersionBundle,
		MigrationChain: config.MigrationChain,
		ParameterName:  config.VersionParameterName,
		Location:       config.VersionLocation,
		Format:         config.VersionFormat,
		DefaultVersion: config.DefaultVersion,
	}
	versionMiddleware := NewVersionMiddleware(middlewareConfig)

	// Create versioned router
	routerConfig := RouterConfig{
		VersionBundle:  config.VersionBundle,
		MigrationChain: config.MigrationChain,
	}
	versionedRouter := NewVersionedRouter(routerConfig)

	app := &Application{
		engine:            engine,
		versionBundle:     config.VersionBundle,
		migrationChain:    config.MigrationChain,
		schemaGenerator:   schemaGenerator,
		versionMiddleware: versionMiddleware,
		versionedRouter:   versionedRouter,
		config:            config,
	}

	// Add version middleware to engine
	app.engine.Use(app.versionMiddleware.Middleware())

	// Add utility endpoints
	app.addUtilityEndpoints()

	return app, nil
}

// applyApplicationDefaults applies default configuration values
func applyApplicationDefaults(config *ApplicationConfig) {
	if config.VersionLocation == "" {
		config.VersionLocation = VersionLocationHeader
	}

	if config.VersionParameterName == "" {
		switch config.VersionLocation {
		case VersionLocationHeader:
			config.VersionParameterName = "X-API-Version"
		case VersionLocationQuery:
			config.VersionParameterName = "version"
		case VersionLocationPath:
			config.VersionParameterName = "v"
		}
	}

	if config.VersionFormat == "" {
		config.VersionFormat = VersionFormatDate
	}

	if config.DefaultVersion == nil && config.VersionBundle != nil {
		config.DefaultVersion = config.VersionBundle.GetHeadVersion()
	}

	if config.Title == "" {
		config.Title = "Cadwyn API"
	}

	if config.Version == "" {
		config.Version = "1.0.0"
	}

	if config.DocsURL == "" {
		config.DocsURL = "/docs"
	}

	if config.OpenAPIURL == "" {
		config.OpenAPIURL = "/openapi.json"
	}

	if config.GinMode == "" {
		config.GinMode = gin.ReleaseMode
	}
}

// addUtilityEndpoints adds built-in utility endpoints
func (app *Application) addUtilityEndpoints() {
	// Add changelog endpoint
	if app.config.EnableChangelog {
		app.engine.GET("/changelog", app.handleChangelog)
	}

	// Add OpenAPI endpoint
	if app.config.OpenAPIURL != "" {
		app.engine.GET(app.config.OpenAPIURL, app.handleOpenAPI)
	}

	// Add docs endpoint
	if app.config.DocsURL != "" {
		app.engine.GET(app.config.DocsURL, app.handleDocs)
	}

	// Add version info endpoint
	app.engine.GET("/versions", app.handleVersions)

	// Add health check
	app.engine.GET("/health", app.handleHealth)
}

// GET registers a GET handler
func (app *Application) GET(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.GET(relativePath, handlers...)
}

// POST registers a POST handler
func (app *Application) POST(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.POST(relativePath, handlers...)
}

// PUT registers a PUT handler
func (app *Application) PUT(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.PUT(relativePath, handlers...)
}

// DELETE registers a DELETE handler
func (app *Application) DELETE(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.DELETE(relativePath, handlers...)
}

// PATCH registers a PATCH handler
func (app *Application) PATCH(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.PATCH(relativePath, handlers...)
}

// HEAD registers a HEAD handler
func (app *Application) HEAD(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.HEAD(relativePath, handlers...)
}

// OPTIONS registers an OPTIONS handler
func (app *Application) OPTIONS(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.OPTIONS(relativePath, handlers...)
}

// Any registers a handler for all HTTP methods
func (app *Application) Any(relativePath string, handlers ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.Any(relativePath, handlers...)
}

// Group creates a route group
func (app *Application) Group(relativePath string, handlers ...gin.HandlerFunc) *gin.RouterGroup {
	return app.engine.Group(relativePath, handlers...)
}

// Use attaches a global middleware to the router
func (app *Application) Use(middleware ...gin.HandlerFunc) gin.IRoutes {
	return app.engine.Use(middleware...)
}

// GetEngine returns the underlying Gin engine
func (app *Application) GetEngine() *gin.Engine {
	return app.engine
}

// GetVersionedRouter returns the versioned router
func (app *Application) GetVersionedRouter() *VersionedRouter {
	return app.versionedRouter
}

// Run starts the HTTP server on the specified address
func (app *Application) Run(addr ...string) error {
	if app.config.EnableDebugLogging {
		fmt.Printf("🚀 Starting Cadwyn Gin server\n")
		if len(addr) > 0 {
			fmt.Printf("📡 Address: %s\n", addr[0])
		} else {
			fmt.Printf("📡 Address: :8080\n")
		}
		fmt.Printf("📚 API Documentation: %s\n", app.config.DocsURL)
		fmt.Printf("📋 OpenAPI Spec: %s\n", app.config.OpenAPIURL)
		fmt.Printf("🔄 Versions: %v\n", app.getVersionStrings())
	}

	return app.engine.Run(addr...)
}

// Utility endpoint handlers

// handleChangelog returns the API changelog
func (app *Application) handleChangelog(c *gin.Context) {
	changelog := app.generateChangelog()
	c.JSON(http.StatusOK, changelog)
}

// handleOpenAPI returns the OpenAPI specification
func (app *Application) handleOpenAPI(c *gin.Context) {
	requestedVersion := c.Query("version")
	if requestedVersion == "" {
		requestedVersion = "head"
	}

	openAPISpec := app.generateOpenAPISpec(requestedVersion)
	c.JSON(http.StatusOK, openAPISpec)
}

// handleDocs returns the API documentation dashboard
func (app *Application) handleDocs(c *gin.Context) {
	requestedVersion := c.Query("version")

	if requestedVersion != "" {
		// Return specific version docs
		app.renderVersionSpecificDocs(c, requestedVersion)
		return
	}

	// Return version selection dashboard
	app.renderDocsDashboard(c)
}

// handleVersions returns available API versions
func (app *Application) handleVersions(c *gin.Context) {
	versions := gin.H{
		"versions": app.getVersionStrings(),
		"head":     app.versionBundle.GetHeadVersion().String(),
		"default":  app.config.DefaultVersion.String(),
	}
	c.JSON(http.StatusOK, versions)
}

// handleHealth returns health status
func (app *Application) handleHealth(c *gin.Context) {
	health := gin.H{
		"status":    "healthy",
		"timestamp": "2024-01-01T00:00:00Z", // Would use actual timestamp
		"versions":  len(app.versionBundle.GetVersions()),
		"app":       app.config.Title,
	}
	c.JSON(http.StatusOK, health)
}

// Helper methods

// generateChangelog generates the API changelog
func (app *Application) generateChangelog() gin.H {
	// This would implement actual changelog generation
	// based on version changes, similar to Python Cadwyn's changelogs.py

	changelog := gin.H{
		"versions": []gin.H{},
	}

	versions := make([]gin.H, 0)
	for _, v := range app.versionBundle.GetVersions() {
		versionChangelog := gin.H{
			"version": v.String(),
			"changes": []gin.H{
				{
					"type":        "version_created",
					"description": fmt.Sprintf("Version %s created", v.String()),
				},
			},
		}
		versions = append(versions, versionChangelog)
	}

	changelog["versions"] = versions
	return changelog
}

// generateOpenAPISpec generates OpenAPI specification for a version
func (app *Application) generateOpenAPISpec(targetVersion string) gin.H {
	spec := gin.H{
		"openapi": "3.0.0",
		"info": gin.H{
			"title":       app.config.Title,
			"description": app.config.Description,
			"version":     targetVersion,
		},
		"paths": gin.H{
			"/health": gin.H{
				"get": gin.H{
					"summary":     "Health check",
					"description": "Returns the health status of the API",
					"responses": gin.H{
						"200": gin.H{
							"description": "Healthy",
						},
					},
				},
			},
		},
	}

	return spec
}

// renderDocsDashboard renders the documentation dashboard
func (app *Application) renderDocsDashboard(c *gin.Context) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>` + app.config.Title + ` - API Documentation</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 40px; }
        .version-table { border-collapse: collapse; width: 100%; }
        .version-table th, .version-table td { border: 1px solid #ddd; padding: 12px; text-align: left; }
        .version-table th { background-color: #f2f2f2; }
        .version-link { color: #0066cc; text-decoration: none; }
        .version-link:hover { text-decoration: underline; }
    </style>
</head>
<body>
    <h1>` + app.config.Title + ` - API Documentation</h1>
    <p>` + app.config.Description + `</p>
    
    <h2>Available Versions</h2>
    <table class="version-table">
        <tr><th>Version</th><th>Documentation</th><th>OpenAPI Spec</th></tr>`

	for _, v := range app.versionBundle.GetVersions() {
		versionStr := v.String()
		html += fmt.Sprintf(`
        <tr>
            <td>%s</td>
            <td><a href="%s?version=%s" class="version-link">View Docs</a></td>
            <td><a href="%s?version=%s" class="version-link">OpenAPI JSON</a></td>
        </tr>`, versionStr, app.config.DocsURL, versionStr, app.config.OpenAPIURL, versionStr)
	}

	html += `
    </table>
</body>
</html>`

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// renderVersionSpecificDocs renders documentation for a specific version
func (app *Application) renderVersionSpecificDocs(c *gin.Context, targetVersion string) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>%s - API Documentation v%s</title>
</head>
<body>
    <h1>%s - Version %s</h1>
    <p>Documentation for API version %s</p>
    <p><a href="%s">← Back to version selection</a></p>
    
    <!-- This would integrate with Swagger UI or similar -->
    <div id="swagger-ui"></div>
    
    <script>
        // Would load Swagger UI here with version-specific OpenAPI spec
        console.log('Loading docs for version: %s');
    </script>
</body>
</html>`, app.config.Title, targetVersion, app.config.Title, targetVersion, targetVersion, app.config.DocsURL, targetVersion)

	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(html))
}

// getVersionStrings returns all version strings
func (app *Application) getVersionStrings() []string {
	versions := make([]string, len(app.versionBundle.GetVersions()))
	for i, v := range app.versionBundle.GetVersions() {
		versions[i] = v.String()
	}
	return versions
}

// GetVersionBundle returns the version bundle
func (app *Application) GetVersionBundle() *VersionBundle {
	return app.versionBundle
}

// GetSchemaGenerator returns the schema generator
func (app *Application) GetSchemaGenerator() *SchemaGenerator {
	return app.schemaGenerator
}

// GenerateStructForVersion generates Go code for a struct at a specific version
func (app *Application) GenerateStructForVersion(structType interface{}, targetVersion string) (string, error) {
	if app.schemaGenerator == nil {
		return "", fmt.Errorf("schema generation is not enabled")
	}

	// Use reflection to get the type
	// This is a simplified version - would need more sophisticated type handling
	return app.schemaGenerator.GenerateStruct(nil, targetVersion)
}
