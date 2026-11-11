//go:build darwin

package probe

import "golang.org/x/sys/unix"

// setDontFragment enables the IPv4 don't-fragment flag on a raw socket.
func setDontFragment(fd int) error {
	return unix.SetsockoptInt(fd, unix.IPPROTO_IP, unix.IP_DONTFRAG, 1)
}
