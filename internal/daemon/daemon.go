package daemon

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/nyasuto/moz/internal/kvstore"
)

// DaemonManager manages the background daemon process
type DaemonManager struct {
	store      *kvstore.KVStore
	listener   net.Listener
	socketPath string
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	running    bool
	mu         sync.RWMutex
}

// Request represents a client request to the daemon
type Request struct {
	ID        string    `json:"id"`
	Command   string    `json:"command"`
	Arguments []string  `json:"arguments"`
	Timestamp time.Time `json:"timestamp"`
}

// Response represents a daemon response to the client
type Response struct {
	ID       string        `json:"id"`
	Success  bool          `json:"success"`
	Result   interface{}   `json:"result,omitempty"`
	Error    string        `json:"error,omitempty"`
	Duration time.Duration `json:"duration"`
}

// NewDaemonManager creates a new daemon manager
func NewDaemonManager(store *kvstore.KVStore) *DaemonManager {
	ctx, cancel := context.WithCancel(context.Background())

	// Use temp directory for socket
	socketPath := filepath.Join(os.TempDir(), "moz-daemon.sock")

	return &DaemonManager{
		store:      store,
		socketPath: socketPath,
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start starts the daemon server
func (d *DaemonManager) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.running {
		return fmt.Errorf("daemon already running")
	}

	// Remove existing socket if present
	_ = os.Remove(d.socketPath)

	// Create Unix socket listener
	listener, err := net.Listen("unix", d.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket listener: %w", err)
	}

	d.listener = listener
	d.running = true

	// Start accepting connections
	d.wg.Add(1)
	go d.acceptConnections()

	return nil
}

// Stop stops the daemon server
func (d *DaemonManager) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return nil
	}

	// Cancel context to stop all goroutines
	d.cancel()

	// Close listener
	if d.listener != nil {
		_ = d.listener.Close() // Ignore error during shutdown
	}

	// Wait for all goroutines to finish
	d.wg.Wait()

	// Remove socket file
	_ = os.Remove(d.socketPath)

	d.running = false
	return nil
}

// IsRunning checks if daemon is running
func (d *DaemonManager) IsRunning() bool {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.running
}

// GetSocketPath returns the socket path
func (d *DaemonManager) GetSocketPath() string {
	return d.socketPath
}

// acceptConnections accepts incoming connections
func (d *DaemonManager) acceptConnections() {
	defer d.wg.Done()

	for {
		select {
		case <-d.ctx.Done():
			return
		default:
			conn, err := d.listener.Accept()
			if err != nil {
				select {
				case <-d.ctx.Done():
					return
				default:
					continue
				}
			}

			// Handle connection in goroutine
			d.wg.Add(1)
			go d.handleConnection(conn)
		}
	}
}

// handleConnection handles a client connection
func (d *DaemonManager) handleConnection(conn net.Conn) {
	defer d.wg.Done()
	defer func() {
		_ = conn.Close() // Ignore connection cleanup errors
	}()

	// Set connection timeout
	if err := conn.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		return
	}

	scanner := bufio.NewScanner(conn)
	encoder := json.NewEncoder(conn)

	for scanner.Scan() {
		var req Request
		if err := json.Unmarshal(scanner.Bytes(), &req); err != nil {
			response := Response{
				ID:      req.ID,
				Success: false,
				Error:   fmt.Sprintf("invalid request format: %v", err),
			}
			if err := encoder.Encode(response); err != nil {
				break
			}
			continue
		}

		// Process request
		response := d.processRequest(req)

		// Send response
		if err := encoder.Encode(response); err != nil {
			break
		}
	}
}

// processRequest processes a client request
func (d *DaemonManager) processRequest(req Request) Response {
	start := time.Now()

	response := Response{
		ID: req.ID,
	}

	switch req.Command {
	case "put":
		if len(req.Arguments) != 2 {
			response.Success = false
			response.Error = "put requires exactly 2 arguments: key and value"
		} else {
			err := d.store.Put(req.Arguments[0], req.Arguments[1])
			if err != nil {
				response.Success = false
				response.Error = err.Error()
			} else {
				response.Success = true
				response.Result = "OK"
			}
		}

	case "get":
		if len(req.Arguments) != 1 {
			response.Success = false
			response.Error = "get requires exactly 1 argument: key"
		} else {
			value, err := d.store.Get(req.Arguments[0])
			if err != nil {
				response.Success = false
				response.Error = err.Error()
			} else {
				response.Success = true
				response.Result = value
			}
		}

	case "delete":
		if len(req.Arguments) != 1 {
			response.Success = false
			response.Error = "delete requires exactly 1 argument: key"
		} else {
			err := d.store.Delete(req.Arguments[0])
			if err != nil {
				response.Success = false
				response.Error = err.Error()
			} else {
				response.Success = true
				response.Result = "OK"
			}
		}

	case "list":
		entries, err := d.store.List()
		if err != nil {
			response.Success = false
			response.Error = err.Error()
		} else {
			response.Success = true
			response.Result = entries
		}

	case "compact":
		err := d.store.Compact()
		if err != nil {
			response.Success = false
			response.Error = err.Error()
		} else {
			response.Success = true
			response.Result = "Compaction completed"
		}

	case "stats":
		stats, err := d.store.GetCompactionStats()
		if err != nil {
			response.Success = false
			response.Error = err.Error()
		} else {
			response.Success = true
			response.Result = stats
		}

	case "ping":
		response.Success = true
		response.Result = "pong"

	case "help":
		response.Success = true
		response.Result = "Available commands: put, get, delete, list, compact, stats, ping, help"

	default:
		response.Success = false
		response.Error = fmt.Sprintf("unknown command: %s", req.Command)
	}

	response.Duration = time.Since(start)
	return response
}

// IsDaemonRunning checks if daemon is already running by trying to connect
func IsDaemonRunning() bool {
	socketPath := filepath.Join(os.TempDir(), "moz-daemon.sock")

	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	defer func() {
		_ = conn.Close() // Ignore connection cleanup errors
	}()

	// Send ping request
	req := Request{
		ID:      "ping-check",
		Command: "ping",
	}

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := encoder.Encode(req); err != nil {
		return false
	}

	var resp Response
	if err := decoder.Decode(&resp); err != nil {
		return false
	}

	return resp.Success
}

// GetDaemonPID returns the PID of the running daemon
func GetDaemonPID() (int, error) {
	// Use a fixed path to avoid gosec issues
	pidFilePath := filepath.Join(os.TempDir(), "moz-daemon.pid")

	// #nosec G304 - This is a controlled file path in temp directory
	data, err := os.ReadFile(pidFilePath)
	if err != nil {
		return 0, err
	}

	var pid int
	if _, err := fmt.Sscanf(string(data), "%d", &pid); err != nil {
		return 0, err
	}

	// Check if process exists
	if err := syscall.Kill(pid, 0); err != nil {
		return 0, err
	}

	return pid, nil
}

// WritePIDFile writes the daemon PID to file
func WritePIDFile() error {
	pidFile := filepath.Join(os.TempDir(), "moz-daemon.pid")
	return os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0600)
}

// RemovePIDFile removes the daemon PID file
func RemovePIDFile() error {
	pidFile := filepath.Join(os.TempDir(), "moz-daemon.pid")
	return os.Remove(pidFile)
}
