package auth

import (
	"context"
	"net/http"
	"strings"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const (
	// UserContextKey is the key for storing user in request context
	UserContextKey contextKey = "user"
)

// Middleware provides authentication middleware for HTTP requests
type Middleware struct {
	authManager *Manager
}

// NewMiddleware creates a new authentication middleware
func NewMiddleware(authManager *Manager) *Middleware {
	return &Middleware{
		authManager: authManager,
	}
}

// RequireAuth is middleware that requires authentication for all routes
func (am *Middleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Check if this is first-time setup
		if am.authManager.IsFirstTimeSetup() && am.authManager.config.Dashboard.IAPAuth == "none" {
			// During first-time setup, only allow /setup and static assets
			if r.URL.Path == "/setup" || strings.HasPrefix(r.URL.Path, "/static/") || r.URL.Path == "/favicon.ico" {
				next.ServeHTTP(w, r)
				return
			}
			// All other paths redirect to setup
			http.Redirect(w, r, "/setup", http.StatusFound)
			return
		}
		
		// Skip authentication for certain paths (after setup is complete)
		if am.isPublicPath(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		
		// Check if authentication is required
		if !am.authManager.RequiresAuthentication() && am.authManager.config.Dashboard.IAPAuth == "none" {
			// No authentication required yet 
			next.ServeHTTP(w, r)
			return
		}
		
		// Authenticate the request
		user, err := am.authManager.AuthenticateRequest(r)
		if err != nil {
			am.handleAuthenticationError(w, r, err)
			return
		}
		
		// Add user to request context
		ctx := context.WithValue(r.Context(), UserContextKey, user)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequireAdmin is middleware that requires admin privileges
func (am *Middleware) RequireAdmin(next http.Handler) http.Handler {
	return am.RequireAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := GetUserFromContext(r.Context())
		if user == nil {
			http.Error(w, "Authentication required", http.StatusUnauthorized)
			return
		}
		
		if !user.IsAdmin {
			http.Error(w, "Admin privileges required", http.StatusForbidden)
			return
		}
		
		next.ServeHTTP(w, r)
	}))
}

// isPublicPath checks if a path should be accessible without authentication
func (am *Middleware) isPublicPath(path string) bool {
	publicPaths := []string{
		"/login",
		"/setup",
		"/static/",
		"/favicon.ico",
	}
	
	for _, publicPath := range publicPaths {
		if strings.HasPrefix(path, publicPath) {
			return true
		}
	}
	
	return false
}

// handleAuthenticationError handles authentication failures
func (am *Middleware) handleAuthenticationError(w http.ResponseWriter, r *http.Request, err error) {
	// For API requests, return JSON error
	if strings.HasPrefix(r.URL.Path, "/api/") {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"success": false, "message": "Authentication required"}`))
		return
	}
	
	// For web requests, redirect to login
	if am.authManager.RequiresAuthentication() {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}
	
	// For first-time setup
	if am.authManager.IsFirstTimeSetup() {
		http.Redirect(w, r, "/setup", http.StatusFound)
		return
	}
	
	// Default error response
	http.Error(w, "Authentication required", http.StatusUnauthorized)
}

// GetUserFromContext retrieves the user from request context
func GetUserFromContext(ctx context.Context) *User {
	user, ok := ctx.Value(UserContextKey).(*User)
	if !ok {
		return nil
	}
	return user
}

// SetAuthCookie sets the authentication cookie
func SetAuthCookie(w http.ResponseWriter, token string) {
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    token,
		Path:     "/",
		MaxAge:   365 * 24 * 60 * 60, // 365 days
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
	}
	
	http.SetCookie(w, cookie)
}

// ClearAuthCookie clears the authentication cookie
func ClearAuthCookie(w http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:     "auth_token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteStrictMode,
	}
	
	http.SetCookie(w, cookie)
}
