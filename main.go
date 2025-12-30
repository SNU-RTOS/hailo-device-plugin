package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"hailo-device-plugin/pkg/monitor"
	"hailo-device-plugin/pkg/statemachine"
)

const (
	kubeletSocket = "/var/lib/kubelet/device-plugins/kubelet.sock"
	pluginSocket  = "/var/lib/kubelet/device-plugins/hailo.sock"
	resourceName  = "hailo.ai/npu"
	cdiDir        = "/etc/cdi"
)

func main() {
	log.Println("Starting Hailo device plugin...")

	// Create CDI directory
	if err := os.MkdirAll(cdiDir, 0755); err != nil {
		log.Fatalf("Failed to create CDI directory: %v", err)
	}

	// Create context with cancellation for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGTERM, syscall.SIGINT)

	// Start resource monitor
	mon := monitor.NewResourceMonitor(cdiDir)
	mon.Start(ctx)
	log.Println("Resource monitor started")

	// Create state machine configuration
	config := &statemachine.Config{
		KubeletSocket: kubeletSocket,
		PluginSocket:  pluginSocket,
		ResourceName:  resourceName,
		CdiDir:        cdiDir,
	}

	// Create and start state machine
	sm := statemachine.New(ctx, config)

	// Handle shutdown signal in a goroutine
	go func() {
		sig := <-sigChan
		log.Printf("Received signal %v, initiating shutdown...", sig)
		sm.Shutdown()
	}()

	// Run state machine (blocking)
	if err := sm.Run(mon); err != nil {
		log.Fatalf("State machine error: %v", err)
	}

	log.Println("Hailo device plugin exited successfully")
}
