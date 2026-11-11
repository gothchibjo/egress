//go:build linux

package platform

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

const defaultRouteMask = "00000000"

// ReadDefaultRoute reads the kernel routing table for one address family. It
// never modifies the table.
func ReadDefaultRoute(family Family) (DefaultRoute, bool, error) {
	if family == FamilyIPv6 {
		data, err := os.ReadFile("/proc/net/ipv6_route")
		if err != nil {
			return DefaultRoute{}, false, fmt.Errorf("read /proc/net/ipv6_route: %w", err)
		}
		return parseLinuxRoute6(data)
	}
	data, err := os.ReadFile("/proc/net/route")
	if err != nil {
		return DefaultRoute{}, false, fmt.Errorf("read /proc/net/route: %w", err)
	}
	return parseLinuxRoute4(data)
}

// parseLinuxRoute4 extracts the first active IPv4 default route from
// /proc/net/route.
func parseLinuxRoute4(data []byte) (DefaultRoute, bool, error) {
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 8 {
			return DefaultRoute{}, false, fmt.Errorf("malformed /proc/net/route line %d", i+1)
		}
		if fields[1] != defaultRouteMask {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil {
			return DefaultRoute{}, false, fmt.Errorf("parse route flags on line %d: %w", i+1, err)
		}
		if flags&0x1 == 0 {
			continue
		}
		route := DefaultRoute{Family: FamilyIPv4, Interface: fields[0]}
		if fields[2] != defaultRouteMask {
			route.Gateway = littleEndianIPv4(fields[2])
		}
		return route, true, nil
	}
	return DefaultRoute{}, false, nil
}

// parseLinuxRoute6 extracts the first active IPv6 default route from
// /proc/net/ipv6_route.
func parseLinuxRoute6(data []byte) (DefaultRoute, bool, error) {
	for i, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 10 {
			return DefaultRoute{}, false, fmt.Errorf("malformed /proc/net/ipv6_route line %d", i+1)
		}
		if fields[0] != strings.Repeat("0", 32) || fields[1] != "00" {
			continue
		}
		flags, err := strconv.ParseUint(fields[8], 16, 32)
		if err != nil {
			return DefaultRoute{}, false, fmt.Errorf("parse route flags on line %d: %w", i+1, err)
		}
		if flags&0x1 == 0 {
			continue
		}
		route := DefaultRoute{Family: FamilyIPv6, Interface: fields[9]}
		if strings.Trim(fields[4], "0") != "" {
			route.Gateway = formatIPv6Hex(fields[4])
		}
		return route, true, nil
	}
	return DefaultRoute{}, false, nil
}

func littleEndianIPv4(value string) string {
	raw, err := strconv.ParseUint(value, 16, 32)
	if err != nil {
		return ""
	}
	return net.IPv4(byte(raw), byte(raw>>8), byte(raw>>16), byte(raw>>24)).String()
}

func formatIPv6Hex(value string) string {
	if len(value) != 32 {
		return ""
	}
	parts := make([]string, 0, 8)
	for i := 0; i < len(value); i += 4 {
		parts = append(parts, value[i:i+4])
	}
	parsed := net.ParseIP(strings.Join(parts, ":"))
	if parsed == nil {
		return ""
	}
	return parsed.String()
}
