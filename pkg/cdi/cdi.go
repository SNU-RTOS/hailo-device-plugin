package cdi

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// CDISpec represents a basic CDI spec structure
type CDISpec struct {
	Version        string            `json:"cdiVersion"`
	Kind           string            `json:"kind"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	Devices        []*DeviceSpec     `json:"devices"`
	ContainerEdits *ContainerEdits   `json:"containerEdits,omitempty"`
}

// DeviceSpec represents a device in CDI
type DeviceSpec struct {
	Name           string            `json:"name"`
	Annotations    map[string]string `json:"annotations,omitempty"`
	ContainerEdits ContainerEdits    `json:"containerEdits"`
}

// ContainerEdits for CDI
type ContainerEdits struct {
	Env         []string      `json:"env,omitempty"`
	DeviceNodes []*DeviceNode `json:"deviceNodes,omitempty"`
	Mounts      []*Mount      `json:"mounts,omitempty"`
	Hooks       []*Hook       `json:"hooks,omitempty"`
}

// DeviceNode for CDI
type DeviceNode struct {
	Path        string `json:"path"`
	HostPath    string `json:"hostPath,omitempty"`
	Type        string `json:"type,omitempty"`
	Major       int    `json:"major,omitempty"`
	Minor       int    `json:"minor,omitempty"`
	Permissions string `json:"permissions,omitempty"`
}

// Mount for CDI
type Mount struct {
	HostPath      string   `json:"hostPath"`
	ContainerPath string   `json:"containerPath"`
	Options       []string `json:"options,omitempty"`
}

// Hook for CDI lifecycle hooks
type Hook struct {
	HookName string   `json:"hookName"`
	Path     string   `json:"path"`
	Args     []string `json:"args,omitempty"`
	Env      []string `json:"env,omitempty"`
	Timeout  int      `json:"timeout,omitempty"`
}


// resolveSysfsPath verifies that the device path exists
func resolveSysfsPath(deviceID string) error {
	symlinkPath := filepath.Join("/sys/class/hailo_chardev", deviceID)

	// Verify the symlink exists
	_, err := os.Stat(symlinkPath)
	if err != nil {
		return fmt.Errorf("device path %s does not exist: %w", symlinkPath, err)
	}

	return nil
}

// createDeviceSpecificSysfsMounts creates mount configurations for device-specific sysfs
// This ensures the container only sees the assigned Hailo device in /sys/class/hailo_chardev/
func createDeviceSpecificSysfsMounts(deviceID string) ([]*Mount, error) {
	err := resolveSysfsPath(deviceID)
	if err != nil {
		return nil, err
	}

	mounts := []*Mount{
		// Mount only the specific device into /sys/class/hailo_chardev/<deviceID>
		// The parent directory (/sys/class/hailo_chardev) is already mounted as empty
		// from the global containerEdits, so this will overlay just this device
		{
			HostPath:      fmt.Sprintf("/sys/class/hailo_chardev/%s", deviceID),
			ContainerPath: fmt.Sprintf("/sys/class/hailo_chardev/%s", deviceID),
			Options:       []string{"ro", "bind"},
		},
	}

	return mounts, nil
}

// GenerateCDI creates a CDI spec file for Hailo devices
// 모니터가 호출, 매 10초마다 디바이스를 발견해서 CDI 스펙을 생성
func GenerateCDI(devices []string, outputDir string) error {

	spec := CDISpec{
		Version: "0.5.0",
		Kind:    "hailo.ai/npu",
		Annotations: map[string]string{
			"vendor":       "Hailo Technologies",
			"description":  "Hailo NPU devices for AI inference acceleration",
			"multi-device": "true",
		},
		Devices: []*DeviceSpec{},
		// Global container edits applied to all devices
		ContainerEdits: &ContainerEdits{
			Mounts: []*Mount{
				{
					// Mount empty directory to /sys/class/hailo_chardev with rw
					// This hides all devices initially and allows mounting devices inside
					HostPath:      "/var/lib/hailo-cdi/empty-chardev",
					ContainerPath: "/sys/class/hailo_chardev",
					Options:       []string{"rw", "bind"},
				},
			},
			Hooks: []*Hook{
				{
					HookName: "poststop",
					Path:     "/var/lib/hailo-cdi/cleanup-empty-chardev.sh",
					Args:     []string{},
					Timeout:  5,
				},
			},
		},
	}

	// Individual devices
	for _, dev := range devices {
		// Create device-specific sysfs mounts to isolate this device
		sysfsMounts, err := createDeviceSpecificSysfsMounts(dev)
		if err != nil {
			// Log warning but continue - device will still work without sysfs isolation
			fmt.Fprintf(os.Stderr, "Warning: failed to create sysfs mounts for %s: %v\n", dev, err)
			sysfsMounts = []*Mount{}
		}

		spec.Devices = append(spec.Devices, &DeviceSpec{
			Name: dev,
			Annotations: map[string]string{
				"device.type":  "npu",
				"device.model": "hailo-8",
				"pci.slot":     "auto-detect",
			},
			ContainerEdits: ContainerEdits{
				DeviceNodes: []*DeviceNode{
					{
						Path:        fmt.Sprintf("/dev/%s", dev),
						HostPath:    fmt.Sprintf("/dev/%s", dev),
						Type:        "c",
						Permissions: "rw",
					},
				},
				Mounts: sysfsMounts,
			},
		})
	}

	data, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}

	cdiFile := filepath.Join(outputDir, "hailo.json")
	return os.WriteFile(cdiFile, data, 0644)
}

// ReadDevices reads the CDI spec and returns the list of device IDs
func ReadDevices(cdiDir string) ([]string, error) {
	cdiFile := filepath.Join(cdiDir, "hailo.json")
	data, err := os.ReadFile(cdiFile)
	if err != nil {
		return nil, err
	}

	var spec CDISpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}

	var devices []string
	for _, dev := range spec.Devices {
		devices = append(devices, dev.Name)
	}
	return devices, nil
}
