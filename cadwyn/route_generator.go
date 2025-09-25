package cadwyn

import (
	"fmt"
	"net/http"
	"reflect"

	"github.com/gin-gonic/gin"
)

// RouteGenerator generates version-specific routes based on version changes
// This is inspired by Python Cadwyn's route_generation.py
type RouteGenerator struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
}

// GeneratedRoutes holds the result of route generation
type GeneratedRoutes struct {
	Endpoints map[string]*VersionedRouter // version -> router
	Webhooks  map[string]*VersionedRouter // version -> webhook router
}

// NewRouteGenerator creates a new route generator
func NewRouteGenerator(versionBundle *VersionBundle, migrationChain *MigrationChain) *RouteGenerator {
	return &RouteGenerator{
		versionBundle:  versionBundle,
		migrationChain: migrationChain,
	}
}

// GenerateVersionedRoutes generates version-specific routers from a head router
func (rg *RouteGenerator) GenerateVersionedRoutes(headRouter *VersionedRouter, webhookRouter *VersionedRouter) (*GeneratedRoutes, error) {
	if webhookRouter == nil {
		webhookRouter = NewVersionedRouter(RouterConfig{
			VersionBundle:  rg.versionBundle,
			MigrationChain: rg.migrationChain,
		})
	}

	result := &GeneratedRoutes{
		Endpoints: make(map[string]*VersionedRouter),
		Webhooks:  make(map[string]*VersionedRouter),
	}

	// Start with head version
	currentRouter := rg.copyRouter(headRouter)
	currentWebhookRouter := rg.copyRouter(webhookRouter)

	// Generate routers for each version
	for _, v := range rg.versionBundle.GetVersions() {
		// Apply version changes to get the router for this version
		if err := rg.applyVersionChangesToRouter(currentRouter, v); err != nil {
			return nil, fmt.Errorf("failed to apply version changes for version %s: %w", v.String(), err)
		}

		// Store the router for this version
		result.Endpoints[v.String()] = rg.copyRouter(currentRouter)
		result.Webhooks[v.String()] = rg.copyRouter(currentWebhookRouter)

		// Prepare for next version (apply changes in reverse)
		currentRouter = rg.copyRouter(currentRouter)
		currentWebhookRouter = rg.copyRouter(currentWebhookRouter)
	}

	return result, nil
}

// copyRouter creates a deep copy of a router
func (rg *RouteGenerator) copyRouter(original *VersionedRouter) *VersionedRouter {
	newRouter := NewVersionedRouter(RouterConfig{
		VersionBundle:           original.versionBundle,
		MigrationChain:          original.migrationChain,
		APIVersionParameterName: original.apiVersionParameterName,
		RedirectSlashes:         original.redirectSlashes,
	})

	// Copy routes
	for routeKey, route := range original.routes {
		newRoute := &Route{
			Pattern:     route.Pattern,
			Method:      route.Method,
			Handler:     route.Handler,
			Versions:    make([]*Version, len(route.Versions)),
			IsDeleted:   route.IsDeleted,
			Middlewares: make([]gin.HandlerFunc, len(route.Middlewares)),
		}

		copy(newRoute.Versions, route.Versions)
		copy(newRoute.Middlewares, route.Middlewares)

		newRouter.routes[routeKey] = newRoute
	}

	return newRouter
}

// applyVersionChangesToRouter applies version changes to transform a router
func (rg *RouteGenerator) applyVersionChangesToRouter(router *VersionedRouter, targetVersion *Version) error {
	// Find all changes that need to be applied
	changes := rg.migrationChain.GetChanges()

	for _, change := range changes {
		// Only apply changes that affect this version
		if change.ToVersion().Equal(targetVersion) || change.FromVersion().Equal(targetVersion) {
			if err := rg.applyVersionChangeToRouter(router, change, targetVersion); err != nil {
				return fmt.Errorf("failed to apply version change %s: %w", change.Description(), err)
			}
		}
	}

	return nil
}

// applyVersionChangeToRouter applies a single version change to a router
func (rg *RouteGenerator) applyVersionChangeToRouter(router *VersionedRouter, change *VersionChange, targetVersion *Version) error {
	// Apply endpoint instructions
	for _, instruction := range change.GetEndpointInstructions() {
		if err := rg.applyEndpointInstruction(router, instruction, targetVersion); err != nil {
			return fmt.Errorf("failed to apply endpoint instruction: %w", err)
		}
	}

	return nil
}

