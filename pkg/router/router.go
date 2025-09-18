package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/isaacchung/cadwyn-go/pkg/migration"
	"github.com/isaacchung/cadwyn-go/pkg/version"
)

// VersionedRouter handles HTTP routing with automatic version support
type VersionedRouter struct {
	versionBundle   *version.VersionBundle
	migrationChain  *migration.MigrationChain
	routes          map[string]*Route
	mux             *http.ServeMux
	registeredPaths map[string]bool
}

// Route represents a versioned HTTP route
type Route struct {
	Pattern     string
	Method      string
	Handler     http.HandlerFunc
	Versions    []*version.Version
	Middlewares []func(http.Handler) http.Handler
}

// Config holds configuration for the versioned router
type Config struct {
	VersionBundle  *version.VersionBundle
	MigrationChain *migration.MigrationChain
}

// NewVersionedRouter creates a new versioned router
func NewVersionedRouter(config Config) *VersionedRouter {
	return &VersionedRouter{
		versionBundle:   config.VersionBundle,
		migrationChain:  config.MigrationChain,
		routes:          make(map[string]*Route),
		mux:             http.NewServeMux(),
		registeredPaths: make(map[string]bool),
	}
}

// Handle registers a handler for a specific HTTP method and pattern
func (vr *VersionedRouter) Handle(method, pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	routeKey := fmt.Sprintf("%s %s", method, pattern)

	// If no versions specified, use head version
	if len(versions) == 0 {
		versions = []*version.Version{vr.versionBundle.GetHeadVersion()}
	}

	// Check if we already have a route (perhaps from middleware registration)
	route, exists := vr.routes[routeKey]
	if !exists {
		route = &Route{
			Pattern:     pattern,
			Method:      method,
			Middlewares: []func(http.Handler) http.Handler{},
		}
	}

	// Update the route with handler and versions
	route.Handler = handler
	route.Versions = versions

	vr.routes[routeKey] = route

	// Check if we already have a handler for this pattern
	// If not, register the method router
	if !vr.registeredPaths[pattern] {
		vr.mux.HandleFunc(pattern, vr.createMethodRouter(pattern))
		vr.registeredPaths[pattern] = true
	}
}

// GET registers a GET handler
func (vr *VersionedRouter) GET(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	vr.Handle("GET", pattern, handler, versions...)
}

// POST registers a POST handler
func (vr *VersionedRouter) POST(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	vr.Handle("POST", pattern, handler, versions...)
}

// PUT registers a PUT handler
func (vr *VersionedRouter) PUT(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	vr.Handle("PUT", pattern, handler, versions...)
}

// DELETE registers a DELETE handler
func (vr *VersionedRouter) DELETE(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	vr.Handle("DELETE", pattern, handler, versions...)
}

// PATCH registers a PATCH handler
func (vr *VersionedRouter) PATCH(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	vr.Handle("PATCH", pattern, handler, versions...)
}

// Use adds middleware to a specific route
func (vr *VersionedRouter) Use(method, pattern string, middleware func(http.Handler) http.Handler) {
	routeKey := fmt.Sprintf("%s %s", method, pattern)

	route, exists := vr.routes[routeKey]
	if !exists {
		// Create a placeholder route
		route = &Route{
			Pattern:     pattern,
			Method:      method,
			Middlewares: []func(http.Handler) http.Handler{},
		}
		vr.routes[routeKey] = route
	}

	route.Middlewares = append(route.Middlewares, middleware)
}

// createMethodRouter creates a router that handles multiple HTTP methods for the same pattern
func (vr *VersionedRouter) createMethodRouter(pattern string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		routeKey := fmt.Sprintf("%s %s", r.Method, pattern)
		route, exists := vr.routes[routeKey]

		if !exists {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Apply middlewares
		handler := http.Handler(route.Handler)
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// The version middleware should have already set the version in context
		// and handled request/response migration, so we just call the handler
		handler.ServeHTTP(w, r)
	}
}

// createVersionedHandler creates a handler that applies version-aware logic (legacy method)
func (vr *VersionedRouter) createVersionedHandler(route *Route) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Only handle the specified HTTP method
		if r.Method != route.Method {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// Apply middlewares
		handler := http.Handler(route.Handler)
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		// The version middleware should have already set the version in context
		// and handled request/response migration, so we just call the handler
		handler.ServeHTTP(w, r)
	}
}

// ServeHTTP implements http.Handler
func (vr *VersionedRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	vr.mux.ServeHTTP(w, r)
}

// GenerateVersionedRoutes generates additional routes for specific versions
func (vr *VersionedRouter) GenerateVersionedRoutes() error {
	for _, route := range vr.routes {
		for _, v := range vr.versionBundle.GetVersions() {
			if !vr.routeSupportsVersion(route, v) {
				continue
			}

			// Generate version-specific path
			versionedPath := vr.generateVersionedPath(route.Pattern, v)
			if versionedPath == route.Pattern {
				continue // Skip if same as original
			}

			// Register versioned route
			vr.mux.HandleFunc(versionedPath, vr.createVersionSpecificHandler(route, v))
		}
	}

	return nil
}

