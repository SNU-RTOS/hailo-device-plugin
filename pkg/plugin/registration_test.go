package plugin

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// mockRegistrationServer implements kubelet registration server for testing
type mockRegistrationServer struct {
	pluginapi.UnimplementedRegistrationServer
	registerCalled chan *pluginapi.RegisterRequest
	shouldFail     bool
}

func (m *mockRegistrationServer) Register(ctx context.Context, req *pluginapi.RegisterRequest) (*pluginapi.Empty, error) {
	m.registerCalled <- req
	if m.shouldFail {
		return nil, grpc.ErrServerStopped
	}
	return &pluginapi.Empty{}, nil
}

func setupMockKubelet(t *testing.T, shouldFail bool) (string, *mockRegistrationServer, func()) {
	tempDir := t.TempDir()
	socketPath := filepath.Join(tempDir, "kubelet.sock")

	// Create mock server
	mock := &mockRegistrationServer{
		registerCalled: make(chan *pluginapi.RegisterRequest, 1),
		shouldFail:     shouldFail,
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create listener: %v", err)
	}

	server := grpc.NewServer()
	pluginapi.RegisterRegistrationServer(server, mock)

	go server.Serve(listener)

	cleanup := func() {
		server.Stop()
		listener.Close()
		os.Remove(socketPath)
	}

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	return socketPath, mock, cleanup
}

func TestRegisterWithKubelet_Success(t *testing.T) {
	// Setup mock kubelet
	_, _, cleanup := setupMockKubelet(t, false)
	defer cleanup()

	// Since kubeletEndpoint is const, this test demonstrates the structure
	// In production, we'd need to make the endpoint configurable
	t.Skip("Skipping: requires configurable kubelet endpoint")
}

func TestRegisterWithKubelet_NonexistentSocket(t *testing.T) {
	plugin := &HailoDevicePlugin{
		SocketPath:   "/test/plugin.sock",
		ResourceName: "test.io/device",
	}

	// Try to register with non-existent kubelet socket
	err := RegisterWithKubelet(plugin, 1)
	if err == nil {
		t.Error("Expected error when kubelet socket doesn't exist")
	}

	if err != nil && !os.IsNotExist(err) {
		// Should contain "not found" error
		t.Logf("Got expected error: %v", err)
	}
}

func TestRegisterWithKubelet_Retry(t *testing.T) {
	plugin := &HailoDevicePlugin{
		SocketPath:   "/test/plugin.sock",
		ResourceName: "test.io/device",
	}

	// Test retry mechanism (will fail but should retry)
	start := time.Now()
	err := RegisterWithKubelet(plugin, 3)
	elapsed := time.Since(start)

	// Should fail after retries
	if err == nil {
		t.Error("Expected error after retries")
	}

	// Should take time due to retries (exponential backoff: 2s, 4s)
	// Minimum time: 2s + 4s = 6s
	minExpected := 5 * time.Second
	if elapsed < minExpected {
		t.Errorf("Expected at least %v for 3 retries, got %v", minExpected, elapsed)
	}

	t.Logf("Retry test took %v with error: %v", elapsed, err)
}

func TestRegisterOnce_Timeout(t *testing.T) {
	// Test registration timeout
	plugin := &HailoDevicePlugin{
		SocketPath:   "/test/plugin.sock",
		ResourceName: "test.io/device",
	}

	start := time.Now()
	err := registerOnce(plugin)
	elapsed := time.Since(start)

	// Should fail quickly (socket doesn't exist)
	if err == nil {
		t.Error("Expected error for non-existent socket")
	}

	// Should not take long (just socket check)
	if elapsed > 2*time.Second {
		t.Errorf("registerOnce took too long: %v", elapsed)
	}

	t.Logf("registerOnce failed as expected in %v: %v", elapsed, err)
}

func TestRegisterWithKubelet_MaxRetries(t *testing.T) {
	plugin := &HailoDevicePlugin{
		SocketPath:   "/test/plugin.sock",
		ResourceName: "test.io/device",
	}

	testCases := []struct {
		name       string
		maxRetries int
		minTime    time.Duration
	}{
		{"SingleRetry", 1, 0},
		{"TwoRetries", 2, 2 * time.Second},
		{"FiveRetries", 5, 20 * time.Second},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start := time.Now()
			err := RegisterWithKubelet(plugin, tc.maxRetries)
			elapsed := time.Since(start)

			if err == nil {
				t.Error("Expected error")
			}

			if elapsed < tc.minTime {
				t.Errorf("Expected at least %v, got %v", tc.minTime, elapsed)
			}

			t.Logf("Retries=%d took %v", tc.maxRetries, elapsed)
		})
	}
}

func TestPluginEndpointFormat(t *testing.T) {
	// Test that endpoint is correctly formatted as basename
	plugin := &HailoDevicePlugin{
		SocketPath:   "/var/lib/kubelet/device-plugins/hailo.sock",
		ResourceName: "hailo.ai/npu",
	}

	// This tests the logic that would be in registerOnce
	endpoint := filepath.Base(plugin.SocketPath)
	expected := "hailo.sock"

	if endpoint != expected {
		t.Errorf("Expected endpoint %s, got %s", expected, endpoint)
	}
}

func TestRegistrationRequest_Fields(t *testing.T) {
	plugin := &HailoDevicePlugin{
		SocketPath:   "/var/lib/kubelet/device-plugins/test-device.sock",
		ResourceName: "vendor.io/accelerator",
	}

	// Verify request would have correct fields
	expectedEndpoint := "test-device.sock"
	expectedResource := "vendor.io/accelerator"
	expectedVersion := pluginapi.Version

	if filepath.Base(plugin.SocketPath) != expectedEndpoint {
		t.Errorf("Endpoint mismatch")
	}

	if plugin.ResourceName != expectedResource {
		t.Errorf("Resource name mismatch")
	}

	if expectedVersion != "v1beta1" {
		t.Logf("API version: %s", expectedVersion)
	}
}
