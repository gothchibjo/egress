package domain

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

// ErrInvalidTarget marks a custom target that cannot be parsed.
var ErrInvalidTarget = errors.New("invalid target")

// Target is a custom probe destination: a hostname or IP literal with an
// optional port and protocol override.
type Target struct {
	Host     string
	Port     int            // 0 means the default ports of the preset protocols
	Protocol PresetProtocol // "" means no override
}

// Key is the de-duplication key: case-insensitive host, port and protocol.
func (t Target) Key() string {
	protocol := string(t.Protocol)
	if protocol == "" {
		protocol = "auto"
	}
	return strings.ToLower(t.Host) + ":" + strconv.Itoa(t.Port) + "@" + protocol
}

// String renders the canonical input form.
func (t Target) String() string {
	host := t.Host
	if ip := net.ParseIP(host); ip != nil && strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	out := host
	if t.Port > 0 {
		out += ":" + strconv.Itoa(t.Port)
	}
	if t.Protocol != "" {
		out += "@" + string(t.Protocol)
	}
	return out
}

// Endpoint renders host:port with the transport separator.
func (t Target) Endpoint() string {
	if t.Port > 0 {
		return net.JoinHostPort(t.Host, strconv.Itoa(t.Port))
	}
	return net.JoinHostPort(t.Host, "443")
}

// ParseTarget parses a custom target:
// `host[:port][@protocol]`, where host is a hostname, IPv4 literal, IPv6
// literal, or a bracketed IPv6 literal. Full URLs are out of scope.
func ParseTarget(input string) (Target, error) {
	raw := strings.TrimSpace(input)
	if raw == "" {
		return Target{}, fmt.Errorf("%w: target is empty", ErrInvalidTarget)
	}
	if strings.Contains(raw, "://") {
		return Target{}, fmt.Errorf("%w: full URLs are not supported, use host[:port][@protocol]", ErrInvalidTarget)
	}
	if strings.ContainsAny(raw, "/?# \t") {
		return Target{}, fmt.Errorf("%w: %q contains a path, query or space", ErrInvalidTarget, raw)
	}

	target := Target{}
	hostPart := raw
	if at := strings.LastIndex(raw, "@"); at >= 0 {
		protocol := strings.ToLower(raw[at+1:])
		switch PresetProtocol(protocol) {
		case PresetTCP, PresetTLS, PresetHTTP, PresetICMP:
			target.Protocol = PresetProtocol(protocol)
		default:
			return Target{}, fmt.Errorf("%w: unknown protocol %q", ErrInvalidTarget, raw[at+1:])
		}
		hostPart = raw[:at]
	}

	host, portPart, err := splitHostPort(hostPart)
	if err != nil {
		return Target{}, err
	}
	if portPart != "" {
		port, convErr := strconv.Atoi(portPart)
		if convErr != nil || port < 1 || port > 65535 {
			return Target{}, fmt.Errorf("%w: port %q must be between 1 and 65535", ErrInvalidTarget, portPart)
		}
		target.Port = port
	}

	if ip := net.ParseIP(host); ip != nil {
		target.Host = host
	} else if validHostname(host) {
		target.Host = strings.ToLower(host)
	} else {
		return Target{}, fmt.Errorf("%w: %q is not a hostname or IP literal", ErrInvalidTarget, host)
	}
	return target, nil
}

// splitHostPort splits a bracketed IPv6 literal, a bare IPv6 literal, or a
// host with an optional port.
func splitHostPort(raw string) (host, port string, err error) {
	if strings.HasPrefix(raw, "[") {
		end := strings.Index(raw, "]")
		if end < 0 {
			return "", "", fmt.Errorf("%w: IPv6 literal is missing ]", ErrInvalidTarget)
		}
		host = raw[1:end]
		if ip := net.ParseIP(host); ip == nil || !strings.Contains(host, ":") {
			return "", "", fmt.Errorf("%w: %q is not an IPv6 literal", ErrInvalidTarget, host)
		}
		rest := raw[end+1:]
		if rest == "" {
			return host, "", nil
		}
		if !strings.HasPrefix(rest, ":") {
			return "", "", fmt.Errorf("%w: unexpected %q after the IPv6 literal", ErrInvalidTarget, rest)
		}
		return host, rest[1:], nil
	}
	if strings.Count(raw, ":") > 1 {
		// A bare IPv6 literal. Ports need brackets.
		return raw, "", nil
	}
	if colon := strings.LastIndex(raw, ":"); colon >= 0 {
		if colon == len(raw)-1 {
			return "", "", fmt.Errorf("%w: missing port after ':'", ErrInvalidTarget)
		}
		return raw[:colon], raw[colon+1:], nil
	}
	return raw, "", nil
}

// validHostname checks the hostname form used for probe targets.
func validHostname(host string) bool {
	if host == "" || len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(host, ".") {
		if label == "" || len(label) > 63 {
			return false
		}
		if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
		for _, r := range label {
			switch {
			case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-':
			default:
				return false
			}
		}
	}
	return true
}
