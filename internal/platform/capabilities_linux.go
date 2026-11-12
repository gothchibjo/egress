//go:build linux

package platform

// System returns the Linux capability set.
func System() Capabilities {
	return systemCapabilities{platform: "linux"}
}

func icmpCapability() ICMPCapability {
	return ICMPCapability{Datagram: true, Raw: true}
}

func tracingCapability() TracingCapability {
	return TracingCapability{Reason: "traceroute is not part of the v1 baseline"}
}
