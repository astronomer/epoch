package cadwyn

import (
	"fmt"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"
)

// VersionedRouter handles Gin routing with version-aware capabilities
// This is inspired by Python Cadwyn's routing.py
type VersionedRouter struct {
	versionBundle  *VersionBundle
	migrationChain *MigrationChain

	// Version-specific Gin engines
	versionedRouters map[string]*gin.Engine
	unversionedGin   *gin.Engine

	// Configuration
	apiVersionParameterName string
	redirectSlashes         bool

	// Route tracking
	routes map[string]*Route
}

// Route represents a version-aware Gin route
type Route struct {
	Pattern     string
	Method      string
	Handler     gin.HandlerFunc
	Versions    []*Version
	IsDeleted   bool
	Middlewares []gin.HandlerFunc
}

// RouterConfig holds configuration for the versioned router
type RouterConfig struct {
	VersionBundle           *VersionBundle
	MigrationChain          *MigrationChain
	APIVersionParameterName string
	RedirectSlashes         bool
}

// NewVersionedRouter creates a new version-aware router
func NewVersionedRouter(config RouterConfig) *VersionedRouter {
	if config.APIVersionParameterName == "" {
		config.APIVersionParameterName = "X-API-Version"
	}

	router := &VersionedRouter{
		versionBundle:           config.VersionBundle,
		migrationChain:          config.MigrationChain,
		versionedRouters:        make(map[string]*gin.Engine),
		unversionedGin:          gin.New(),
		apiVersionParameterName: config.APIVersionParameterName,
		redirectSlashes:         config.RedirectSlashes,
		routes:                  make(map[string]*Route),
	}

	// Initialize version-specific routers
	router.initializeVersionRouters()

	return router
}

// initializeVersionRouters creates Gin engines for each version
func (vr *VersionedRouter) initializeVersionRouters() {
	// Create router for head version
	vr.versionedRouters["head"] = gin.New()

	// Create routers for each version
	for _, v := range vr.versionBundle.GetVersions() {
		vr.versionedRouters[v.String()] = gin.New()
	}
}

// GET registers a GET handler for specific versions
func (vr *VersionedRouter) GET(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	vr.Handle("GET", pattern, handler, versions...)
}

// POST registers a POST handler for specific versions
func (vr *VersionedRouter) POST(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	vr.Handle("POST", pattern, handler, versions...)
}

// PUT registers a PUT handler for specific versions
func (vr *VersionedRouter) PUT(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	vr.Handle("PUT", pattern, handler, versions...)
}

// DELETE registers a DELETE handler for specific versions
func (vr *VersionedRouter) DELETE(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	vr.Handle("DELETE", pattern, handler, versions...)
}

// PATCH registers a PATCH handler for specific versions
func (vr *VersionedRouter) PATCH(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	vr.Handle("PATCH", pattern, handler, versions...)
}

// Handle registers a handler for specific versions
func (vr *VersionedRouter) Handle(method, pattern string, handler gin.HandlerFunc, versions ...*Version) {
	route := &Route{
		Pattern:  pattern,
		Method:   method,
		Handler:  handler,
		Versions: versions,
	}

	routeKey := fmt.Sprintf("%s %s", method, pattern)
	vr.routes[routeKey] = route

	// If no versions specified, register for all versions
	if len(versions) == 0 {
		versions = append([]*Version{vr.versionBundle.GetHeadVersion()}, vr.versionBundle.GetVersions()...)
	}

	// Register with version-specific routers
	for _, v := range versions {
		if engine, exists := vr.versionedRouters[v.String()]; exists {
			switch method {
			case "GET":
				engine.GET(pattern, handler)
			case "POST":
				engine.POST(pattern, handler)
			case "PUT":
				engine.PUT(pattern, handler)
			case "DELETE":
				engine.DELETE(pattern, handler)
			case "PATCH":
				engine.PATCH(pattern, handler)
			case "HEAD":
				engine.HEAD(pattern, handler)
			case "OPTIONS":
				engine.OPTIONS(pattern, handler)
			default:
				engine.Any(pattern, handler)
			}
		}
	}
}

