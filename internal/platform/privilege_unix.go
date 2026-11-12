//go:build darwin || linux

package platform

import "os"

// DetectPrivilege reports the effective mode without prompting or escalating.
func DetectPrivilege() Privilege {
	elevated := os.Geteuid() == 0
	icmp := System().ICMP()
	privilege := Privilege{Elevated: elevated, RawICMP: elevated && icmp.Raw}
	if !elevated {
		privilege.Reason = "raw ICMP requires elevated mode; run with elevation (sudo)"
	} else if !icmp.Raw {
		privilege.Reason = icmp.Reason
	}
	return privilege
}
