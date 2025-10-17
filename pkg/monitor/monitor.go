package monitor

import (
	"log"
	"time"
	"os/exec"
	"strings"
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

// Start begins monitoring devices
func (m *ResourceMonitor) Start() {
	go func() {
		ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
		defer ticker.Stop()

		for range ticker.C {
			devices := m.discoverDevices() // Implement device discovery
			log.Printf("Discovered devices: %v", devices)
			if err := cdi.GenerateCDI(devices, m.cdiDir); err != nil {
				log.Printf("Failed to generate CDI: %v", err)
			} else {
				log.Println("CDI updated")
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