// HandleUnversioned registers a handler that doesn't participate in versioning
func (vr *VersionedRouter) HandleUnversioned(method, pattern string, handler gin.HandlerFunc) {
	switch method {
	case "GET":
		vr.unversionedGin.GET(pattern, handler)
	case "POST":
		vr.unversionedGin.POST(pattern, handler)
	case "PUT":
		vr.unversionedGin.PUT(pattern, handler)
	case "DELETE":
		vr.unversionedGin.DELETE(pattern, handler)
	case "PATCH":
		vr.unversionedGin.PATCH(pattern, handler)
	case "HEAD":
		vr.unversionedGin.HEAD(pattern, handler)
	case "OPTIONS":
		vr.unversionedGin.OPTIONS(pattern, handler)
	default:
		vr.unversionedGin.Any(pattern, handler)
	}
}

// GetEngineForVersion returns the appropriate Gin engine for a version
func (vr *VersionedRouter) GetEngineForVersion(requestedVersion *Version) *gin.Engine {
	if requestedVersion == nil {
		return vr.unversionedGin
	}

	versionStr := requestedVersion.String()
	if engine, exists := vr.versionedRouters[versionStr]; exists {
		return engine
	}

	// Try to find closest older version (waterfall logic)
	closestVersion := vr.findClosestOlderVersion(requestedVersion)
	if closestVersion != nil {
		return vr.versionedRouters[closestVersion.String()]
	}

	// Fallback to unversioned
	return vr.unversionedGin
}

// GetEngine returns the unversioned Gin engine
func (vr *VersionedRouter) GetEngine() *gin.Engine {
	return vr.unversionedGin
}

// findClosestOlderVersion finds the closest older version for waterfall routing
func (vr *VersionedRouter) findClosestOlderVersion(requestedVersion *Version) *Version {
	var closestVersion *Version

	for _, v := range vr.versionBundle.GetVersions() {
		if v.IsOlderThan(requestedVersion) {
			if closestVersion == nil || v.IsNewerThan(closestVersion) {
				closestVersion = v
			}
		}
	}

	return closestVersion
}

// GetVersions returns sorted list of available versions
func (vr *VersionedRouter) GetVersions() []string {
	versions := make([]string, 0, len(vr.versionedRouters))
	for versionStr := range vr.versionedRouters {
		if versionStr != "head" {
			versions = append(versions, versionStr)
		}
	}

	sort.Strings(versions)
	return versions
}

// GetRoutes returns all registered routes
func (vr *VersionedRouter) GetRoutes() map[string]*Route {
	return vr.routes
}

// RouteExists checks if a route exists for a specific version
func (vr *VersionedRouter) RouteExists(method, pattern string, targetVersion *Version) bool {
	routeKey := fmt.Sprintf("%s %s", method, pattern)
	route, exists := vr.routes[routeKey]
	if !exists {
		return false
	}

	if route.IsDeleted {
		return false
	}

	// Check if route is available in the target version
	for _, v := range route.Versions {
		if v.Equal(targetVersion) {
			return true
		}
	}

	return false
}

// DeleteRoute marks a route as deleted for specific versions
// This is useful for version changes that remove endpoints
func (vr *VersionedRouter) DeleteRoute(method, pattern string, fromVersion *Version) error {
	routeKey := fmt.Sprintf("%s %s", method, pattern)
	route, exists := vr.routes[routeKey]
	if !exists {
		return fmt.Errorf("route %s does not exist", routeKey)
	}

	// Remove the version from the route's available versions
	newVersions := make([]*Version, 0, len(route.Versions))
	for _, v := range route.Versions {
		if !v.Equal(fromVersion) && !v.IsOlderThan(fromVersion) {
			newVersions = append(newVersions, v)
		}
	}

	route.Versions = newVersions

	// If no versions left, mark as deleted
	if len(newVersions) == 0 {
		route.IsDeleted = true
	}

	return nil
}

// RestoreRoute restores a deleted route for specific versions
func (vr *VersionedRouter) RestoreRoute(method, pattern string, forVersion *Version, handler gin.HandlerFunc) {
	routeKey := fmt.Sprintf("%s %s", method, pattern)
	route, exists := vr.routes[routeKey]

	if !exists {
		// Create new route
		route = &Route{
			Pattern:  pattern,
			Method:   method,
			Handler:  handler,
			Versions: []*Version{forVersion},
		}
		vr.routes[routeKey] = route
	} else {
		// Add version to existing route
		route.Versions = append(route.Versions, forVersion)
		route.IsDeleted = false
		route.Handler = handler
	}

	// Register with version-specific router
	if engine, exists := vr.versionedRouters[forVersion.String()]; exists {
		switch method {
		case "GET":
			engine.GET(pattern, handler)
		case "POST":
			engine.POST(pattern, handler)
		case "PUT":
			engine.PUT(pattern, handler)
		case "DELETE":
			engine.DELETE(pattern, handler)
		case "PATCH":
			engine.PATCH(pattern, handler)
		case "HEAD":
			engine.HEAD(pattern, handler)
		case "OPTIONS":
			engine.OPTIONS(pattern, handler)
		default:
			engine.Any(pattern, handler)
		}
	}
}

