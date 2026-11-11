package platform

import (
	"strings"
)

// Resolver is one configured nameserver address.
type Resolver struct {
	Address string `json:"address"`
}

// parseResolvConf extracts nameserver entries from resolv.conf content.
func parseResolvConf(data []byte) []Resolver {
	resolvers := make([]Resolver, 0)
	seen := make(map[string]bool)
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 || !strings.EqualFold(fields[0], "nameserver") {
			continue
		}
		resolvers = appendResolver(resolvers, seen, fields[1])
	}
	return resolvers
}

// parseScutilDNS extracts nameserver entries from `scutil --dns` output.
func parseScutilDNS(output string) []Resolver {
	resolvers := make([]Resolver, 0)
	seen := make(map[string]bool)
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "nameserver[") {
			continue
		}
		separator := strings.Index(line, ":")
		if separator < 0 {
			continue
		}
		resolvers = appendResolver(resolvers, seen, strings.TrimSpace(line[separator+1:]))
	}
	return resolvers
}

func appendResolver(resolvers []Resolver, seen map[string]bool, address string) []Resolver {
	address = strings.TrimSpace(address)
	if address == "" || seen[address] {
		return resolvers
	}
	seen[address] = true
	return append(resolvers, Resolver{Address: address})
}
