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

### Quick Start with Makefile

The project includes a Makefile for easy building and deployment:

```bash
# Build, dockerize, and deploy in one command
make all

# Or step by step:
make build                    # Build the binary
make docker-build            # Build Docker image
make deploy                  # Deploy to Kubernetes

# Check status
make status                  # View deployment status
make check-nodes             # Check node Hailo resources
```

### Manual Deployment Steps

#### 1. Build the Binary
```bash
go mod tidy
go build -o hailo-device-plugin
```

#### 2. Build Docker Image
```bash
docker build -t hailo-device-plugin:latest .
```

#### 3. (Optional) Push to Registry
If using a private registry:
```bash
# Tag the image
docker tag hailo-device-plugin:latest your-registry.com/hailo-device-plugin:latest

# Push to registry
docker push your-registry.com/hailo-device-plugin:latest

# Update deploy/daemonset.yaml with your registry URL
```

Or use Makefile:
```bash
make docker-push REGISTRY=your-registry.com IMAGE_TAG=v1.0.0
```

#### 4. Deploy RBAC Resources
```bash
kubectl apply -f deploy/rbac.yaml
```

This creates:
- ServiceAccount: `hailo-device-plugin` in `kube-system` namespace
- ClusterRole with permissions to access nodes and pods
- ClusterRoleBinding to bind the role to the service account

#### 5. Deploy DaemonSet
```bash
kubectl apply -f deploy/daemonset.yaml
```

If using custom registry:
```bash
make deploy-custom REGISTRY=your-registry.com IMAGE_TAG=v1.0.0
```

#### 6. Verify Deployment
```bash
# Check if the DaemonSet is running
kubectl get ds -n kube-system hailo-device-plugin

# Check pod status
kubectl get pods -n kube-system -l app=hailo-device-plugin

# View logs
kubectl logs -n kube-system -l app=hailo-device-plugin -f

# Check device plugin registration
kubectl describe node <node-name> | grep hailo

# Check available resources
kubectl get nodes -o custom-columns=NAME:.metadata.name,HAILO:.status.capacity.hailo\\.ai/npu
```

### Cleanup / Removal

```bash
# Using Makefile
make undeploy

# Or manually
kubectl delete -f deploy/daemonset.yaml
kubectl delete -f deploy/rbac.yaml
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
        hailo.ai/npu: 1
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