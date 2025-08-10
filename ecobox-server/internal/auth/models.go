package auth

import (
	"time"
)

// User represents a user in the system
type User struct {
	Username    string    `json:"username"`
	PasswordHash string   `json:"-"` // Never serialize password hash
	CreatedAt   time.Time `json:"created_at"`
	LastLogin   time.Time `json:"last_login"`
	IsAdmin     bool      `json:"is_admin"`
}

// Claims represents the JWT token claims
type Claims struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
	IssuedAt int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

// AuthMethod represents the authentication method being used
type AuthMethod string

const (
	AuthMethodNone       AuthMethod = "none"
	AuthMethodTailscale  AuthMethod = "tailscale" 
	AuthMethodAuthentik  AuthMethod = "authentik"
	AuthMethodCloudflare AuthMethod = "cloudflare"
)

// LoginRequest represents a login request
type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// PasswordChangeRequest represents a password change request
type PasswordChangeRequest struct {
	CurrentPassword string `json:"current_password,omitempty"`
	NewPassword     string `json:"new_password"`
	ConfirmPassword string `json:"confirm_password"`
}

// UserCreateRequest represents a user creation request
type UserCreateRequest struct {
	Username string `json:"username"`
	IsAdmin  bool   `json:"is_admin"`
}

// UserListResponse represents the response for listing users
type UserListResponse struct {
	Users []User `json:"users"`
}

// AuthResponse represents authentication response
type AuthResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Token   string `json:"token,omitempty"`
	User    *User  `json:"user,omitempty"`
}
