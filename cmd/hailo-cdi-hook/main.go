// hailo-cdi-hook is an OCI runtime hook executed by the container runtime
// (runc/crun) on the host after a container has stopped. It removes the
// empty mountpoint directory the runtime created inside the plugin's shared
// "empty-chardev" directory when it bind-mounted the allocated device's sysfs
// directory into the container.
//
// It replaces the original cleanup-empty-chardev.sh, which cannot run on
// hosts without a shell (Talos Linux, among others). Build it static
// (CGO_ENABLED=0) so it runs on any host regardless of libc.
//
// Usage, as placed in a CDI device's containerEdits.hooks:
//
//	{
//	  "hookName": "poststop",
//	  "path": "/var/lib/hailo-cdi/hailo-cdi-hook",
//	  "args": ["hailo-cdi-hook", "/var/lib/hailo-cdi/empty-chardev/hailo1"],
//	  "timeout": 5
//	}
//
// The runtime passes the container state as JSON on stdin; it is drained and
// otherwise ignored. The hook always exits 0 once the directory is gone, and
// exits non-zero only for a genuinely unexpected failure, so a missing
// directory (already cleaned, or never created) is not an error.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// baseDir is the only directory this hook will ever remove entries from.
// Anything outside it is refused, so a mistaken or hostile spec cannot turn
// the hook into a generic rmdir running as root on the host.
const baseDir = "/var/lib/hailo-cdi/empty-chardev"

func main() {
	// Drain stdin so the runtime never blocks writing the container state.
	_, _ = io.Copy(io.Discard, os.Stdin)

	if len(os.Args) != 2 {
		fmt.Fprintf(os.Stderr, "usage: %s <mountpoint-dir>\n", filepath.Base(os.Args[0]))
		os.Exit(2)
	}

	target, err := safeTarget(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "hailo-cdi-hook: refusing %q: %v\n", os.Args[1], err)
		os.Exit(2)
	}

	// If a container crashes and the kubelet restarts it, the new container
	// reuses the same device and may already have mounted its sysfs directory
	// onto this path before our poststop runs. rmdir on a live mountpoint in
	// another mount namespace succeeds and lazily detaches that mount, so
	// check /proc/*/mountinfo first. The old container's processes are gone
	// by poststop time, so any hit belongs to a new, legitimate user.
	if mountedSomewhere(filepath.Base(target)) {
		fmt.Fprintf(os.Stderr, "hailo-cdi-hook: %s is in use by another container, leaving it\n", target)
		return
	}

	// os.Remove on a directory succeeds only if it is empty, which is exactly
	// the invariant for a stale mountpoint. A non-empty directory is not ours.
	err = os.Remove(target)
	switch {
	case err == nil:
		return
	case errors.Is(err, os.ErrNotExist):
		return
	default:
		fmt.Fprintf(os.Stderr, "hailo-cdi-hook: remove %s: %v\n", target, err)
		os.Exit(1)
	}
}

// mountedSomewhere reports whether any process on the host has a sysfs mount
// at /sys/class/hailo_chardev/<dev> in its mount namespace. On the host that
// path is a plain directory, never a mount point, so a match can only come
// from a container that received the device via CDI.
func mountedSomewhere(dev string) bool {
	want := "/sys/class/hailo_chardev/" + dev
	procs, err := filepath.Glob("/proc/[0-9]*/mountinfo")
	if err != nil {
		return false
	}
	for _, mi := range procs {
		data, err := os.ReadFile(mi)
		if err != nil {
			continue // process exited between glob and read
		}
		for _, line := range strings.Split(string(data), "\n") {
			// mountinfo: id parent major:minor root mountpoint opts [optional...] - fstype source superopts
			pre, post, ok := strings.Cut(line, " - ")
			if !ok {
				continue
			}
			f := strings.Fields(pre)
			if len(f) < 5 || f[4] != want {
				continue
			}
			if strings.HasPrefix(post, "sysfs ") {
				return true
			}
		}
	}
	return false
}

// safeTarget cleans p and verifies it is a direct child of baseDir.
func safeTarget(p string) (string, error) {
	clean := filepath.Clean(p)
	if !filepath.IsAbs(clean) {
		return "", errors.New("path must be absolute")
	}
	if filepath.Dir(clean) != baseDir {
		return "", fmt.Errorf("path must be a direct child of %s", baseDir)
	}
	name := filepath.Base(clean)
	if name == "." || name == ".." || strings.ContainsAny(name, "/") {
		return "", errors.New("invalid directory name")
	}
	return clean, nil
}
