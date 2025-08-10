package web

import (
	"encoding/json"
	"net/http"
	"text/template"
	"time"

	"ecobox-server/internal/auth"
	"github.com/gorilla/mux"
)

// handleLogin handles login page and login requests
func (ws *WebServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		ws.renderLoginPage(w, r, "")
		return
	}

	// Handle POST request (login form submission)
	if err := r.ParseForm(); err != nil {
		ws.renderLoginPage(w, r, "Invalid form data")
		return
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	if username == "" || password == "" {
		ws.renderLoginPage(w, r, "Username and password are required")
		return
	}

	// Authenticate user
	token, user, err := ws.authManager.Login(username, password)
	if err != nil {
		ws.logger.Warnf("Login failed for user %s: %v", username, err)
		ws.renderLoginPage(w, r, "Invalid username or password")
		return
	}

	// Set authentication cookie
	auth.SetAuthCookie(w, token)

	// Update last login time
	user.LastLogin = time.Now()
	if err := ws.authManager.UpdateUser(user); err != nil {
		ws.logger.Errorf("Failed to update last login time: %v", err)
	}

	ws.logger.Infof("User %s logged in successfully", username)

	// Redirect to main page
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleLogout handles logout requests
func (ws *WebServer) handleLogout(w http.ResponseWriter, r *http.Request) {
	// Clear authentication cookie
	auth.ClearAuthCookie(w)

	// For API requests, return JSON
	if r.Header.Get("Accept") == "application/json" {
		ws.writeJSONResponse(w, http.StatusOK, APIResponse{
			Success: true,
			Message: "Logged out successfully",
		})
		return
	}

	// Redirect to login page
	http.Redirect(w, r, "/login", http.StatusFound)
}

// handleSetup handles first-time setup
func (ws *WebServer) handleSetup(w http.ResponseWriter, r *http.Request) {
	// Check if setup is needed
	if !ws.authManager.IsFirstTimeSetup() {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	if r.Method == "GET" {
		ws.renderSetupPage(w, r, "")
		return
	}

	// Handle POST request (setup form submission)
	if err := r.ParseForm(); err != nil {
		ws.renderSetupPage(w, r, "Invalid form data")
		return
	}

	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	if password == "" {
		ws.renderSetupPage(w, r, "Password is required")
		return
	}

	if password != confirmPassword {
		ws.renderSetupPage(w, r, "Passwords do not match")
		return
	}

	// Complete first-time setup
	if err := ws.authManager.CompleteFirstTimeSetup(password); err != nil {
		ws.logger.Errorf("First-time setup failed: %v", err)
		ws.renderSetupPage(w, r, "Setup failed. Please try again.")
		return
	}

	ws.logger.Info("First-time setup completed successfully")

	// Redirect to login page
	http.Redirect(w, r, "/login?message=Setup completed successfully. Please log in.", http.StatusFound)
}

// handleGetCurrentUser returns the current authenticated user
func (ws *WebServer) handleGetCurrentUser(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		ws.writeJSONResponse(w, http.StatusUnauthorized, APIResponse{
			Success: false,
			Message: "Authentication required",
		})
		return
	}

	ws.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    user,
	})
}

// handleChangePassword handles password change requests
func (ws *WebServer) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		ws.writeJSONResponse(w, http.StatusUnauthorized, APIResponse{
			Success: false,
			Message: "Authentication required",
		})
		return
	}

	var req auth.PasswordChangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ws.writeJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	// For IAP users, current password is not required
	requireCurrentPassword := ws.config.Dashboard.IAPAuth == "none"

	if err := ws.authManager.ChangePassword(user.Username, req.CurrentPassword, req.NewPassword, requireCurrentPassword); err != nil {
		ws.writeJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	ws.logger.Infof("Password changed successfully for user %s", user.Username)

	ws.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "Password changed successfully",
	})
}

// handleGetUsers returns a list of all users (admin only)
func (ws *WebServer) handleGetUsers(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil || user.Username != "admin" {
		ws.writeJSONResponse(w, http.StatusForbidden, APIResponse{
			Success: false,
			Message: "Admin privileges required",
		})
		return
	}

	users := ws.authManager.ListUsers()
	
	ws.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Data: auth.UserListResponse{
			Users: users,
		},
	})
}

