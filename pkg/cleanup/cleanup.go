// Package cleanup removes stale per-device mountpoint directories that the
// container runtime leaves behind in the shared "empty-chardev" directory.
//
// Background: the CDI spec bind-mounts an empty host directory over
// /sys/class/hailo_chardev to hide all NPUs, then bind-mounts the allocated
// device's real sysfs directory on top. To create that inner mountpoint runc
// does mkdir on the (rw) host directory, and the poststop hook that is meant
// to remove it again is a shell script that cannot run on shell-less hosts
// such as Talos Linux. The directory then persists and is visible to the next
// pod, whose HailoRT scan aborts on the empty, attribute-less directory.
//
// This package performs that cleanup from the plugin itself, on every
// Allocate call and at startup. It consults the kubelet PodResources API to
// learn which devices are currently assigned to pods and never removes the
// directory of a device in use. That check is not optional: rmdir on a
// directory that is a live bind-mount target in another mount namespace
// succeeds and lazily detaches that mount, which would pull
// /sys/class/hailo_chardev/<dev> out from under a running container.
package cleanup

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	podresourcesv1 "k8s.io/kubelet/pkg/apis/podresources/v1"
)

const (
	// DefaultPodResourcesSocket is the kubelet PodResources endpoint. Mount
	// /var/lib/kubelet/pod-resources from the host into the plugin container.
	DefaultPodResourcesSocket = "/var/lib/kubelet/pod-resources/kubelet.sock"

	// DefaultEmptyChardevDir is the host directory the CDI spec bind-mounts
	// over /sys/class/hailo_chardev, as seen inside the plugin container.
	DefaultEmptyChardevDir = "/var/lib/hailo-cdi/empty-chardev"

	podResourcesTimeout = 5 * time.Second
)

// Sweeper removes stale mountpoint directories from EmptyChardevDir.
type Sweeper struct {
	// ResourceName is the extended resource the plugin advertises, e.g. "hailo.ai/npu".
	ResourceName string
	// PodResourcesSocket is the path to the kubelet PodResources unix socket.
	PodResourcesSocket string
	// EmptyChardevDir is the directory holding one subdirectory per device
	// that has ever been mounted into a container.
	EmptyChardevDir string
}

// New returns a Sweeper with default paths for the given resource name.
func New(resourceName string) *Sweeper {
	return &Sweeper{
		ResourceName:       resourceName,
		PodResourcesSocket: DefaultPodResourcesSocket,
		EmptyChardevDir:    DefaultEmptyChardevDir,
	}
}

// Sweep removes every empty subdirectory of EmptyChardevDir whose name is a
// device ID that is neither assigned to a pod on this node nor listed in keep.
//
// keep should contain the device IDs of the allocation in progress: the
// kubelet records an allocation before the container is created, so those IDs
// normally already appear in PodResources, but passing them explicitly closes
// any window between the two.
//
// Sweep fails closed. If the kubelet cannot be queried, nothing is removed and
// the error is returned; callers in the Allocate path should log it and
// proceed with the allocation rather than fail the pod.
func (s *Sweeper) Sweep(ctx context.Context, keep []string) error {
	inUse, err := s.devicesInUse(ctx)
	if err != nil {
		return fmt.Errorf("querying kubelet pod resources: %w", err)
	}
	for _, id := range keep {
		inUse[id] = struct{}{}
	}

	entries, err := os.ReadDir(s.EmptyChardevDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading %s: %w", s.EmptyChardevDir, err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if _, ok := inUse[name]; ok {
			continue
		}
		p := filepath.Join(s.EmptyChardevDir, name)
		// os.Remove only removes a directory if it is empty. A stale
		// mountpoint is always empty, so anything non-empty is not ours.
		if err := os.Remove(p); err != nil {
			log.Printf("cleanup: leaving %s in place: %v", p, err)
			continue
		}
		log.Printf("cleanup: removed stale mountpoint %s", p)
	}
	return nil
}

// devicesInUse returns the set of device IDs of ResourceName currently
// assigned to any pod on this node, as reported by the kubelet.
func (s *Sweeper) devicesInUse(ctx context.Context) (map[string]struct{}, error) {
	ctx, cancel := context.WithTimeout(ctx, podResourcesTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, "unix://"+s.PodResourcesSocket,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	client := podresourcesv1.NewPodResourcesListerClient(conn)
	resp, err := client.List(ctx, &podresourcesv1.ListPodResourcesRequest{})
	if err != nil {
		return nil, err
	}

	inUse := make(map[string]struct{})
	for _, pod := range resp.GetPodResources() {
		for _, c := range pod.GetContainers() {
			for _, d := range c.GetDevices() {
				if d.GetResourceName() != s.ResourceName {
					continue
				}
				for _, id := range d.GetDeviceIds() {
					inUse[id] = struct{}{}
				}
			}
		}
	}
	return inUse, nil
}
