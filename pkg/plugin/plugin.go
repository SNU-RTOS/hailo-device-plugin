package plugin

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"hailo-device-plugin/pkg/cdi"
	"hailo-device-plugin/pkg/monitor"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

type HailoDevicePlugin struct {
	Monitor      *monitor.ResourceMonitor
	CdiDir       string
	SocketPath   string
	ResourceName string
}

const (
	kubeletEndpoint = "/var/lib/kubelet/device-plugins/kubelet.sock"
	defaultResourceName = "hailo.ai/npu"
)

// Register registers the device plugin with the kubelet
func (p *HailoDevicePlugin) Register() error {
	// Check if kubelet socket exists
	log.Printf("Checking kubelet socket at: %s", kubeletEndpoint)
	if _, err := os.Stat(kubeletEndpoint); os.IsNotExist(err) {
		return fmt.Errorf("kubelet socket not found at %s", kubeletEndpoint)
	}
	log.Println("Kubelet socket found, attempting to connect...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "unix://"+kubeletEndpoint, 
		grpc.WithInsecure(), 
		grpc.WithBlock())
	if err != nil {
		return fmt.Errorf("failed to connect to kubelet: %v", err)
	}
	defer conn.Close()
	
	log.Println("Connected to kubelet, sending registration request...")

	client := pluginapi.NewRegistrationClient(conn)
	req := &pluginapi.RegisterRequest{
		Version:      pluginapi.Version,
		Endpoint:     filepath.Base(p.SocketPath),
		ResourceName: p.ResourceName,
	}

	log.Printf("Registering with kubelet: Version=%s, Endpoint=%s, ResourceName=%s", 
		req.Version, req.Endpoint, req.ResourceName)

	_, err = client.Register(context.Background(), req)
	if err != nil {
		return fmt.Errorf("registration failed: %v", err)
	}

	log.Println("Device plugin registered successfully")
	return nil
}

// Start starts the device plugin
func (p *HailoDevicePlugin) Start() error {
	err := p.Register()
	if err != nil {
		return err
	}
	
	// Keep checking kubelet connection and re-register if needed
	go func() {
		for {
			time.Sleep(30 * time.Second)
			if err := p.Register(); err != nil {
				log.Printf("Failed to re-register with kubelet: %v", err)
			}
		}
	}()
	
	return nil
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
	
	for _, containerReq := range req.ContainerRequests {
		log.Printf("Processing container request for %d devices: %v", len(containerReq.DevicesIDs), containerReq.DevicesIDs)
		
		// Build CDI device names for requested devices
		var cdiDevices []string
		for _, deviceID := range containerReq.DevicesIDs {
			// CDI device name format: hailo.ai/npu=<device-name>
			cdiDeviceName := fmt.Sprintf("hailo.ai/npu=%s", deviceID)
			cdiDevices = append(cdiDevices, cdiDeviceName)
		}
		
		containerResponse := &pluginapi.ContainerAllocateResponse{
			// Use CDI device names in annotations
			Annotations: map[string]string{
				"cdi.k8s.io/hailo": strings.Join(cdiDevices, ","),
			},
		}
		
		log.Printf("Allocated CDI devices: %v", cdiDevices)
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
