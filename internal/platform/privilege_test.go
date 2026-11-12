//go:build darwin || linux

package platform

import (
	"os"
	"testing"
)

func TestDetectPrivilege(t *testing.T) {
	privilege := DetectPrivilege()
	if privilege.Elevated != (os.Geteuid() == 0) {
		t.Fatalf("Elevated = %v, want %v", privilege.Elevated, os.Geteuid() == 0)
	}
	if privilege.RawICMP && !privilege.Elevated {
		t.Fatal("raw ICMP reported without elevation")
	}
	if !privilege.RawICMP && privilege.Reason == "" {
		t.Fatal("unavailable raw ICMP has no reason")
	}
}