// routeSupportsVersion checks if a route supports a specific version
func (vr *VersionedRouter) routeSupportsVersion(route *Route, v *version.Version) bool {
	for _, supportedVersion := range route.Versions {
		if supportedVersion.Equal(v) {
			return true
		}
	}
	return false
}

// generateVersionedPath generates a version-specific path
func (vr *VersionedRouter) generateVersionedPath(pattern string, v *version.Version) string {
	if v.IsHead {
		return pattern
	}

	// Add version prefix
	return fmt.Sprintf("/v%s%s", v.String(), pattern)
}

// createVersionSpecificHandler creates a handler for a specific version
func (vr *VersionedRouter) createVersionSpecificHandler(route *Route, v *version.Version) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Add version to context
		ctx := context.WithValue(r.Context(), "cadwyn_version", v)
		r = r.WithContext(ctx)

		// Apply middlewares
		handler := http.Handler(route.Handler)
		for i := len(route.Middlewares) - 1; i >= 0; i-- {
			handler = route.Middlewares[i](handler)
		}

		handler.ServeHTTP(w, r)
	}
}

// GetRoutes returns all registered routes
func (vr *VersionedRouter) GetRoutes() map[string]*Route {
	return vr.routes
}

// RouteGroup provides a way to group routes with common middleware
type RouteGroup struct {
	router      *VersionedRouter
	prefix      string
	middlewares []func(http.Handler) http.Handler
}

// Group creates a new route group with a common prefix
func (vr *VersionedRouter) Group(prefix string) *RouteGroup {
	return &RouteGroup{
		router: vr,
		prefix: strings.TrimSuffix(prefix, "/"),
	}
}

// Use adds middleware to all routes in this group
func (rg *RouteGroup) Use(middleware func(http.Handler) http.Handler) {
	rg.middlewares = append(rg.middlewares, middleware)
}

// Handle registers a handler in this group
func (rg *RouteGroup) Handle(method, pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	fullPattern := rg.prefix + pattern

	// Wrap handler with group middlewares
	finalHandler := handler
	for i := len(rg.middlewares) - 1; i >= 0; i-- {
		finalHandler = rg.wrapWithMiddleware(finalHandler, rg.middlewares[i])
	}

	rg.router.Handle(method, fullPattern, finalHandler, versions...)
}

// wrapWithMiddleware wraps a handler with middleware
func (rg *RouteGroup) wrapWithMiddleware(handler http.HandlerFunc, middleware func(http.Handler) http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		middleware(handler).ServeHTTP(w, r)
	}
}

// GET registers a GET handler in this group
func (rg *RouteGroup) GET(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	rg.Handle("GET", pattern, handler, versions...)
}

// POST registers a POST handler in this group
func (rg *RouteGroup) POST(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	rg.Handle("POST", pattern, handler, versions...)
}

// PUT registers a PUT handler in this group
func (rg *RouteGroup) PUT(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	rg.Handle("PUT", pattern, handler, versions...)
}

// DELETE registers a DELETE handler in this group
func (rg *RouteGroup) DELETE(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	rg.Handle("DELETE", pattern, handler, versions...)
}

// PATCH registers a PATCH handler in this group
func (rg *RouteGroup) PATCH(pattern string, handler http.HandlerFunc, versions ...*version.Version) {
	rg.Handle("PATCH", pattern, handler, versions...)
}

// VersionAwareHandler is a handler that can access version information
type VersionAwareHandler func(w http.ResponseWriter, r *http.Request, v *version.Version)

// WrapVersionAware wraps a version-aware handler to be used with the router
func WrapVersionAware(handler VersionAwareHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract version from context
		var v *version.Version
		if ctxVersion, ok := r.Context().Value("cadwyn_version").(*version.Version); ok {
			v = ctxVersion
		}

		handler(w, r, v)
	}
}

// RouteInfo provides information about registered routes
type RouteInfo struct {
	Pattern     string
	Method      string
	Versions    []string
	HasVersions bool
}

// GetRouteInfo returns information about all registered routes
func (vr *VersionedRouter) GetRouteInfo() []RouteInfo {
	var info []RouteInfo

	for _, route := range vr.routes {
		versionStrings := make([]string, len(route.Versions))
		for i, v := range route.Versions {
			versionStrings[i] = v.String()
		}

		info = append(info, RouteInfo{
			Pattern:     route.Pattern,
			Method:      route.Method,
			Versions:    versionStrings,
			HasVersions: len(route.Versions) > 0,
		})
	}

	return info
}

// PrintRoutes prints all registered routes (useful for debugging)
func (vr *VersionedRouter) PrintRoutes() {
	fmt.Println("Registered routes:")
	for routeKey, route := range vr.routes {
		versions := make([]string, len(route.Versions))
		for i, v := range route.Versions {
			versions[i] = v.String()
		}
		fmt.Printf("  %s -> versions: %v\n", routeKey, versions)
	}
}
