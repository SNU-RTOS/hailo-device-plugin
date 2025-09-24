package plugin

import (
	"context"
	"log"
	"path/filepath"

	"hailo-device-plugin/pkg/cdi"
	"hailo-device-plugin/pkg/monitor"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type HailoDevicePlugin struct {
	Monitor *monitor.ResourceMonitor
	CdiDir  string
}

var _ pluginapi.DevicePluginServer = (*HailoDevicePlugin)(nil)

func (p *HailoDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (p *HailoDevicePlugin) ListAndWatch(_ *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	// Read devices from CDI
	devices, err := cdi.ReadDevices(p.CdiDir)
	if err != nil {
		log.Printf("Failed to read devices from CDI: %v", err)
		// Fallback to empty list or handle error
		devices = []string{}
	}

	var pluginDevices []*pluginapi.Device
	for _, id := range devices {
		pluginDevices = append(pluginDevices, &pluginapi.Device{
			ID:     id,
			Health: pluginapi.Healthy,
		})
	}

	log.Println("Sending device list:", pluginDevices)
	return server.Send(&pluginapi.ListAndWatchResponse{Devices: pluginDevices})
}

func (p *HailoDevicePlugin) Allocate(ctx context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	// Placeholder: Allocate hailo devices using CDI
	var response pluginapi.AllocateResponse
	cdiPath := filepath.Join(p.CdiDir, "hailo.json")
	for range req.ContainerRequests {
		containerResponse := &pluginapi.ContainerAllocateResponse{
			// Use CDI for device allocation
			Annotations: map[string]string{
				"cdi.k8s.io/hailo": cdiPath,
			},
			// Add other allocations as needed
		}
		response.ContainerResponses = append(response.ContainerResponses, containerResponse)
	}
	log.Println("Allocated devices for request:", req)
	return &response, nil
}

func (p *HailoDevicePlugin) PreStartContainer(context.Context, *pluginapi.PreStartContainerRequest) (*pluginapi.PreStartContainerResponse, error) {
	// Optional: Implement pre-start logic if needed
	return &pluginapi.PreStartContainerResponse{}, nil
}

func (p *HailoDevicePlugin) GetPreferredAllocation(context.Context, *pluginapi.PreferredAllocationRequest) (*pluginapi.PreferredAllocationResponse, error) {
	// Optional: Implement preferred allocation logic if needed
	return &pluginapi.PreferredAllocationResponse{}, nil
}