// handleCreateUser creates a new user (admin only)
func (ws *WebServer) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil || user.Username != "admin" {
		ws.writeJSONResponse(w, http.StatusForbidden, APIResponse{
			Success: false,
			Message: "Admin privileges required",
		})
		return
	}

	var req auth.UserCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		ws.writeJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Invalid request body",
		})
		return
	}

	newUser, password, err := ws.authManager.CreateUser(req.Username, req.IsAdmin)
	if err != nil {
		ws.writeJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	ws.logger.Infof("User %s created successfully by %s", req.Username, user.Username)

	ws.writeJSONResponse(w, http.StatusCreated, APIResponse{
		Success: true,
		Message: "User created successfully",
		Data: map[string]interface{}{
			"user":            newUser,
			"initial_password": password,
		},
	})
}

// handleDeleteUser deletes a user (admin only)
func (ws *WebServer) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	currentUser := auth.GetUserFromContext(r.Context())
	if currentUser == nil || currentUser.Username != "admin" {
		ws.writeJSONResponse(w, http.StatusForbidden, APIResponse{
			Success: false,
			Message: "Admin privileges required",
		})
		return
	}

	vars := mux.Vars(r)
	username := vars["username"]

	if username == "" {
		ws.writeJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Username is required",
		})
		return
	}

	// Prevent deleting self
	if username == currentUser.Username {
		ws.writeJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: "Cannot delete your own account",
		})
		return
	}

	if err := ws.authManager.DeleteUser(username); err != nil {
		ws.writeJSONResponse(w, http.StatusBadRequest, APIResponse{
			Success: false,
			Message: err.Error(),
		})
		return
	}

	ws.logger.Infof("User %s deleted successfully by %s", username, currentUser.Username)

	ws.writeJSONResponse(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "User deleted successfully",
	})
}

// handleUsersPage renders the user management page
func (ws *WebServer) handleUsersPage(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil || user.Username != "admin" {
		http.Error(w, "Admin privileges required", http.StatusForbidden)
		return
	}

	ws.renderUsersPage(w, r, user)
}

// handleChangePasswordPage renders the change password page
func (ws *WebServer) handleChangePasswordPage(w http.ResponseWriter, r *http.Request) {
	user := auth.GetUserFromContext(r.Context())
	if user == nil {
		http.Redirect(w, r, "/login", http.StatusFound)
		return
	}

	ws.renderChangePasswordPage(w, r, user, "")
}

// Template rendering methods

