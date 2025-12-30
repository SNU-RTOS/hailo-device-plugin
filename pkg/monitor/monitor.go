package monitor

import (
	"context"
	"log"
	"os/exec"
	"strings"
	"time"

	"hailo-device-plugin/pkg/cdi"
)

// ResourceMonitor monitors Hailo devices and updates CDI
type ResourceMonitor struct {
	cdiDir string
}

// NewResourceMonitor creates a new monitor
func NewResourceMonitor(cdiDir string) *ResourceMonitor {
	return &ResourceMonitor{cdiDir: cdiDir}
}

// Start begins monitoring devices with context support
func (m *ResourceMonitor) Start(ctx context.Context) {
	go func() {
		// Generate CDI immediately on startup
		devices := m.discoverDevices()
		log.Printf("Initial device discovery: %v", devices)
		if err := cdi.GenerateCDI(devices, m.cdiDir); err != nil {
			log.Printf("Failed to generate initial CDI: %v", err)
		} else {
			log.Println("Initial CDI generated")
		}

		ticker := time.NewTicker(60 * time.Second) // Check every 60 seconds
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				devices := m.discoverDevices()
				log.Printf("Discovered devices: %v", devices)
				if err := cdi.GenerateCDI(devices, m.cdiDir); err != nil {
					log.Printf("Failed to generate CDI: %v", err)
				} else {
					log.Println("CDI updated")
				}
			case <-ctx.Done():
				log.Println("Monitor stopping due to context cancellation")
				return
			}
		}
	}()
}

// discoverDevices simulates device discovery (replace with actual Hailo API)
func (m *ResourceMonitor) discoverDevices() []string {
	// Placeholder: Return list of device paths
	output, err := exec.Command("ls", "/sys/class/hailo_chardev").Output()
	if err != nil {
		log.Printf("Failed to list devices: %v", err)
		return nil
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n")
}
