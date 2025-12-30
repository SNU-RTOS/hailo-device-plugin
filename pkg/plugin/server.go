package plugin

import (
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"google.golang.org/grpc"
	pluginapi "k8s.io/kubelet/pkg/apis/deviceplugin/v1beta1"
)

// Server manages the gRPC server lifecycle for the device plugin
type Server struct {
	grpcServer *grpc.Server
	listener   net.Listener
	socketPath string
	plugin     *HailoDevicePlugin
	serveDone  chan error
}

// NewServer creates a new gRPC server for the device plugin
func NewServer(plugin *HailoDevicePlugin, socketPath string) (*Server, error) {
	return &Server{
		plugin:     plugin,
		socketPath: socketPath,
		serveDone:  make(chan error, 1),
	}, nil
}

// Start creates the socket, sets up the gRPC server, and starts serving
func (s *Server) Start() error {
	// Remove stale socket if it exists
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove existing socket: %w", err)
	}

	// Create Unix domain socket
	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("failed to create socket listener: %w", err)
	}
	s.listener = listener

	// Create gRPC server and register plugin
	s.grpcServer = grpc.NewServer()
	pluginapi.RegisterDevicePluginServer(s.grpcServer, s.plugin)

	// Start serving in a goroutine
	go func() {
		log.Printf("Starting gRPC server on %s", s.socketPath)
		if err := s.grpcServer.Serve(s.listener); err != nil {
			log.Printf("gRPC server error: %v", err)
			s.serveDone <- err
		} else {
			s.serveDone <- nil
		}
	}()

	log.Println("gRPC server started successfully")
	return nil
}

// Done returns a channel that receives an error when the server stops
func (s *Server) Done() <-chan error {
	return s.serveDone
}

// Stop stops the gRPC server gracefully with a timeout
func (s *Server) Stop() error {
	if s.grpcServer == nil {
		return nil // Already stopped
	}

	log.Println("Stopping gRPC server gracefully...")

	// Try graceful stop with timeout
	stopped := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(stopped)
	}()

	select {
	case <-stopped:
		log.Println("gRPC server stopped gracefully")
	case <-time.After(5 * time.Second):
		log.Println("Graceful stop timeout, forcing stop")
		s.grpcServer.Stop()
	}

	// Close listener
	if s.listener != nil {
		if err := s.listener.Close(); err != nil {
			log.Printf("Failed to close listener: %v", err)
		}
	}

	// Remove socket file
	if err := os.Remove(s.socketPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to remove socket: %v", err)
	}

	s.grpcServer = nil
	s.listener = nil

	log.Println("gRPC server cleanup complete")
	return nil
}