func (ws *WebServer) renderLoginPage(w http.ResponseWriter, r *http.Request, errorMsg string) {
	tmplContent := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Login - Network Dashboard</title>
    <link rel="stylesheet" href="/static/css/dashboard.css">
    <style>
        .login-container {
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .login-form {
            background: white;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
            width: 100%;
            max-width: 400px;
        }
        .login-form h1 {
            text-align: center;
            margin-bottom: 2rem;
            color: #333;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            font-weight: 600;
            color: #555;
        }
        .form-group input {
            width: 100%;
            padding: 0.75rem;
            border: 2px solid #ddd;
            border-radius: 5px;
            font-size: 1rem;
            transition: border-color 0.3s;
        }
        .form-group input:focus {
            outline: none;
            border-color: #667eea;
        }
        .btn-login {
            width: 100%;
            padding: 0.75rem;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 5px;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: background 0.3s;
        }
        .btn-login:hover {
            background: #5a67d8;
        }
        .error {
            color: #e53e3e;
            background: #fed7d7;
            padding: 0.75rem;
            border-radius: 5px;
            margin-bottom: 1rem;
            text-align: center;
        }
        .success {
            color: #38a169;
            background: #c6f6d5;
            padding: 0.75rem;
            border-radius: 5px;
            margin-bottom: 1rem;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="login-container">
        <form class="login-form" method="POST">
            <h1>Network Dashboard</h1>
            {{if .Error}}
                <div class="error">{{.Error}}</div>
            {{end}}
            {{if .Message}}
                <div class="success">{{.Message}}</div>
            {{end}}
            <div class="form-group">
                <label for="username">Username</label>
                <input type="text" id="username" name="username" required>
            </div>
            <div class="form-group">
                <label for="password">Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            <button type="submit" class="btn-login">Login</button>
        </form>
    </div>
</body>
</html>`

	tmpl, err := template.New("login").Parse(tmplContent)
	if err != nil {
		ws.logger.Errorf("Failed to parse login template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Error   string
		Message string
	}{
		Error:   errorMsg,
		Message: r.URL.Query().Get("message"),
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		ws.logger.Errorf("Failed to execute login template: %v", err)
	}
}

func (ws *WebServer) renderSetupPage(w http.ResponseWriter, r *http.Request, errorMsg string) {
	tmplContent := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Setup - Network Dashboard</title>
    <link rel="stylesheet" href="/static/css/dashboard.css">
    <style>
        .setup-container {
            display: flex;
            justify-content: center;
            align-items: center;
            min-height: 100vh;
            background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
        }
        .setup-form {
            background: white;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.2);
            width: 100%;
            max-width: 500px;
        }
        .setup-form h1 {
            text-align: center;
            margin-bottom: 1rem;
            color: #333;
        }
        .setup-info {
            background: #e6f3ff;
            padding: 1rem;
            border-radius: 5px;
            margin-bottom: 2rem;
            border-left: 4px solid #667eea;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            font-weight: 600;
            color: #555;
        }
        .form-group input {
            width: 100%;
            padding: 0.75rem;
            border: 2px solid #ddd;
            border-radius: 5px;
            font-size: 1rem;
            transition: border-color 0.3s;
        }
        .form-group input:focus {
            outline: none;
            border-color: #667eea;
        }
        .btn-setup {
            width: 100%;
            padding: 0.75rem;
            background: #667eea;
            color: white;
            border: none;
            border-radius: 5px;
            font-size: 1rem;
            font-weight: 600;
            cursor: pointer;
            transition: background 0.3s;
        }
        .btn-setup:hover {
            background: #5a67d8;
        }
        .error {
            color: #e53e3e;
            background: #fed7d7;
            padding: 0.75rem;
            border-radius: 5px;
            margin-bottom: 1rem;
            text-align: center;
        }
    </style>
</head>
<body>
    <div class="setup-container">
        <form class="setup-form" method="POST">
            <h1>First-Time Setup</h1>
            <div class="setup-info">
                <strong>Welcome to Network Dashboard!</strong><br>
                Please set up the administrator password to get started.
            </div>
            {{if .Error}}
                <div class="error">{{.Error}}</div>
            {{end}}
            <div class="form-group">
                <label for="password">Administrator Password</label>
                <input type="password" id="password" name="password" required>
            </div>
            <div class="form-group">
                <label for="confirm_password">Confirm Password</label>
                <input type="password" id="confirm_password" name="confirm_password" required>
            </div>
            <button type="submit" class="btn-setup">Complete Setup</button>
        </form>
    </div>
</body>
</html>`

	tmpl, err := template.New("setup").Parse(tmplContent)
	if err != nil {
		ws.logger.Errorf("Failed to parse setup template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Error string
	}{
		Error: errorMsg,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		ws.logger.Errorf("Failed to execute setup template: %v", err)
	}
}

func (ws *WebServer) renderUsersPage(w http.ResponseWriter, r *http.Request, currentUser *auth.User) {
	tmplContent := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>User Management - Network Dashboard</title>
    <link rel="stylesheet" href="/static/css/dashboard.css">
    <style>
        .user-management {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        .user-actions {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 2rem;
        }
        .user-table {
            width: 100%;
            border-collapse: collapse;
            background: white;
            border-radius: 10px;
            overflow: hidden;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        .user-table th,
        .user-table td {
            padding: 1rem;
            text-align: left;
            border-bottom: 1px solid #e2e8f0;
        }
        .user-table th {
            background: #667eea;
            color: white;
            font-weight: 600;
        }
        .user-table tr:hover {
            background: #f8f9ff;
        }
        .btn {
            padding: 0.5rem 1rem;
            border: none;
            border-radius: 5px;
            cursor: pointer;
            font-weight: 600;
            transition: background 0.3s;
            margin: 0 0.25rem;
        }
        .btn-primary {
            background: #667eea;
            color: white;
        }
        .btn-primary:hover {
            background: #5a67d8;
        }
        .btn-danger {
            background: #e53e3e;
            color: white;
        }
        .btn-danger:hover {
            background: #c53030;
        }
        .btn-secondary {
            background: #718096;
            color: white;
        }
        .btn-secondary:hover {
            background: #4a5568;
        }
    </style>
</head>
<body>
    <div class="user-management">
        <div class="user-actions">
            <h1>User Management</h1>
            <div>
                <button onclick="createUser()" class="btn btn-primary">Add User</button>
                <a href="/" class="btn btn-secondary">Back to Dashboard</a>
            </div>
        </div>
        
        <table class="user-table">
            <thead>
                <tr>
                    <th>Username</th>
                    <th>Created</th>
                    <th>Last Login</th>
                    <th>Actions</th>
                </tr>
            </thead>
            <tbody id="users-tbody">
                <!-- Users will be loaded here -->
            </tbody>
        </table>
    </div>

    <script>
        let users = [];

        async function loadUsers() {
            try {
                const response = await fetch('/api/auth/users');
                const result = await response.json();
                
                if (result.success) {
                    users = result.data.users;
                    renderUsers();
                } else {
                    alert('Failed to load users: ' + result.message);
                }
            } catch (error) {
                alert('Error loading users: ' + error.message);
            }
        }

        function renderUsers() {
            const tbody = document.getElementById('users-tbody');
            tbody.innerHTML = '';

            users.forEach(user => {
                const row = document.createElement('tr');
                
                const createdAt = new Date(user.created_at).toLocaleDateString();
                const lastLogin = user.last_login ? new Date(user.last_login).toLocaleDateString() : 'Never';
                
                row.innerHTML = ` + "`" + `
                    <td>${user.username}</td>
                    <td>${createdAt}</td>
                    <td>${lastLogin}</td>
                    <td>
                        ${user.username !== '{{.CurrentUser}}' && user.username !== 'admin' ? 
                            ` + "`" + `<button onclick="deleteUser('${user.username}')" class="btn btn-danger">Delete</button>` + "`" + ` : 
                            (user.username === 'admin' ? '<em>Admin User</em>' : '<em>Current User</em>')
                        }
                    </td>
                ` + "`" + `;
                
                tbody.appendChild(row);
            });
        }

        async function createUser() {
            const username = prompt('Enter username:');
            if (!username) return;

            try {
                const response = await fetch('/api/auth/users', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        username: username,
                        is_admin: false  // Always false since admin is determined by username
                    })
                });

                const result = await response.json();
                
                if (result.success) {
                    alert(` + "`" + `User created successfully!\n\nUsername: ${username}\nInitial Password: ${result.data.initial_password}\n\nPlease save this password securely.` + "`" + `);
                    loadUsers();
                } else {
                    alert('Failed to create user: ' + result.message);
                }
            } catch (error) {
                alert('Error creating user: ' + error.message);
            }
        }

        async function deleteUser(username) {
            if (!confirm(` + "`" + `Are you sure you want to delete user "${username}"?` + "`" + `)) return;

            try {
                const response = await fetch(` + "`" + `/api/auth/users/${username}` + "`" + `, {
                    method: 'DELETE'
                });

                const result = await response.json();
                
                if (result.success) {
                    alert('User deleted successfully');
                    loadUsers();
                } else {
                    alert('Failed to delete user: ' + result.message);
                }
            } catch (error) {
                alert('Error deleting user: ' + error.message);
            }
        }

        // Load users on page load
        loadUsers();
    </script>
</body>
</html>`

	tmpl, err := template.New("users").Parse(tmplContent)
	if err != nil {
		ws.logger.Errorf("Failed to parse users template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		CurrentUser string
	}{
		CurrentUser: currentUser.Username,
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		ws.logger.Errorf("Failed to execute users template: %v", err)
	}
}

func (ws *WebServer) renderChangePasswordPage(w http.ResponseWriter, r *http.Request, user *auth.User, errorMsg string) {
	tmplContent := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Change Password - Network Dashboard</title>
    <link rel="stylesheet" href="/static/css/dashboard.css">
    <style>
        .password-container {
            max-width: 500px;
            margin: 2rem auto;
            padding: 2rem;
            background: white;
            border-radius: 10px;
            box-shadow: 0 4px 6px rgba(0,0,0,0.1);
        }
        .form-group {
            margin-bottom: 1rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            font-weight: 600;
            color: #555;
        }
        .form-group input {
            width: 100%;
            padding: 0.75rem;
            border: 2px solid #ddd;
            border-radius: 5px;
            font-size: 1rem;
        }
        .form-group input:focus {
            outline: none;
            border-color: #667eea;
        }
        .btn {
            padding: 0.75rem 1.5rem;
            border: none;
            border-radius: 5px;
            cursor: pointer;
            font-weight: 600;
            transition: background 0.3s;
            margin: 0 0.5rem;
        }
        .btn-primary {
            background: #667eea;
            color: white;
        }
        .btn-primary:hover {
            background: #5a67d8;
        }
        .btn-secondary {
            background: #718096;
            color: white;
        }
        .btn-secondary:hover {
            background: #4a5568;
        }
        .error {
            color: #e53e3e;
            background: #fed7d7;
            padding: 0.75rem;
            border-radius: 5px;
            margin-bottom: 1rem;
        }
        .success {
            color: #38a169;
            background: #c6f6d5;
            padding: 0.75rem;
            border-radius: 5px;
            margin-bottom: 1rem;
        }
        .actions {
            display: flex;
            justify-content: space-between;
            margin-top: 2rem;
        }
    </style>
</head>
<body>
    <div class="password-container">
        <h1>Change Password</h1>
        
        <div id="message-area"></div>
        
        <form id="password-form">
            {{if .RequireCurrentPassword}}
            <div class="form-group">
                <label for="current_password">Current Password</label>
                <input type="password" id="current_password" name="current_password" required>
            </div>
            {{end}}
            <div class="form-group">
                <label for="new_password">New Password</label>
                <input type="password" id="new_password" name="new_password" required>
            </div>
            <div class="form-group">
                <label for="confirm_password">Confirm New Password</label>
                <input type="password" id="confirm_password" name="confirm_password" required>
            </div>
            <div class="actions">
                <a href="/" class="btn btn-secondary">Cancel</a>
                <button type="submit" class="btn btn-primary">Change Password</button>
            </div>
        </form>
    </div>

    <script>
        document.getElementById('password-form').addEventListener('submit', async function(e) {
            e.preventDefault();
            
            const currentPassword = document.getElementById('current_password')?.value || '';
            const newPassword = document.getElementById('new_password').value;
            const confirmPassword = document.getElementById('confirm_password').value;
            
            if (newPassword !== confirmPassword) {
                showMessage('Passwords do not match', 'error');
                return;
            }
            
            try {
                const response = await fetch('/api/auth/password', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        current_password: currentPassword,
                        new_password: newPassword,
                        confirm_password: confirmPassword
                    })
                });
                
                const result = await response.json();
                
                if (result.success) {
                    showMessage('Password changed successfully', 'success');
                    // Redirect after a delay
                    setTimeout(() => {
                        window.location.href = '/';
                    }, 2000);
                } else {
                    showMessage(result.message, 'error');
                }
            } catch (error) {
                showMessage('Error changing password: ' + error.message, 'error');
            }
        });
        
        function showMessage(message, type) {
            const messageArea = document.getElementById('message-area');
            messageArea.innerHTML = ` + "`" + `<div class="${type}">${message}</div>` + "`" + `;
        }
    </script>
</body>
</html>`

	tmpl, err := template.New("change-password").Parse(tmplContent)
	if err != nil {
		ws.logger.Errorf("Failed to parse change password template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := struct {
		Error                  string
		RequireCurrentPassword bool
	}{
		Error:                  errorMsg,
		RequireCurrentPassword: ws.config.Dashboard.IAPAuth == "none",
	}

	w.Header().Set("Content-Type", "text/html")
	if err := tmpl.Execute(w, data); err != nil {
		ws.logger.Errorf("Failed to execute change password template: %v", err)
	}
}
