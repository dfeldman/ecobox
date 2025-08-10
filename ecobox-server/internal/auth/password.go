package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"math/big"
	"strings"
	
	"golang.org/x/crypto/bcrypt"
)

const (
	// BCrypt cost factor - high enough for security, not too slow for UX
	bcryptCost = 12
	
	// Password generation character sets
	upperChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowerChars = "abcdefghijklmnopqrstuvwxyz"
	digitChars = "0123456789"
	specialChars = "!@#$%^&*()_+-=[]{}|;:,.<>?"
)

// HashPassword creates a bcrypt hash of the password with salt and work factor
func HashPassword(password string) (string, error) {
	if password == "" {
		return "", fmt.Errorf("password cannot be empty")
	}
	
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	
	// Encode to base64 for storage
	encoded := base64.StdEncoding.EncodeToString(hash)
	return encoded, nil
}

// VerifyPassword verifies a password against its hash
func VerifyPassword(password, encodedHash string) error {
	if password == "" {
		return fmt.Errorf("password cannot be empty")
	}
	
	if encodedHash == "" {
		return fmt.Errorf("password hash cannot be empty")
	}
	
	// Decode from base64
	hash, err := base64.StdEncoding.DecodeString(encodedHash)
	if err != nil {
		return fmt.Errorf("failed to decode password hash: %w", err)
	}
	
	// Compare password with hash using constant-time comparison
	err = bcrypt.CompareHashAndPassword(hash, []byte(password))
	if err != nil {
		return fmt.Errorf("invalid password")
	}
	
	return nil
}

// GenerateStrongPassword generates a cryptographically secure random password
func GenerateStrongPassword(length int) (string, error) {
	if length < 8 {
		length = 16 // Default to 16 characters if too short
	}
	
	// Ensure we have at least one character from each category
	allChars := upperChars + lowerChars + digitChars + specialChars
	
	password := make([]byte, length)
	
	// First, add one character from each category
	categories := []string{upperChars, lowerChars, digitChars, specialChars}
	for i, chars := range categories {
		if i >= length {
			break
		}
		char, err := randomCharFromSet(chars)
		if err != nil {
			return "", fmt.Errorf("failed to generate password: %w", err)
		}
		password[i] = char
	}
	
	// Fill the rest with random characters from all categories
	for i := len(categories); i < length; i++ {
		char, err := randomCharFromSet(allChars)
		if err != nil {
			return "", fmt.Errorf("failed to generate password: %w", err)
		}
		password[i] = char
	}
	
	// Shuffle the password to avoid predictable patterns
	if err := shuffleBytes(password); err != nil {
		return "", fmt.Errorf("failed to shuffle password: %w", err)
	}
	
	return string(password), nil
}

// randomCharFromSet returns a random character from the given character set
func randomCharFromSet(chars string) (byte, error) {
	if len(chars) == 0 {
		return 0, fmt.Errorf("character set cannot be empty")
	}
	
	n, err := rand.Int(rand.Reader, big.NewInt(int64(len(chars))))
	if err != nil {
		return 0, err
	}
	
	return chars[n.Int64()], nil
}

// shuffleBytes randomly shuffles a byte slice in place
func shuffleBytes(data []byte) error {
	for i := len(data) - 1; i > 0; i-- {
		j, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return err
		}
		data[i], data[j.Int64()] = data[j.Int64()], data[i]
	}
	return nil
}

// ValidatePassword checks if a password meets minimum security requirements
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters long")
	}
	
	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false
	
	for _, char := range password {
		switch {
		case strings.ContainsRune(upperChars, char):
			hasUpper = true
		case strings.ContainsRune(lowerChars, char):
			hasLower = true
		case strings.ContainsRune(digitChars, char):
			hasDigit = true
		case strings.ContainsRune(specialChars, char):
			hasSpecial = true
		}
	}
	
	missing := []string{}
	if !hasUpper {
		missing = append(missing, "uppercase letter")
	}
	if !hasLower {
		missing = append(missing, "lowercase letter")
	}
	if !hasDigit {
		missing = append(missing, "digit")
	}
	if !hasSpecial {
		missing = append(missing, "special character")
	}
	
	if len(missing) > 0 {
		return fmt.Errorf("password must contain at least one: %s", strings.Join(missing, ", "))
	}
	
	return nil
}

// SecureCompare performs a constant-time comparison of two strings
func SecureCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
