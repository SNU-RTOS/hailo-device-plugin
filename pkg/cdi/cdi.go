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
	Name           string         `json:"name"`
	ContainerEdits ContainerEdits `json:"containerEdits"`
}

// ContainerEdits for CDI
type ContainerEdits struct {
	Env         []string      `json:"env,omitempty"`
	DeviceNodes []*DeviceNode `json:"deviceNodes,omitempty"`
	Mounts      []*Mount      `json:"mounts,omitempty"`
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
	HostPath      string `json:"hostPath"`
	ContainerPath string `json:"containerPath"`
}

// GenerateCDI creates a CDI spec file for Hailo devices
// 모니터가 호출, 매 10초마다 디바이스를 발견해서 CDI 스펙을 생성
func GenerateCDI(devices []string, outputDir string) error {
	spec := CDISpec{
		Version: "0.7.0",
		Kind:    "hailo.ai/npu",
		Annotations: map[string]string{
			"vendor":       "Hailo Technologies",
			"description":  "Hailo NPU devices for AI inference acceleration",
			"multi-device": "true",
		},
		Devices: []*DeviceSpec{},
		ContainerEdits: &ContainerEdits{
			Env: []string{"HAILO_LOG_LEVEL=INFO"},
			Mounts: []*Mount{
				{
					HostPath:      "/sys/bus/pci/devices",
					ContainerPath: "/sys/bus/pci/devices",
					// Type and options not in struct, but can add if needed
				},
			},
		},
	}

	for i, dev := range devices {
		major := 507
		minor := i
		spec.Devices = append(spec.Devices, &DeviceSpec{
			Name: fmt.Sprintf("hailo%d", i),
			ContainerEdits: ContainerEdits{
				DeviceNodes: []*DeviceNode{
					{
						Path:        dev,
						HostPath:    dev,
						Type:        "c",
						Major:       major,
						Minor:       minor,
						Permissions: "rw",
					},
				},
				Env: []string{
					fmt.Sprintf("HAILO_DEVICE_ID=%d", i),
					fmt.Sprintf("HAILO_PRIMARY_DEVICE=%s", dev),
				},
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
