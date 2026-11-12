//go:build !darwin && !linux

package platform

// DetectPrivilege reports that this platform cannot probe the host in v1.
func DetectPrivilege() Privilege {
	return Privilege{Reason: "egress v1 supports macOS and Linux only"}
}
