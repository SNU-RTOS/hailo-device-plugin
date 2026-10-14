package main

import "testing"

func TestSafeTarget(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"/var/lib/hailo-cdi/empty-chardev/hailo1", "/var/lib/hailo-cdi/empty-chardev/hailo1", true},
		{"/var/lib/hailo-cdi/empty-chardev/hailo1/", "/var/lib/hailo-cdi/empty-chardev/hailo1", true},
		{"/var/lib/hailo-cdi/empty-chardev", "", false},
		{"/var/lib/hailo-cdi/empty-chardev/../hailo-cdi-hook", "", false},
		{"/var/lib/hailo-cdi/empty-chardev/a/b", "", false},
		{"/etc/passwd", "", false},
		{"hailo1", "", false},
		{"/", "", false},
	}
	for _, c := range cases {
		got, err := safeTarget(c.in)
		if (err == nil) != c.ok || got != c.want {
			t.Errorf("safeTarget(%q) = %q, %v; want %q, ok=%v", c.in, got, err, c.want, c.ok)
		}
	}
}
