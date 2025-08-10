package auth

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

// UserStore manages user authentication data
type UserStore struct {
	passwordFile string
	users        map[string]*User
	mu           sync.RWMutex
}

// NewUserStore creates a new user store
func NewUserStore(passwordFile string) *UserStore {
	return &UserStore{
		passwordFile: passwordFile,
		users:        make(map[string]*User),
	}
}

// Initialize sets up the user store and handles first-time setup
func (us *UserStore) Initialize() error {
	// Check if password file exists
	if _, err := os.Stat(us.passwordFile); os.IsNotExist(err) {
		// Create initial password file with empty admin user
		if err := us.createInitialPasswordFile(); err != nil {
			return fmt.Errorf("failed to create initial password file: %w", err)
		}
	}
	
	// Load users from file
	if err := us.loadUsers(); err != nil {
		return fmt.Errorf("failed to load users: %w", err)
	}
	
	return nil
}

// createInitialPasswordFile creates the initial password file with admin user
func (us *UserStore) createInitialPasswordFile() error {
	file, err := os.Create(us.passwordFile)
	if err != nil {
		return fmt.Errorf("failed to create password file: %w", err)
	}
	defer file.Close()
	
	// Write initial admin user with empty password
	_, err = file.WriteString("admin:\n")
	if err != nil {
		return fmt.Errorf("failed to write initial admin user: %w", err)
	}
	
	return nil
}

// loadUsers loads users from the password file
func (us *UserStore) loadUsers() error {
	file, err := os.Open(us.passwordFile)
	if err != nil {
		return fmt.Errorf("failed to open password file: %w", err)
	}
	defer file.Close()
	
	us.mu.Lock()
	defer us.mu.Unlock()
	
	// Clear existing users
	us.users = make(map[string]*User)
	
	scanner := bufio.NewScanner(file)
	lineNum := 0
	
	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())
		
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid format in password file at line %d", lineNum)
		}
		
		username := strings.TrimSpace(parts[0])
		passwordHash := strings.TrimSpace(parts[1])
		
		if username == "" {
			return fmt.Errorf("empty username at line %d", lineNum)
		}
		
		user := &User{
			Username:     username,
			PasswordHash: passwordHash,
			CreatedAt:    time.Now(), // We don't store creation time in file, use current time
			IsAdmin:      username == "admin", // Admin user is always admin
		}
		
		us.users[username] = user
	}
	
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading password file: %w", err)
	}
	
	return nil
}

// saveUsers saves users to the password file
func (us *UserStore) saveUsers() error {
	us.mu.RLock()
	defer us.mu.RUnlock()
	return us.saveUsersLocked()
}

// saveUsersLocked saves users to the password file (assumes lock is already held)
func (us *UserStore) saveUsersLocked() error {
	// Create temporary file
	tmpFile := us.passwordFile + ".tmp"
	file, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temporary password file: %w", err)
	}
	
	// Write users to temporary file
	for _, user := range us.users {
		line := fmt.Sprintf("%s:%s\n", user.Username, user.PasswordHash)
		if _, err := file.WriteString(line); err != nil {
			file.Close()
			os.Remove(tmpFile)
			return fmt.Errorf("failed to write user data: %w", err)
		}
	}
	
	if err := file.Close(); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to close temporary file: %w", err)
	}
	
	// Atomically replace the original file
	if err := os.Rename(tmpFile, us.passwordFile); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to replace password file: %w", err)
	}
	
	return nil
}

// GetUser retrieves a user by username
func (us *UserStore) GetUser(username string) (*User, bool) {
	us.mu.RLock()
	defer us.mu.RUnlock()
	
	user, exists := us.users[username]
	if !exists {
		return nil, false
	}
	
	// Return a copy to prevent external modifications
	userCopy := *user
	return &userCopy, true
}

// GetAllUsers returns all users (without password hashes)
func (us *UserStore) GetAllUsers() []*User {
	us.mu.RLock()
	defer us.mu.RUnlock()
	
	users := make([]*User, 0, len(us.users))
	for _, user := range us.users {
		userCopy := *user
		userCopy.PasswordHash = "" // Never expose password hash
		users = append(users, &userCopy)
	}
	
	return users
}

