package daemon

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Client represents a daemon client
type Client struct {
	socketPath string
	timeout    time.Duration
}

// NewClient creates a new daemon client
func NewClient() *Client {
	return &Client{
		socketPath: filepath.Join(os.TempDir(), "moz-daemon.sock"),
		timeout:    30 * time.Second,
	}
}

// ExecuteCommand executes a command via the daemon
func (c *Client) ExecuteCommand(command string, args ...string) (interface{}, error) {
	// Connect to daemon
	conn, err := net.DialTimeout("unix", c.socketPath, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}
	defer func() {
		_ = conn.Close() // Ignore connection cleanup errors
	}()

	// Set connection timeout
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("failed to set deadline: %w", err)
	}

	// Create request
	req := Request{
		ID:        fmt.Sprintf("client-%d", time.Now().UnixNano()),
		Command:   command,
		Arguments: args,
		Timestamp: time.Now(),
	}

	// Send request
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(req); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	decoder := json.NewDecoder(conn)
	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Check response
	if !resp.Success {
		return nil, fmt.Errorf("daemon error: %s", resp.Error)
	}

	return resp.Result, nil
}

// Put executes a PUT command via daemon
func (c *Client) Put(key, value string) error {
	_, err := c.ExecuteCommand("put", key, value)
	return err
}

// Get executes a GET command via daemon
func (c *Client) Get(key string) (string, error) {
	result, err := c.ExecuteCommand("get", key)
	if err != nil {
		return "", err
	}

	value, ok := result.(string)
	if !ok {
		return "", fmt.Errorf("unexpected response type: %T", result)
	}

	return value, nil
}

// Delete executes a DELETE command via daemon
func (c *Client) Delete(key string) error {
	_, err := c.ExecuteCommand("delete", key)
	return err
}

// List executes a LIST command via daemon
func (c *Client) List() (map[string]string, error) {
	result, err := c.ExecuteCommand("list")
	if err != nil {
		return nil, err
	}

	// Convert interface{} to map[string]string
	resultMap, ok := result.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response type: %T", result)
	}

	entries := make(map[string]string)
	for k, v := range resultMap {
		if str, ok := v.(string); ok {
			entries[k] = str
		}
	}

	return entries, nil
}

// Compact executes a COMPACT command via daemon
func (c *Client) Compact() error {
	_, err := c.ExecuteCommand("compact")
	return err
}

// Stats executes a STATS command via daemon
func (c *Client) Stats() (interface{}, error) {
	return c.ExecuteCommand("stats")
}

// Ping checks if daemon is responsive
func (c *Client) Ping() error {
	result, err := c.ExecuteCommand("ping")
	if err != nil {
		return err
	}

	if pong, ok := result.(string); !ok || pong != "pong" {
		return fmt.Errorf("unexpected ping response: %v", result)
	}

	return nil
}

// IsConnected checks if client can connect to daemon
func (c *Client) IsConnected() bool {
	return c.Ping() == nil
}
