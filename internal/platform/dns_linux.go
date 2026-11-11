//go:build linux

package platform

import (
	"fmt"
	"os"
)

// ReadResolvers lists the configured nameservers from /etc/resolv.conf. It
// never changes DNS configuration.
func ReadResolvers() ([]Resolver, error) {
	data, err := os.ReadFile("/etc/resolv.conf")
	if err != nil {
		return nil, fmt.Errorf("read /etc/resolv.conf: %w", err)
	}
	return parseResolvConf(data), nil
}