// applyEndpointInstruction applies an endpoint instruction to a router
func (rg *RouteGenerator) applyEndpointInstruction(router *VersionedRouter, instruction *EndpointInstruction, targetVersion *Version) error {
	switch instruction.Type {
	case "endpoint_added":
		// Remove the endpoint (reverse of addition)
		return router.DeleteRoute(
			getStringFromAttributes(instruction.Attributes, "method", "GET"),
			instruction.Path,
			targetVersion,
		)

	case "endpoint_removed":
		// Add the endpoint back (reverse of removal)
		handler := rg.createPlaceholderHandler(instruction.Path)
		router.RestoreRoute(
			getStringFromAttributes(instruction.Attributes, "method", "GET"),
			instruction.Path,
			targetVersion,
			handler,
		)

	case "endpoint_changed":
		// Apply the changes in reverse
		oldPath := getStringFromAttributes(instruction.Attributes, "old_path", instruction.Path)
		oldMethod := getStringFromAttributes(instruction.Attributes, "old_method", "GET")

		handler := rg.createPlaceholderHandler(oldPath)
		return router.ChangeRoute(
			getStringFromAttributes(instruction.Attributes, "method", "GET"),
			instruction.Path,
			oldMethod,
			oldPath,
			targetVersion,
			handler,
		)

	default:
		// Unknown instruction type, log and continue
		fmt.Printf("Warning: Unknown endpoint instruction type: %s\n", instruction.Type)
	}

	return nil
}

// createPlaceholderHandler creates a placeholder handler for restored endpoints
func (rg *RouteGenerator) createPlaceholderHandler(path string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message": fmt.Sprintf("Endpoint %s %s", c.Request.Method, path),
			"path":    path,
			"method":  c.Request.Method,
			"note":    "This is a generated handler for version-specific endpoint",
		})
	}
}

// Helper function to safely get string values from attributes
func getStringFromAttributes(attributes map[string]interface{}, key, defaultValue string) string {
	if attributes == nil {
		return defaultValue
	}

	if value, exists := attributes[key]; exists {
		if strValue, ok := value.(string); ok {
			return strValue
		}
	}

	return defaultValue
}

// RouteTransformer handles transformation of routes based on version changes
type RouteTransformer struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain
}

// NewRouteTransformer creates a new route transformer
func NewRouteTransformer(versionBundle *VersionBundle, migrationChain *MigrationChain) *RouteTransformer {
	return &RouteTransformer{
		versionBundle:  versionBundle,
		migrationChain: migrationChain,
	}
}

// TransformRoute transforms a route handler to be version-aware
func (rt *RouteTransformer) TransformRoute(handler gin.HandlerFunc, route *Route) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get version from context
		requestedVersion := getVersionFromGinContext(c)
		if requestedVersion == nil {
			// No version specified, use original handler
			handler(c)
			return
		}

		// Apply request migration
		err := rt.migrateGinRequest(c, requestedVersion)
		if err != nil {
			c.JSON(400, gin.H{"error": fmt.Sprintf("Request migration failed: %v", err)})
			return
		}

		// Apply response migration for Gin
		rt.handleWithMigration(c, handler, route)
	}
}

// migrateRequest migrates a request from the requested version to head version
func (rt *RouteTransformer) migrateRequest(r *http.Request, requestedVersion *Version) (*http.Request, error) {
	// This would implement actual request migration
	// For now, return the original request
	return r, nil
}

// handleWithMigration handles request/response with migration support
func (rt *RouteTransformer) handleWithMigration(c *gin.Context, handler gin.HandlerFunc, route *Route) {
	// Get the requested version from context
	requestedVersion := GetVersionFromContext(c)
	if requestedVersion == nil || requestedVersion.IsHead {
		// No migration needed for head version
		handler(c)
		return
	}

	// Create a response writer that captures the response
	responseCapture := &ResponseCapture{
		ResponseWriter: c.Writer,
		body:           make([]byte, 0),
		statusCode:     200,
	}
	c.Writer = responseCapture

	// Call the handler (which expects head version data)
	handler(c)

	// Migrate the captured response back to the requested version
	if err := rt.migrateResponse(c, requestedVersion, responseCapture); err != nil {
		c.JSON(500, gin.H{"error": "Response migration failed", "details": err.Error()})
		return
	}
}

