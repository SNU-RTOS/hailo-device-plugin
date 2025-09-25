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

## Deployment

### 1. Build the binary
```bash
go build -o hailo-device-plugin
```

### 2. Build Docker image
```bash
docker build -t hailo-device-plugin:latest .
```

### 3. Deploy to Kubernetes cluster
```bash
kubectl apply -f deploy/daemonset.yaml
```

### 4. Verify deployment
```bash
# Check if the DaemonSet is running
kubectl get ds -n kube-system hailo-device-plugin

# Check device plugin registration
kubectl describe node <node-name> | grep hailo

# Check available resources
kubectl get nodes -o yaml | grep hailo
```

## Usage in Pods

Once deployed, you can request Hailo devices in your pod specifications:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: hailo-test
spec:
  containers:
  - name: hailo-container
    image: ubuntu:20.04
    resources:
      limits:
        hailo.ai/device: 1
    command: ["/bin/bash"]
    args: ["-c", "while true; do sleep 30; done;"]
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

## TODO

- [ ] has to implement monitor using hailortcli