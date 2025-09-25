package main

import (
	"log"
	"net"
	"os"
	"path"
	"time"

	"hailo-device-plugin/pkg/monitor"
	"hailo-device-plugin/pkg/plugin"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

const (
	socketPath = "/var/lib/kubelet/device-plugins/hailo.sock"
)

func main() {
	cdiDir := "/etc/cdi" // Or appropriate directory
	if err := os.MkdirAll(cdiDir, 0755); err != nil {
		log.Fatalf("failed to create CDI directory: %v", err)
	}

	// Start resource monitor
	mon := monitor.NewResourceMonitor(cdiDir)
	mon.Start()

	// Create plugin with monitor
	hailoPlugin := &plugin.HailoDevicePlugin{
		Monitor:      mon,
		CdiDir:       cdiDir,
		SocketPath:   socketPath,
		ResourceName: "hailo.ai/npu",
	}

	if err := os.MkdirAll(path.Dir(socketPath), 0755); err != nil {
		log.Fatalf("failed to create socket directory: %v", err)
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Fatalf("failed to remove existing socket: %v", err)
	}

	lis, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	defer lis.Close()

	s := grpc.NewServer()
	pluginapi.RegisterDevicePluginServer(s, hailoPlugin)

	// Start the gRPC server in a goroutine
	go func() {
		log.Println("Starting Hailo device plugin server...")
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Give the server a moment to start
	time.Sleep(1 * time.Second)

	// Register with kubelet
	if err := hailoPlugin.Start(); err != nil {
		log.Fatalf("failed to register with kubelet: %v", err)
	}

	// Keep the main goroutine alive
	select {}
}