// AuthenticateUser verifies username and password
func (us *UserStore) AuthenticateUser(username, password string) (*User, error) {
	user, exists := us.GetUser(username)
	if !exists {
		return nil, fmt.Errorf("user not found")
	}
	
	// Check if password hash is empty (first-time setup or IAP mode)
	if user.PasswordHash == "" {
		if password != "" {
			return nil, fmt.Errorf("invalid password")
		}
		// Allow login with empty password only if hash is also empty
		return user, nil
	}
	
	// Verify password
	if err := VerifyPassword(password, user.PasswordHash); err != nil {
		return nil, fmt.Errorf("invalid password")
	}
	
	// Update last login time
	us.updateLastLogin(username)
	
	return user, nil
}

// SetPassword sets a user's password
func (us *UserStore) SetPassword(username, password string) error {
	// Hash the password
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	
	us.mu.Lock()
	defer us.mu.Unlock()
	
	user, exists := us.users[username]
	if !exists {
		return fmt.Errorf("user not found")
	}
	
	user.PasswordHash = hashedPassword
	
	// Save to file
	return us.saveUsersLocked()
}

// CreateUser creates a new user
func (us *UserStore) CreateUser(username string, isAdmin bool) (string, error) {
	if username == "" {
		return "", fmt.Errorf("username cannot be empty")
	}
	
	us.mu.Lock()
	defer us.mu.Unlock()
	
	// Check if user already exists
	if _, exists := us.users[username]; exists {
		return "", fmt.Errorf("user already exists")
	}
	
	// Generate a strong initial password
	initialPassword, err := GenerateStrongPassword(16)
	if err != nil {
		return "", fmt.Errorf("failed to generate initial password: %w", err)
	}
	
	// Hash the password
	hashedPassword, err := HashPassword(initialPassword)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	
	// Create user
	user := &User{
		Username:     username,
		PasswordHash: hashedPassword,
		CreatedAt:    time.Now(),
		IsAdmin:      isAdmin,
	}
	
	us.users[username] = user
	
	// Save to file
	if err := us.saveUsersLocked(); err != nil {
		delete(us.users, username) // Rollback
		return "", fmt.Errorf("failed to save user: %w", err)
	}
	
	return initialPassword, nil
}

// DeleteUser removes a user
func (us *UserStore) DeleteUser(username string) error {
	if username == "admin" {
		return fmt.Errorf("cannot delete admin user")
	}
	
	us.mu.Lock()
	defer us.mu.Unlock()
	
	if _, exists := us.users[username]; !exists {
		return fmt.Errorf("user not found")
	}
	
	delete(us.users, username)
	
	// Save to file
	return us.saveUsersLocked()
}

// HasUsersWithPasswords checks if any user has a password set
func (us *UserStore) HasUsersWithPasswords() bool {
	us.mu.RLock()
	defer us.mu.RUnlock()
	
	for _, user := range us.users {
		if user.PasswordHash != "" {
			return true
		}
	}
	
	return false
}

// IsFirstTimeSetup checks if this is the first time setup (admin user with no password)
func (us *UserStore) IsFirstTimeSetup() bool {
	us.mu.RLock()
	defer us.mu.RUnlock()
	
	admin, exists := us.users["admin"]
	if !exists {
		return true // No admin user means first time setup
	}
	
	return admin.PasswordHash == ""
}

// CompleteFirstTimeSetup sets the admin password for first-time setup
func (us *UserStore) CompleteFirstTimeSetup(password string) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	
	admin, exists := us.users["admin"]
	if !exists {
		return fmt.Errorf("admin user does not exist")
	}
	
	if admin.PasswordHash != "" {
		return fmt.Errorf("first-time setup already completed")
	}
	
	// Set password for admin user
	hashedPassword, err := HashPassword(password)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}
	
	admin.PasswordHash = hashedPassword
	
	return us.saveUsersLocked()
}

// UpdateUser updates user information
func (us *UserStore) UpdateUser(user *User) error {
	us.mu.Lock()
	defer us.mu.Unlock()
	
	if _, exists := us.users[user.Username]; !exists {
		return fmt.Errorf("user does not exist: %s", user.Username)
	}
	
	us.users[user.Username] = user
	
	// For non-password updates, we can skip saving to disk
	// The password hash and created time are the only persistent fields
	return nil
}

// updateLastLogin updates the last login time for a user
func (us *UserStore) updateLastLogin(username string) {
	us.mu.Lock()
	defer us.mu.Unlock()
	
	if user, exists := us.users[username]; exists {
		user.LastLogin = time.Now()
		// Note: We don't save to file for performance reasons
		// Last login time is not persisted across restarts
	}
}
