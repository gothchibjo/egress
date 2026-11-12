//go:build darwin || linux

package platform

import (
	"runtime"
	"testing"
)

func TestSupportedPlatformCapabilities(t *testing.T) {
	caps := System()
	if caps.Platform() != runtime.GOOS {
		t.Fatalf("platform = %q, want %q", caps.Platform(), runtime.GOOS)
	}
	icmp := caps.ICMP()
	if !icmp.Datagram || !icmp.Raw {
		t.Fatalf("ICMP capability = %+v", icmp)
	}
}
