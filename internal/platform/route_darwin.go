//go:build darwin

package platform

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// ReadDefaultRoute reads the default route for one address family with
// `route -n get`. It never modifies the routing table.
func ReadDefaultRoute(family Family) (DefaultRoute, bool, error) {
	flag := "-inet"
	if family == FamilyIPv6 {
		flag = "-inet6"
	}
	output, err := exec.Command("route", "-n", "get", flag, "default").Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			stderr := strings.ToLower(strings.TrimSpace(string(exitErr.Stderr)))
			if strings.Contains(stderr, "not in table") || strings.Contains(stderr, "no such process") {
				return DefaultRoute{}, false, nil
			}
			return DefaultRoute{}, false, fmt.Errorf("route get %s default: %s", family, stderr)
		}
		return DefaultRoute{}, false, fmt.Errorf("route get %s default: %w", family, err)
	}
	return parseDarwinRoute(string(output), family)
}

// parseDarwinRoute extracts the gateway and interface from `route -n get`
// output.
func parseDarwinRoute(output string, family Family) (DefaultRoute, bool, error) {
	route := DefaultRoute{Family: family}
	found := false
	for _, line := range strings.Split(output, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		switch strings.TrimSuffix(fields[0], ":") {
		case "gateway":
			route.Gateway = fields[1]
			found = true
		case "interface":
			route.Interface = fields[1]
		}
	}
	if !found {
		return DefaultRoute{}, false, nil
	}
	if route.Gateway == "" || route.Interface == "" {
		return DefaultRoute{}, false, fmt.Errorf("incomplete default route: gateway %q interface %q", route.Gateway, route.Interface)
	}
	return route, true, nil
}
