package plugin

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"hailo-device-plugin/pkg/cdi"
	"hailo-device-plugin/pkg/cleanup"

	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type HailoDevicePlugin struct {
	CdiDir       string
	SocketPath   string
	ResourceName string
	// Sweeper removes stale sysfs mountpoints before each allocation.
	// Optional; nil disables the sweep (used in tests).
	Sweeper *cleanup.Sweeper
}

var _ pluginapi.DevicePluginServer = (*HailoDevicePlugin)(nil)

func (p *HailoDevicePlugin) GetDevicePluginOptions(context.Context, *pluginapi.Empty) (*pluginapi.DevicePluginOptions, error) {
	return &pluginapi.DevicePluginOptions{}, nil
}

func (p *HailoDevicePlugin) ListAndWatch(_ *pluginapi.Empty, server pluginapi.DevicePlugin_ListAndWatchServer) error {
	log.Printf("ListAndWatch called, reading devices from CDI dir: %s", p.CdiDir)

	// Send initial device list
	if err := p.sendDeviceList(server); err != nil {
		return err
	}

	// Keep the stream alive and periodically check for device changes
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			log.Println("Periodic device list update")
			if err := p.sendDeviceList(server); err != nil {
				log.Printf("Failed to send periodic device list update: %v", err)
				return err
			}
		case <-server.Context().Done():
			log.Println("ListAndWatch stream closed")
			return server.Context().Err()
		}
	}
}

func (p *HailoDevicePlugin) sendDeviceList(server pluginapi.DevicePlugin_ListAndWatchServer) error {
	// Read devices from CDI
	devices, err := cdi.ReadDevices(p.CdiDir)
	if err != nil {
		log.Printf("Failed to read devices from CDI: %v", err)
		// Fallback to empty list or handle error
		devices = []string{}
	}

	log.Printf("Found %d devices from CDI: %v", len(devices), devices)

	var pluginDevices []*pluginapi.Device
	for _, id := range devices {
		device := &pluginapi.Device{
			ID:     id,
			Health: pluginapi.Healthy,
		}
		pluginDevices = append(pluginDevices, device)
		log.Printf("Added device: ID=%s, Health=%s", device.ID, device.Health)
	}

	log.Printf("Sending %d devices to kubelet: %v", len(pluginDevices), pluginDevices)

	response := &pluginapi.ListAndWatchResponse{Devices: pluginDevices}

	if err := server.Send(response); err != nil {
		log.Printf("Failed to send device list: %v", err)
		return err
	}

	log.Println("Successfully sent device list to kubelet")
	return nil
}

func (p *HailoDevicePlugin) Allocate(ctx context.Context, req *pluginapi.AllocateRequest) (*pluginapi.AllocateResponse, error) {
	log.Printf("Allocate called with request: %v", req)

	var response pluginapi.AllocateResponse

	// Remove stale sysfs mountpoints for devices no longer assigned to any
	// pod, so the containers created from this allocation don't see them.
	// Never fails the allocation: a skipped sweep at worst leaves a visible
	// scan error in the pod, whereas a failed Allocate is not retried.
	if p.Sweeper != nil {
		var ids []string
		for _, cr := range req.ContainerRequests {
			ids = append(ids, cr.DevicesIDs...)
		}
		if err := p.Sweeper.Sweep(ctx, ids); err != nil {
			log.Printf("Warning: stale mountpoint sweep skipped: %v", err)
		}
	}

	for _, containerReq := range req.ContainerRequests {
		log.Printf("Processing container request for %d devices: %v", len(containerReq.DevicesIDs), containerReq.DevicesIDs)

		// Build CDI device names for requested devices.
		var cdiDeviceNames []string
		var cdiDevices []*pluginapi.CDIDevice
		for _, deviceID := range containerReq.DevicesIDs {
			cdiDeviceName := fmt.Sprintf("hailo.ai/npu=%s", deviceID)
			cdiDeviceNames = append(cdiDeviceNames, cdiDeviceName)
			cdiDevices = append(cdiDevices, &pluginapi.CDIDevice{Name: cdiDeviceName})
		}

		// Send the same set via both the legacy CDI annotation (older
		// kubelet/containerd) and the typed CDIDevices field (kubelet
		// v1.27+ with containerd v1.7+/v2.x). Modern toolchains read
		// CDIDevices and ignore the annotation; older ones do the
		// reverse — keeping both makes the plugin work on both.
		containerResponse := &pluginapi.ContainerAllocateResponse{
			Annotations: map[string]string{
				"cdi.k8s.io/hailo": strings.Join(cdiDeviceNames, ","),
			},
			CDIDevices: cdiDevices,
		}

		log.Printf("Allocated CDI devices: %v", cdiDeviceNames)
		response.ContainerResponses = append(response.ContainerResponses, containerResponse)
	}

	log.Printf("Allocation response: %v", response)
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
