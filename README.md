# Hailo Device Plugin for Kubernetes

This is a Kubernetes device plugin for Hailo AI accelerators. It provides the necessary gRPC server to manage Hailo devices in a Kubernetes cluster, with automatic CDI generation via resource monitoring.

## Features

- gRPC server implementing Kubernetes Device Plugin API v1beta1
- Resource monitor for automatic device discovery
- CDI (Container Device Interface) spec generation
- Device allocation using CDI annotations

## Prerequisites

- Kubernetes cluster with device plugin support
- containerd 1.7 or later (1.7.x or 2.0+)
- **Recommended**: containerd 2.0+ with K3s v1.31.6+
- CDI (Container Device Interface) support enabled in the container runtime

### Enabling CDI Support

**Note**: containerd 2.0+ enables CDI by default. The following configuration is only required for containerd 1.7.x.

For detailed CDI configuration, refer to the [official CDI documentation](https://github.com/cncf-tags/container-device-interface#how-to-configure-cdi).

**For K3s with containerd 1.7.x:**

Edit `/var/lib/rancher/k3s/agent/etc/containerd/config.toml.tmpl` and add:

```toml
[plugins."io.containerd.grpc.v1.cri"]
  enable_cdi = true
  cdi_spec_dirs = ["/etc/cdi", "/var/run/cdi"]
```

Then restart K3s:
```bash
sudo systemctl restart k3s
```

## Quick Start

Deploy the Hailo device plugin with a single command:

```bash
kubectl create -f https://raw.githubusercontent.com/SNU-RTOS/hailo-device-plugin/main/deploy/hailo-device-plugin.yaml
```

Verify the deployment:

```bash
# Check DaemonSet status
kubectl get ds -n kube-system hailo-device-plugin

# Check available Hailo devices on nodes
kubectl get nodes -o custom-columns=NAME:.metadata.name,HAILO:.status.capacity.hailo\\.ai/npu
```

Test with a pod:

```bash
kubectl run hailo-test --image=ghcr.io/snu-rtos/hailort:4.23.0-runtime-amd64 \
  --restart=Never --rm -it \
  --overrides='{"spec":{"containers":[{"name":"hailo-test","image":"ghcr.io/snu-rtos/hailort:4.23.0-runtime-amd64","command":["hailortcli","scan"],"resources":{"limits":{"hailo.ai/npu":"1"}}}]}}' \
  -- hailortcli scan
```

Expected output showing detected Hailo devices:

```
Hailo Devices:
[0] PCIe: 0000:01:00.0
```

If use want to check in pod's bash shell, try this command

```bash
kubectl run hailo-test --image=ghcr.io/snu-rtos/hailort:4.23.0-runtime-amd64 or \
kubectl run hailo-test --image=ghcr.io/snu-rtos/hailort:4.23.0-runtime-arm64 \
  --restart=Never --rm -it \
  --overrides='{
    "spec": {
      "containers": [
        {
          "name": "hailo-test",
          "image": "ghcr.io/snu-rtos/hailort:4.23.0-runtime-arm64",
          "command": ["/bin/bash"],
          "stdin": true,
          "tty": true,
          "resources": {
            "limits": {
              "hailo.ai/npu": "1"
            }
          }
        }
      ]
    }
  }' \
  -- /bin/bash
```

## Building and Deployment

### Using Makefile

```bash
# Build and push multi-architecture image (amd64 + arm64)
make docker-build-multiarch

# Or build locally for single architecture
make build                    # Build the binary
make docker-build            # Build Docker image
make deploy                  # Deploy to Kubernetes

# Check status
make status                  # View deployment status
make check-nodes             # Check node Hailo resources
```

### Manual Steps

For custom builds or modifications:

```bash
# 1. Build binary
go mod tidy
go build -o hailo-device-plugin

# 2. Build and push Docker image
docker build -t your-registry.com/hailo-device-plugin:latest .
docker push your-registry.com/hailo-device-plugin:latest

# 3. Deploy with custom image
kubectl apply -f deploy/hailo-device-plugin.yaml
# (Update image in deploy/hailo-device-plugin.yaml if needed)
```

### Cleanup

```bash
make undeploy
# or
kubectl delete -f deploy/hailo-device-plugin.yaml
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
    image: ghcr.io/snu-rtos/hailort:4.23.0-runtime-arm64  # Use amd64 for x86 architecture
    resources:
      limits:
        hailo.ai/npu: 1
    command: ["hailortcli", "scan"]  # Or use ["/bin/bash", "-c", "while true; do sleep 30; done;"] for long-running pod
```

**Note**: Change the image tag to match your architecture:
- `ghcr.io/snu-rtos/hailort:4.23.0-runtime-amd64` for x86_64/AMD64
- `ghcr.io/snu-rtos/hailort:4.23.0-runtime-arm64` for ARM64/aarch64

## Cleanup of sysfs mountpoints

The CDI spec hides the host's NPUs by bind-mounting an empty directory
(`/var/lib/hailo-cdi/empty-chardev`) over `/sys/class/hailo_chardev` and
overlaying only the allocated device's sysfs directory. The container runtime
creates a mountpoint directory for that overlay, which persists after the
container exits. Two mechanisms remove it:

- **Poststop hook.** Each device in the spec carries a `poststop` hook that
  removes that device's mountpoint when its container stops. The hook is a
  static binary (`hailo-cdi-hook`, built from `cmd/hailo-cdi-hook`) that the
  plugin installs to `/var/lib/hailo-cdi/` at startup, so it works on hosts
  without a shell. It refuses to remove a directory that is still a live
  mountpoint in another container (e.g. after a crash-restart that reused the
  device).
- **Sweep.** At startup and on every `Allocate`, the plugin removes leftover
  mountpoints for devices the kubelet does not report as assigned to a pod,
  via the kubelet PodResources API. This requires the
  `/var/lib/kubelet/pod-resources` hostPath mount in the DaemonSet (included
  in `deploy/hailo-device-plugin.yaml`). Without it the plugin logs
  `stale mountpoint sweep skipped` and continues without sweeping.

A pod that sees a leftover mountpoint fails `hailortcli scan` with
`Failed open /sys/class/hailo_chardev/<dev>/board_location` and
`HAILO_DRIVER_INVALID_RESPONSE(85)`; that error indicates a stale directory,
not a driver problem.

## Read-only hosts (Talos Linux)

On hosts where `/etc` is read-only, containerd cannot use its default CDI
spec directory `/etc/cdi`. Point containerd at a writable location with a
machine config patch:

```yaml
machine:
  files:
    - path: /etc/cri/conf.d/20-customization.part
      op: create
      content: |
        [plugins."io.containerd.cri.v1.runtime"]
          cdi_spec_dirs = ["/var/cdi/static", "/var/cdi/dynamic"]
```

(`[plugins."io.containerd.grpc.v1.cri"]` on containerd 1.7.) Apply with
`talosctl patch mc --mode=reboot`, then change the `cdi` hostPath in the
DaemonSet from `/etc/cdi` to `/var/cdi/dynamic`. The plugin itself keeps
writing to `/etc/cdi` inside its container; the hostPath mapping does the
rest.

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

- [ ] Implement detailed resource monitor by using hailortcli