// ChangeRoute modifies a route's properties for specific versions
func (vr *VersionedRouter) ChangeRoute(oldMethod, oldPattern, newMethod, newPattern string, fromVersion *Version, handler gin.HandlerFunc) error {
	// Delete old route
	if err := vr.DeleteRoute(oldMethod, oldPattern, fromVersion); err != nil {
		return fmt.Errorf("failed to delete old route: %w", err)
	}

	// Add new route
	vr.RestoreRoute(newMethod, newPattern, fromVersion, handler)

	return nil
}

// PrintRoutes prints all registered routes (for debugging)
func (vr *VersionedRouter) PrintRoutes() {
	fmt.Println("🛣️  Registered Routes:")
	fmt.Println(strings.Repeat("-", 60))

	for routeKey, route := range vr.routes {
		status := "✅"
		if route.IsDeleted {
			status = "❌"
		}

		versions := make([]string, len(route.Versions))
		for i, v := range route.Versions {
			versions[i] = v.String()
		}

		fmt.Printf("%s %s -> Versions: [%s]\n",
			status, routeKey, strings.Join(versions, ", "))
	}

	fmt.Println(strings.Repeat("-", 60))
}

// RouteInfo provides information about a route
type RouteInfo struct {
	Pattern     string
	Method      string
	Versions    []string
	IsDeleted   bool
	IsVersioned bool
}

// GetRouteInfo returns detailed information about all routes
func (vr *VersionedRouter) GetRouteInfo() []RouteInfo {
	var info []RouteInfo

	for _, route := range vr.routes {
		versions := make([]string, len(route.Versions))
		for i, v := range route.Versions {
			versions[i] = v.String()
		}

		routeInfo := RouteInfo{
			Pattern:     route.Pattern,
			Method:      route.Method,
			Versions:    versions,
			IsDeleted:   route.IsDeleted,
			IsVersioned: len(route.Versions) > 0,
		}

		info = append(info, routeInfo)
	}

	return info
}

// Group creates a route group with a common prefix
type RouteGroup struct {
	router     *VersionedRouter
	prefix     string
	middleware []gin.HandlerFunc
}

// Group creates a new route group with a common prefix
func (vr *VersionedRouter) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		router: vr,
		prefix: strings.TrimSuffix(prefix, "/"),
	}
}

// Use adds middleware to the route group
func (rg *RouteGroup) Use(middleware gin.HandlerFunc) {
	rg.middleware = append(rg.middleware, middleware)
}

// GET registers a GET handler with the group prefix
func (rg *RouteGroup) GET(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	rg.Handle("GET", pattern, handler, versions...)
}

// POST registers a POST handler with the group prefix
func (rg *RouteGroup) POST(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	rg.Handle("POST", pattern, handler, versions...)
}

// PUT registers a PUT handler with the group prefix
func (rg *RouteGroup) PUT(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	rg.Handle("PUT", pattern, handler, versions...)
}

// DELETE registers a DELETE handler with the group prefix
func (rg *RouteGroup) DELETE(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	rg.Handle("DELETE", pattern, handler, versions...)
}

// PATCH registers a PATCH handler with the group prefix
func (rg *RouteGroup) PATCH(pattern string, handler gin.HandlerFunc, versions ...*Version) {
	rg.Handle("PATCH", pattern, handler, versions...)
}

// Handle registers a handler with method and group prefix
func (rg *RouteGroup) Handle(method, pattern string, handler gin.HandlerFunc, versions ...*Version) {
	fullPattern := rg.prefix + pattern

	// Apply group middleware by wrapping the handler
	finalHandler := handler
	for i := len(rg.middleware) - 1; i >= 0; i-- {
		middleware := rg.middleware[i]
		prevHandler := finalHandler
		finalHandler = func(c *gin.Context) {
			middleware(c)
			if !c.IsAborted() {
				prevHandler(c)
			}
		}
	}

	rg.router.Handle(method, fullPattern, finalHandler, versions...)
}
