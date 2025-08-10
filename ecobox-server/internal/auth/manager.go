package auth

import (
	"fmt"
	"net/http"
	"strings"
	
	"ecobox-server/internal/config"
	"github.com/sirupsen/logrus"
)

// Manager handles all authentication operations
type Manager struct {
	config     *config.Config
	userStore  *UserStore
	jwtManager *JWTManager
	logger     *logrus.Logger
}

// NewManager creates a new authentication manager
func NewManager(cfg *config.Config) *Manager {
	return &Manager{
		config:     cfg,
		userStore:  NewUserStore(cfg.Dashboard.PasswordFile),
		jwtManager: NewJWTManager(cfg.Dashboard.SessionKeyFile),
		logger:     logrus.New(),
	}
}

// Initialize sets up the authentication system
func (am *Manager) Initialize() error {
	// Initialize user store
	if err := am.userStore.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize user store: %w", err)
	}
	
	// Initialize JWT manager
	if err := am.jwtManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize JWT manager: %w", err)
	}
	
	am.logger.Info("Authentication system initialized")
	return nil
}

// AuthenticateRequest authenticates an HTTP request
func (am *Manager) AuthenticateRequest(r *http.Request) (*User, error) {
	// Check for Identity-Aware Proxy authentication first
	if am.config.Dashboard.IAPAuth != "none" {
		if user := am.checkIAPAuthentication(r); user != nil {
			return user, nil
		}
		
		// If IAP is configured but header is missing, fall through to standard auth
		am.logger.Debugf("IAP authentication failed, falling back to standard auth")
	}
	
	// Check for JWT token in cookie
	cookie, err := r.Cookie("auth_token")
	if err != nil {
		return nil, fmt.Errorf("authentication required")
	}
	
	claims, err := am.jwtManager.ValidateToken(cookie.Value)
	if err != nil {
		return nil, fmt.Errorf("invalid authentication token: %w", err)
	}
	
	// Get user from store to ensure it still exists
	user, exists := am.userStore.GetUser(claims.Username)
	if !exists {
		return nil, fmt.Errorf("user no longer exists")
	}
	
	return user, nil
}

// checkIAPAuthentication checks for Identity-Aware Proxy headers
func (am *Manager) checkIAPAuthentication(r *http.Request) *User {
	var username string
	
	switch AuthMethod(am.config.Dashboard.IAPAuth) {
	case AuthMethodTailscale:
		// Tailscale sets X-Forwarded-User header
		username = r.Header.Get("X-Forwarded-User")
	case AuthMethodAuthentik:
		// Authentik sets X-Forwarded-User header
		username = r.Header.Get("X-Forwarded-User")
	case AuthMethodCloudflare:
		// Cloudflare Access sets X-Forwarded-User header
		username = r.Header.Get("X-Forwarded-User")
	default:
		return nil
	}
	
	if username == "" {
		return nil
	}
	
	// Clean up username (remove domain part if present)
	if idx := strings.Index(username, "@"); idx != -1 {
		username = username[:idx]
	}
	
	// Get or create user
	user, exists := am.userStore.GetUser(username)
	if !exists {
		// Create user automatically for IAP
		am.logger.Infof("Creating new user for IAP authentication: %s", username)
		_, err := am.userStore.CreateUser(username, false)
		if err != nil {
			am.logger.Errorf("Failed to create IAP user %s: %v", username, err)
			return nil
		}
		
		// Set empty password for IAP users
		if err := am.userStore.SetPassword(username, ""); err != nil {
			am.logger.Errorf("Failed to set empty password for IAP user %s: %v", username, err)
			return nil
		}
		
		user, _ = am.userStore.GetUser(username)
	}
	
	return user
}

// Login authenticates a user with username/password
func (am *Manager) Login(username, password string) (string, *User, error) {
	user, err := am.userStore.AuthenticateUser(username, password)
	if err != nil {
		return "", nil, fmt.Errorf("authentication failed: %w", err)
	}
	
	// Generate JWT token
	token, err := am.jwtManager.GenerateToken(user)
	if err != nil {
		return "", nil, fmt.Errorf("failed to generate token: %w", err)
	}
	
	return token, user, nil
}

// CompleteFirstTimeSetup completes the first-time setup process
func (am *Manager) CompleteFirstTimeSetup(password string) error {
	return am.userStore.CompleteFirstTimeSetup(password)
}

// ListUsers returns all users (alias for GetAllUsers for consistency with API)
func (am *Manager) ListUsers() []User {
	users := am.userStore.GetAllUsers()
	result := make([]User, len(users))
	for i, user := range users {
		result[i] = *user
	}
	return result
}

// UpdateUser updates user information
func (am *Manager) UpdateUser(user *User) error {
	return am.userStore.UpdateUser(user)
}

// ChangePassword changes a user's password with optional current password requirement
func (am *Manager) ChangePassword(username, currentPassword, newPassword string, requireCurrentPassword bool) error {
	if requireCurrentPassword {
		// Verify current password
		if _, err := am.userStore.AuthenticateUser(username, currentPassword); err != nil {
			return fmt.Errorf("current password is incorrect")
		}
	}
	
	// Validate new password
	if err := ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("new password validation failed: %w", err)
	}
	
	// Set new password
	if err := am.userStore.SetPassword(username, newPassword); err != nil {
		return fmt.Errorf("failed to set new password: %w", err)
	}
	
	return nil
}

// CreateUser creates a new user and returns the user and initial password
func (am *Manager) CreateUser(username string, isAdmin bool) (*User, string, error) {
	initialPassword, err := am.userStore.CreateUser(username, isAdmin)
	if err != nil {
		return nil, "", err
	}
	
	user, exists := am.userStore.GetUser(username)
	if !exists {
		return nil, "", fmt.Errorf("failed to retrieve created user")
	}
	
	return user, initialPassword, nil
}

// DeleteUser removes a user
func (am *Manager) DeleteUser(username string) error {
	return am.userStore.DeleteUser(username)
}

// GetAllUsers returns all users
func (am *Manager) GetAllUsers() []*User {
	return am.userStore.GetAllUsers()
}

// GetUser returns a specific user
func (am *Manager) GetUser(username string) (*User, bool) {
	return am.userStore.GetUser(username)
}

// IsFirstTimeSetup checks if this is the first time setup
func (am *Manager) IsFirstTimeSetup() bool {
	// If IAP is enabled and not "none", first-time setup is not required
	if am.config.Dashboard.IAPAuth != "none" {
		return false
	}
	
	return am.userStore.IsFirstTimeSetup()
}

// RequiresAuthentication checks if authentication is required for the system
func (am *Manager) RequiresAuthentication() bool {
	// If IAP is enabled, authentication is handled by proxy
	if am.config.Dashboard.IAPAuth != "none" {
		return false // No login screen needed
	}
	
	// If no users have passwords set, authentication is not yet required
	return am.userStore.HasUsersWithPasswords()
}

// SetLogger sets a custom logger
func (am *Manager) SetLogger(logger *logrus.Logger) {
	am.logger = logger
}
