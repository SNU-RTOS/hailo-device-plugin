package tests

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"hailo-device-plugin/pkg/cdi"
	"hailo-device-plugin/pkg/plugin"
)

// MockResourceMonitor is a stub monitor for testing
type MockResourceMonitor struct{}

func (m *MockResourceMonitor) Start() {}

func TestPluginWithExistingCDI(t *testing.T) {
	// Create a temporary directory for CDI
	cdiDir, err := os.MkdirTemp("", "cdi-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(cdiDir)

	// Copy the test CDI file
	src := "hailo.json"
	dst := filepath.Join(cdiDir, "hailo.json")
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("Failed to copy CDI file: %v", err)
	}

	// Create plugin with mock monitor
	p := &plugin.HailoDevicePlugin{
		Monitor: nil, // Not used in ListAndWatch
		CdiDir:  cdiDir,
	}

	// Test ReadDevices
	devices, err := cdi.ReadDevices(cdiDir)
	if err != nil {
		t.Fatalf("Failed to read devices: %v", err)
	}
	expected := []string{"hailo0", "hailo1"}
	if len(devices) != len(expected) {
		t.Errorf("Expected %d devices, got %d", len(expected), len(devices))
	}
	for i, exp := range expected {
		if i >= len(devices) || devices[i] != exp {
			t.Errorf("Expected device %s, got %s", exp, devices[i])
		}
	}

	// Plugin creation test (just ensure no panic)
	_ = p
}

// copyFile copies a file from src to dst
func copyFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	return err
}
