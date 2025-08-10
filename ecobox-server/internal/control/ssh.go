package control

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"time"

	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
)

// SSHClient handles SSH connections and command execution
type SSHClient struct{}

// NewSSHClient creates a new SSH client instance
func NewSSHClient() *SSHClient {
	return &SSHClient{}
}

// ExecuteCommand establishes SSH connection and executes a command
func (s *SSHClient) ExecuteCommand(host string, port int, user string, keyPath string, command string) error {
	// Create SSH client configuration
	config, err := s.createSSHConfig(user, keyPath)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	// Connect to the SSH server
	address := host + ":" + strconv.Itoa(port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server %s: %w", address, err)
	}
	defer client.Close()

	// Create a session
	session, err := client.NewSession()
	if err != nil {
		return fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	// Execute the command
	if err := session.Run(command); err != nil {
		return fmt.Errorf("failed to execute command '%s': %w", command, err)
	}

	return nil
}

// createSSHConfig creates SSH client configuration
func (s *SSHClient) createSSHConfig(user string, keyPath string) (*ssh.ClientConfig, error) {
	var authMethods []ssh.AuthMethod

	// Try key-based authentication first if key path provided
	if keyPath != "" {
		key, err := s.loadPrivateKey(keyPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load private key: %w", err)
		}
		authMethods = append(authMethods, ssh.PublicKeys(key))
	}

	// Try ssh-agent authentication
	if agentAuth := s.getSSHAgentAuth(); agentAuth != nil {
		authMethods = append(authMethods, agentAuth)
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication methods available")
	}

	config := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		Timeout:         10 * time.Second,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Note: In production, use proper host key verification
	}

	return config, nil
}

// loadPrivateKey loads a private key from file
func (s *SSHClient) loadPrivateKey(keyPath string) (ssh.Signer, error) {
	key, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file: %w", err)
	}

	// Parse the private key
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return signer, nil
}

// getSSHAgentAuth attempts to get SSH agent authentication
func (s *SSHClient) getSSHAgentAuth() ssh.AuthMethod {
	if sshAgent, err := net.Dial("unix", os.Getenv("SSH_AUTH_SOCK")); err == nil {
		return ssh.PublicKeysCallback(agent.NewClient(sshAgent).Signers)
	}
	return nil
}

// TestConnection tests SSH connectivity without executing commands
func (s *SSHClient) TestConnection(host string, port int, user string, keyPath string) error {
	config, err := s.createSSHConfig(user, keyPath)
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	address := host + ":" + strconv.Itoa(port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return fmt.Errorf("failed to connect to SSH server %s: %w", address, err)
	}
	defer client.Close()

	return nil
}

// ExecuteCommandWithOutput executes command and returns output
func (s *SSHClient) ExecuteCommandWithOutput(host string, port int, user string, keyPath string, command string) (string, error) {
	config, err := s.createSSHConfig(user, keyPath)
	if err != nil {
		return "", fmt.Errorf("failed to create SSH config: %w", err)
	}

	address := host + ":" + strconv.Itoa(port)
	client, err := ssh.Dial("tcp", address, config)
	if err != nil {
		return "", fmt.Errorf("failed to connect to SSH server %s: %w", address, err)
	}
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return "", fmt.Errorf("failed to create SSH session: %w", err)
	}
	defer session.Close()

	output, err := session.Output(command)
	if err != nil {
		return "", fmt.Errorf("failed to execute command '%s': %w", command, err)
	}

	return string(output), nil
}
