package statemachine

import (
	"fmt"
	"log"
	"os"
	"time"

	"hailo-device-plugin/pkg/plugin"
)

// handleWaitingForKubelet waits for the kubelet socket to exist
func (sm *StateMachine) handleWaitingForKubelet() error {
	log.Println("Waiting for kubelet socket...")

	// Check if socket already exists
	if _, err := os.Stat(sm.config.KubeletSocket); err == nil {
		log.Printf("Kubelet socket found at %s", sm.config.KubeletSocket)
		return nil
	}

	// Socket doesn't exist, watch for its creation
	log.Printf("Kubelet socket not found, watching for creation...")

	watcher, err := NewKubeletWatcher(sm.ctx, sm.config.KubeletSocket)
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("failed to start watcher: %w", err)
	}

	// Wait for socket to be created
	for {
		select {
		case event := <-watcher.Events():
			if event == EventSocketCreated {
				log.Println("Kubelet socket created")
				return nil
			}

		case err := <-watcher.Errors():
			log.Printf("Watcher error (non-fatal): %v", err)
			// Continue waiting despite errors

		case <-sm.ctx.Done():
			return fmt.Errorf("context cancelled while waiting for kubelet")
		}
	}
}

// handleInitializingServer creates and starts the gRPC server
func (sm *StateMachine) handleInitializingServer() error {
	log.Println("Initializing gRPC server...")

	// Verify kubelet socket still exists
	if _, err := os.Stat(sm.config.KubeletSocket); os.IsNotExist(err) {
		return fmt.Errorf("kubelet socket disappeared during initialization")
	}

	// Create server
	server, err := plugin.NewServer(sm.plugin, sm.config.PluginSocket)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	sm.server = server

	// Start server (this launches a goroutine)
	if err := sm.server.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Give server a moment to initialize
	time.Sleep(500 * time.Millisecond)

	log.Println("gRPC server initialized successfully")
	return nil
}

// handleRegistering registers the device plugin with kubelet
func (sm *StateMachine) handleRegistering() error {
	log.Println("Registering with kubelet...")

	// Verify kubelet socket still exists
	if _, err := os.Stat(sm.config.KubeletSocket); os.IsNotExist(err) {
		return fmt.Errorf("kubelet socket disappeared before registration")
	}

	// Register with retry (max 5 attempts)
	if err := plugin.RegisterWithKubelet(sm.plugin, 5); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	log.Println("Registration successful")
	return nil
}

// handleRunning monitors the kubelet socket and waits for events
func (sm *StateMachine) handleRunning() WatchEvent {
	log.Println("Entering RUNNING state, monitoring kubelet socket...")

	// Create helper directories
	ensureHelperDirectories()

	// Create watcher for kubelet socket
	watcher, err := NewKubeletWatcher(sm.ctx, sm.config.KubeletSocket)
	if err != nil {
		log.Printf("Failed to create watcher: %v", err)
		return EventSocketDeleted // Trigger cleanup
	}
	defer watcher.Close()

	if err := watcher.Start(); err != nil {
		log.Printf("Failed to start watcher: %v", err)
		return EventSocketDeleted // Trigger cleanup
	}

	// Monitor events
	for {
		select {
		case event := <-watcher.Events():
			if event == EventSocketDeleted {
				log.Println("Kubelet socket deleted, needs cleanup")
				return EventSocketDeleted
			}

		case err := <-watcher.Errors():
			log.Printf("Watcher error: %v", err)
			// Don't exit on watcher errors, just log them

		case serverErr := <-sm.server.Done():
			log.Printf("gRPC server exited: %v", serverErr)
			return EventSocketDeleted // Trigger cleanup

		case <-sm.ctx.Done():
			log.Println("Shutdown signal received in RUNNING state")
			return EventSocketDeleted // Will be handled as shutdown in main loop
		}
	}
}

// handleCleanup stops the gRPC server and cleans up resources
func (sm *StateMachine) handleCleanup() {
	log.Println("Cleaning up resources...")

	// Stop gRPC server
	if sm.server != nil {
		if err := sm.server.Stop(); err != nil {
			log.Printf("Error stopping server: %v", err)
		}
		sm.server = nil
	}

	// Close watcher if exists
	if sm.watcher != nil {
		sm.watcher.Close()
		sm.watcher = nil
	}

	log.Println("Cleanup complete")
}

// handleShutdown performs final cleanup and exits
func (sm *StateMachine) handleShutdown() error {
	log.Println("Shutting down device plugin...")

	// Cleanup resources
	sm.handleCleanup()

	log.Println("Device plugin shutdown complete")
	return nil
}

// ensureHelperDirectories creates directories needed by CDI
func ensureHelperDirectories() {
	// Create empty chardev directory for sysfs isolation
	if err := os.MkdirAll("/var/lib/hailo-cdi/empty-chardev", 0755); err != nil {
		log.Printf("Warning: failed to create empty-chardev directory: %v", err)
	}

	// Create cleanup script
	if err := ensureCleanupScript(); err != nil {
		log.Printf("Warning: failed to create cleanup script: %v", err)
	}
}

// ensureCleanupScript creates a cleanup script for removing files from empty-chardev
func ensureCleanupScript() error {
	scriptPath := "/var/lib/hailo-cdi/cleanup-empty-chardev.sh"
	scriptContent := `#!/bin/sh
# Cleanup script to remove all files from empty-chardev directory
rm -rf /var/lib/hailo-cdi/empty-chardev/*
exit 0
`
	if err := os.WriteFile(scriptPath, []byte(scriptContent), 0755); err != nil {
		return fmt.Errorf("failed to create cleanup script: %w", err)
	}
	return nil
}
