package monitor

import (
	"log"
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

// Start begins monitoring devices
func (m *ResourceMonitor) Start() {
	go func() {
		ticker := time.NewTicker(10 * time.Second) // Check every 10 seconds
		defer ticker.Stop()

		for range ticker.C {
			devices := m.discoverDevices() // Implement device discovery
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
	return []string{"/dev/hailo0", "/dev/hailo1"}
}
