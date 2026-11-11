//go:build linux

package probe

import "golang.org/x/sys/unix"

// setDontFragment enables path-MTU discovery with the don't-fragment policy.
func setDontFragment(fd int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_MTU_DISCOVER, unix.IP_PMTUDISC_DO)
}
