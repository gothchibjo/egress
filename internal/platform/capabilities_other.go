//go:build !darwin && !linux

package platform

import "runtime"

// System reports that this platform is outside the v1 support matrix.
func System() Capabilities {
	return systemCapabilities{platform: runtime.GOOS}
}

func icmpCapability() ICMPCapability {
	return ICMPCapability{Reason: "egress v1 supports macOS and Linux only"}
}

func tracingCapability() TracingCapability {
	return TracingCapability{Reason: "egress v1 supports macOS and Linux only"}
}
