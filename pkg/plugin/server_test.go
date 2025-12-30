package plugin

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func TestServer_StartStop(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-plugin.sock")

	// Create minimal plugin
	plugin := &HailoDevicePlugin{
		CdiDir:       tempDir,
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	// Create server
	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Start server
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Verify socket exists
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Fatal("Socket file was not created")
	}

	// Try to connect to verify server is running
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to connect to server: %v", err)
	}
	conn.Close()

	// Stop server
	if err := server.Stop(); err != nil {
		t.Fatalf("Failed to stop server: %v", err)
	}

	// Verify socket is removed
	if _, err := os.Stat(socketPath); !os.IsNotExist(err) {
		t.Error("Socket file was not removed after stop")
	}
}

func TestServer_MultipleStopCalls(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-plugin.sock")

	plugin := &HailoDevicePlugin{
		CdiDir:       tempDir,
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// First stop
	if err := server.Stop(); err != nil {
		t.Fatalf("First stop failed: %v", err)
	}

	// Second stop should not error
	if err := server.Stop(); err != nil {
		t.Errorf("Second stop should not error: %v", err)
	}
}

func TestServer_StaleSocketCleanup(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "stale-socket.sock")

	// Create stale socket file
	staleFile, err := os.Create(socketPath)
	if err != nil {
		t.Fatalf("Failed to create stale socket: %v", err)
	}
	staleFile.Close()

	plugin := &HailoDevicePlugin{
		CdiDir:       tempDir,
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Should successfully start and clean up stale socket
	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server with stale socket: %v", err)
	}

	// Verify new socket is listening
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock())
	if err != nil {
		t.Fatalf("Failed to connect after stale socket cleanup: %v", err)
	}
	conn.Close()

	server.Stop()
}

func TestServer_DoneChannel(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-plugin.sock")

	plugin := &HailoDevicePlugin{
		CdiDir:       tempDir,
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Stop server
	server.Stop()

	// Done channel should eventually receive
	select {
	case <-server.Done():
		// Success
	case <-time.After(3 * time.Second):
		t.Fatal("Done channel did not receive after stop")
	}
}

func TestServer_InvalidSocketPath(t *testing.T) {
	// Try to create socket in non-existent directory
	socketPath := "/nonexistent/directory/test.sock"

	plugin := &HailoDevicePlugin{
		CdiDir:       "/tmp",
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	// Should fail to start with invalid path
	if err := server.Start(); err == nil {
		t.Error("Expected error starting server with invalid socket path")
		server.Stop()
	}
}

func TestServer_GracefulStopTimeout(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-plugin.sock")

	plugin := &HailoDevicePlugin{
		CdiDir:       tempDir,
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}

	// Create a client connection to keep server busy
	conn, err := grpc.Dial("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		t.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	// Measure stop time (should respect 5s timeout)
	start := time.Now()
	server.Stop()
	elapsed := time.Since(start)

	// Should complete within reasonable time (< 7s including overhead)
	if elapsed > 7*time.Second {
		t.Errorf("Stop took too long: %v (expected < 7s)", elapsed)
	}
}

func TestServer_SocketPermissions(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-plugin.sock")

	plugin := &HailoDevicePlugin{
		CdiDir:       tempDir,
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Check socket file exists and has correct type
	info, err := os.Stat(socketPath)
	if err != nil {
		t.Fatalf("Failed to stat socket: %v", err)
	}

	// Unix socket should have mode indicating socket type
	if info.Mode()&os.ModeSocket == 0 {
		t.Error("Socket file is not a Unix socket")
	}
}

func TestServer_ConcurrentAccess(t *testing.T) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "test-plugin.sock")

	plugin := &HailoDevicePlugin{
		CdiDir:       tempDir,
		SocketPath:   socketPath,
		ResourceName: "test/device",
	}

	server, err := NewServer(plugin, socketPath)
	if err != nil {
		t.Fatalf("Failed to create server: %v", err)
	}

	if err := server.Start(); err != nil {
		t.Fatalf("Failed to start server: %v", err)
	}
	defer server.Stop()

	// Create multiple concurrent connections
	done := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			conn, err := grpc.DialContext(ctx, "unix://"+socketPath,
				grpc.WithTransportCredentials(insecure.NewCredentials()),
				grpc.WithBlock())
			if err != nil {
				done <- err
				return
			}
			conn.Close()
			done <- nil
		}()
	}

	// All connections should succeed
	for i := 0; i < 5; i++ {
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("Concurrent connection %d failed: %v", i, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent connections")
		}
	}
}

// Helper function to check if server is listening
func isServerListening(socketPath string) bool {
	conn, err := net.DialTimeout("unix", socketPath, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
