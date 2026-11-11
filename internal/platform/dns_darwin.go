//go:build darwin

package platform

import (
	"fmt"
	"os/exec"
)

// ReadResolvers lists the configured nameservers with `scutil --dns`. It never
// changes DNS configuration.
func ReadResolvers() ([]Resolver, error) {
	output, err := exec.Command("scutil", "--dns").Output()
	if err != nil {
		return nil, fmt.Errorf("scutil --dns: %w", err)
	}
	return parseScutilDNS(string(output)), nil
}
