//go:build darwin

package platform

// System returns the macOS capability set.
func System() Capabilities {
	return systemCapabilities{platform: "darwin"}
}

func icmpCapability() ICMPCapability {
	return ICMPCapability{Datagram: true, Raw: true}
}

func tracingCapability() TracingCapability {
	return TracingCapability{Reason: "traceroute is not part of the v1 baseline"}
}
