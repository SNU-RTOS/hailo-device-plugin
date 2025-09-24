# Hailo Device Plugin for Kubernetes

This is a Kubernetes device plugin for Hailo AI accelerators. It provides the necessary gRPC server to manage Hailo devices in a Kubernetes cluster, with automatic CDI generation via resource monitoring.

## Features

- gRPC server implementing Kubernetes Device Plugin API v1beta1
- Resource monitor for automatic device discovery
- CDI (Container Device Interface) spec generation
- Device allocation using CDI annotations

## Prerequisites

- Go 1.21 or later
- Kubernetes cluster with device plugin support
- CDI support enabled in the cluster

## Building

```bash
go mod tidy
go build -o hailo-device-plugin
```

## Running

The plugin must be run as a privileged container or directly on the node with access to the device plugin socket and CDI directory.

```bash
./hailo-device-plugin
```

## Implementation Notes

- The resource monitor discovers devices and generates CDI specs every 30 seconds.
- `ListAndWatch` reads the current CDI spec to report available devices to kubelet.
- `Allocate` uses CDI annotations for device allocation.
- Implement actual Hailo device discovery in `pkg/monitor/monitor.go` `discoverDevices` method.
- CDI specs are generated in `/etc/cdi/` by default.
- Ensure the socket path `/var/lib/kubelet/device-plugins/hailo.sock` and CDI directory are accessible.

## API Reference

Implements the Kubernetes Device Plugin API v1beta1:
- `GetDevicePluginOptions`
- `ListAndWatch`
- `Allocate`
- `GetPreferredAllocation`
- `PreStartContainer`