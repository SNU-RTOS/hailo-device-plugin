package plugin

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	kubeletEndpoint = "/var/lib/kubelet/device-plugins/kubelet.sock"
)

// RegisterWithKubelet registers the device plugin with kubelet
// Returns error if registration fails after retries
func RegisterWithKubelet(plugin *HailoDevicePlugin, maxRetries int) error {
	var lastErr error

	for attempt := 1; attempt <= maxRetries; attempt++ {
		err := registerOnce(plugin)
		if err == nil {
			log.Println("Device plugin registered successfully with kubelet")
			return nil
		}

		lastErr = err
		log.Printf("Registration attempt %d/%d failed: %v", attempt, maxRetries, err)

		if attempt < maxRetries {
			// Wait before retrying (exponential backoff)
			backoff := time.Duration(attempt) * 2 * time.Second
			log.Printf("Retrying in %v...", backoff)
			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("failed to register after %d attempts: %w", maxRetries, lastErr)
}

// registerOnce attempts a single registration with kubelet
func registerOnce(plugin *HailoDevicePlugin) error {
	// Check if kubelet socket exists
	log.Printf("Checking kubelet socket at: %s", kubeletEndpoint)
	if _, err := os.Stat(kubeletEndpoint); os.IsNotExist(err) {
		return fmt.Errorf("kubelet socket not found at %s", kubeletEndpoint)
	}
	log.Println("Kubelet socket found, attempting to connect...")

	// Connect to kubelet with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "unix://"+kubeletEndpoint,
		grpc.WithInsecure(),
		grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("failed to connect to kubelet: %w", err)
	}
	defer conn.Close()

	log.Println("Connected to kubelet, sending registration request...")

	// Create registration request
	client := pluginapi.NewRegistrationClient(conn)
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     filepath.Base(plugin.SocketPath),
		ResourceName: plugin.ResourceName,
	}

	log.Printf("Registering with kubelet: Version=%s, Endpoint=%s, ResourceName=%s",
		req.Version, req.Endpoint, req.ResourceName)

	// Send registration request with timeout
	regCtx, regCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer regCancel()

	_, err = client.Register(regCtx, req)
	if err != nil {
		return fmt.Errorf("registration RPC failed: %w", err)
	}

	return nil
}
