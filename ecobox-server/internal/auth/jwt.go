package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// JWTManager handles JWT token creation and validation
type JWTManager struct {
	sessionKeyFile string
	sessionKey     []byte
}

// NewJWTManager creates a new JWT manager
func NewJWTManager(sessionKeyFile string) *JWTManager {
	return &JWTManager{
		sessionKeyFile: sessionKeyFile,
	}
}

// Initialize sets up the JWT manager and ensures session key exists
func (jm *JWTManager) Initialize() error {
	// Check if session key file exists
	if _, err := os.Stat(jm.sessionKeyFile); os.IsNotExist(err) {
		// Generate new session key
		if err := jm.generateSessionKey(); err != nil {
			return fmt.Errorf("failed to generate session key: %w", err)
		}
	}
	
	// Load session key
	if err := jm.loadSessionKey(); err != nil {
		return fmt.Errorf("failed to load session key: %w", err)
	}
	
	return nil
}

// generateSessionKey generates a cryptographically secure session key
func (jm *JWTManager) generateSessionKey() error {
	// Generate 256-bit (32 bytes) random key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("failed to generate random key: %w", err)
	}
	
	// Encode to base64 for storage
	encoded := base64.StdEncoding.EncodeToString(key)
	
	// Write to file
	if err := os.WriteFile(jm.sessionKeyFile, []byte(encoded), 0600); err != nil {
		return fmt.Errorf("failed to write session key file: %w", err)
	}
	
	return nil
}

// loadSessionKey loads the session key from file
func (jm *JWTManager) loadSessionKey() error {
	data, err := os.ReadFile(jm.sessionKeyFile)
	if err != nil {
		return fmt.Errorf("failed to read session key file: %w", err)
	}
	
	// Decode from base64
	key, err := base64.StdEncoding.DecodeString(string(data))
	if err != nil {
		return fmt.Errorf("failed to decode session key: %w", err)
	}
	
	if len(key) < 32 {
		return fmt.Errorf("session key too short, must be at least 32 bytes")
	}
	
	jm.sessionKey = key
	return nil
}

// GenerateToken creates a JWT token for the given user
func (jm *JWTManager) GenerateToken(user *User) (string, error) {
	if jm.sessionKey == nil {
		return "", fmt.Errorf("session key not loaded")
	}
	
	// Token expires in 365 days
	expiresAt := time.Now().Add(365 * 24 * time.Hour).Unix()
	
	claims := Claims{
		Username:  user.Username,
		IsAdmin:   user.IsAdmin,
		IssuedAt:  time.Now().Unix(),
		ExpiresAt: expiresAt,
	}
	
	// Create JWT header
	header := map[string]interface{}{
		"alg": "HS256",
		"typ": "JWT",
	}
	
	// Encode header
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", fmt.Errorf("failed to marshal header: %w", err)
	}
	headerEncoded := base64.RawURLEncoding.EncodeToString(headerJSON)
	
	// Encode claims
	claimsJSON, err := json.Marshal(claims)
	if err != nil {
		return "", fmt.Errorf("failed to marshal claims: %w", err)
	}
	claimsEncoded := base64.RawURLEncoding.EncodeToString(claimsJSON)
	
	// Create signature
	message := headerEncoded + "." + claimsEncoded
	signature := jm.sign(message)
	signatureEncoded := base64.RawURLEncoding.EncodeToString(signature)
	
	// Combine all parts
	token := message + "." + signatureEncoded
	
	return token, nil
}

// ValidateToken validates a JWT token and returns the claims
func (jm *JWTManager) ValidateToken(tokenString string) (*Claims, error) {
	if jm.sessionKey == nil {
		return nil, fmt.Errorf("session key not loaded")
	}
	
	// Split token into parts
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("invalid token format")
	}
	
	headerEncoded := parts[0]
	claimsEncoded := parts[1]
	signatureEncoded := parts[2]
	
	// Verify signature
	message := headerEncoded + "." + claimsEncoded
	expectedSignature := jm.sign(message)
	
	signature, err := base64.RawURLEncoding.DecodeString(signatureEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode signature: %w", err)
	}
	
	if !hmac.Equal(signature, expectedSignature) {
		return nil, fmt.Errorf("invalid token signature")
	}
	
	// Decode and validate claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(claimsEncoded)
	if err != nil {
		return nil, fmt.Errorf("failed to decode claims: %w", err)
	}
	
	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("failed to unmarshal claims: %w", err)
	}
	
	// Check expiration
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("token expired")
	}
	
	return &claims, nil
}

// sign creates an HMAC signature for the given message
func (jm *JWTManager) sign(message string) []byte {
	h := hmac.New(sha256.New, jm.sessionKey)
	h.Write([]byte(message))
	return h.Sum(nil)
}

// RefreshToken creates a new token with extended expiration
func (jm *JWTManager) RefreshToken(oldTokenString string) (string, error) {
	claims, err := jm.ValidateToken(oldTokenString)
	if err != nil {
		return "", fmt.Errorf("cannot refresh invalid token: %w", err)
	}
	
	// Create new token with same claims but new expiration
	user := &User{
		Username: claims.Username,
		IsAdmin:  claims.IsAdmin,
	}
	
	return jm.GenerateToken(user)
}