// migrateResponse migrates response data from head version to requested version
func (rt *RouteTransformer) migrateResponse(c *gin.Context, toVersion *Version, responseCapture *ResponseCapture) error {
	// Parse captured response body
	var responseData interface{}
	if len(responseCapture.body) > 0 {
		if err := c.ShouldBindJSON(&responseData); err != nil {
			// If JSON parsing fails, write original response
			c.Writer = responseCapture.ResponseWriter
			c.Writer.WriteHeader(responseCapture.statusCode)
			c.Writer.Write(responseCapture.body)
			return nil
		}
	}

	// Create ResponseInfo for migration
	responseInfo := NewResponseInfo(c, responseData)
	responseInfo.StatusCode = responseCapture.statusCode

	// Find migration chain from head version back to requested version
	headVersion := rt.versionBundle.GetHeadVersion()
	migrationChain := rt.migrationChain.GetMigrationPath(headVersion, toVersion)

	// Apply migrations in reverse direction
	for i := len(migrationChain) - 1; i >= 0; i-- {
		change := migrationChain[i]
		if err := change.MigrateResponse(c.Request.Context(), responseInfo, reflect.TypeOf(responseInfo.Body), 0); err != nil {
			return fmt.Errorf("failed to migrate response with change %s: %w", change.Description(), err)
		}
	}

	// Write the migrated response
	c.Writer = responseCapture.ResponseWriter
	c.Writer.WriteHeader(responseInfo.StatusCode)

	if responseInfo.Body != nil {
		c.JSON(responseInfo.StatusCode, responseInfo.Body)
	}

	return nil
}

// ResponseInterceptor intercepts responses to apply version-specific transformations
type ResponseInterceptor struct {
	http.ResponseWriter
	requestedVersion *Version
	transformer      *RouteTransformer
	body             []byte
}

// Write intercepts the response body
func (ri *ResponseInterceptor) Write(data []byte) (int, error) {
	// Store the response body for migration
	ri.body = append(ri.body, data...)

	// For now, just write the original data
	// Migration will be handled by the RouteTransformer
	return ri.ResponseWriter.Write(data)
}

// migrateResponse migrates a response from head version to the requested version

// getVersionFromRequest extracts version from request (helper function)
func getVersionFromGinContext(c *gin.Context) *Version {
	// This would typically get version from context set by middleware
	// For now, return nil
	return nil
}

// migrateGinRequest migrates a Gin request from the requested version to head version
func (rt *RouteTransformer) migrateGinRequest(c *gin.Context, requestedVersion *Version) error {
	// This would implement actual request migration
	// For now, do nothing
	return nil
}

// migrateGinResponse migrates a Gin response from head version to the requested version
func (rt *RouteTransformer) migrateGinResponse(c *gin.Context, requestedVersion *Version) error {
	// This would implement actual response migration
	// For now, do nothing
	return nil
}

// EndpointGenerator generates endpoints based on schema changes
type EndpointGenerator struct {
	versionBundle *VersionBundle
	schemaTypes   map[reflect.Type]string
}

// NewEndpointGenerator creates a new endpoint generator
func NewEndpointGenerator(versionBundle *VersionBundle) *EndpointGenerator {
	return &EndpointGenerator{
		versionBundle: versionBundle,
		schemaTypes:   make(map[reflect.Type]string),
	}
}

// GenerateCRUDEndpoints generates CRUD endpoints for a given type
func (eg *EndpointGenerator) GenerateCRUDEndpoints(resourceType reflect.Type, basePath string) map[string]gin.HandlerFunc {
	handlers := make(map[string]gin.HandlerFunc)

	resourceName := resourceType.Name()

	// GET /resources
	handlers["GET "+basePath] = eg.createListHandler(resourceName)

	// GET /resources/{id}
	handlers["GET "+basePath+"/{id}"] = eg.createGetHandler(resourceName)

	// POST /resources
	handlers["POST "+basePath] = eg.createCreateHandler(resourceName)

	// PUT /resources/{id}
	handlers["PUT "+basePath+"/{id}"] = eg.createUpdateHandler(resourceName)

	// DELETE /resources/{id}
	handlers["DELETE "+basePath+"/{id}"] = eg.createDeleteHandler(resourceName)

	return handlers
}

// Helper methods to create CRUD handlers
func (eg *EndpointGenerator) createListHandler(resourceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message":  fmt.Sprintf("List %s", resourceName),
			"resource": resourceName,
		})
	}
}

func (eg *EndpointGenerator) createGetHandler(resourceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message":  fmt.Sprintf("Get %s", resourceName),
			"resource": resourceName,
		})
	}
}

func (eg *EndpointGenerator) createCreateHandler(resourceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(201, gin.H{
			"message":  fmt.Sprintf("Create %s", resourceName),
			"resource": resourceName,
		})
	}
}

func (eg *EndpointGenerator) createUpdateHandler(resourceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(200, gin.H{
			"message":  fmt.Sprintf("Update %s", resourceName),
			"resource": resourceName,
		})
	}
}

func (eg *EndpointGenerator) createDeleteHandler(resourceName string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Status(204)
	}
}
