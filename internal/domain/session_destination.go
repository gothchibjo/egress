package domain

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
)

// SessionDestinationKind groups the endpoints one probing session may contact.
type SessionDestinationKind string

const (
	// SessionTarget is a selected probe target.
	SessionTarget SessionDestinationKind = "target"
	// SessionResolver is a nameserver configured on this host.
	SessionResolver SessionDestinationKind = "resolver"
	// SessionDoH is the public DNS-over-HTTPS comparison endpoint.
	SessionDoH SessionDestinationKind = "doh"
)

// SessionTransport names the protocol a session probe uses.
type SessionTransport string

const (
	SessionUDPAndTCP SessionTransport = "udp+tcp"
	SessionTCP       SessionTransport = "tcp"
	SessionTLS       SessionTransport = "https"
	SessionHTTP      SessionTransport = "http"
	SessionICMP      SessionTransport = "icmp"
	SessionTraceUDP  SessionTransport = "udp"
)

// SessionDestination is one endpoint a probing session may contact. Address is
// the resolved address when the ladder already knows it; Host keeps the
// declared target name.
type SessionDestination struct {
	Kind      SessionDestinationKind `json:"kind"`
	Host      string                 `json:"host"`
	Address   string                 `json:"address,omitempty"`
	Port      int                    `json:"port,omitempty"`
	Transport SessionTransport       `json:"transport"`
	Purpose   string                 `json:"purpose"`
}

// ErrInvalidSessionDestination marks a session endpoint that cannot be
// documented.
var ErrInvalidSessionDestination = errors.New("invalid session destination")

// Validate reports whether the session endpoint can be reviewed.
func (d SessionDestination) Validate() error {
	if !d.Kind.valid() {
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidSessionDestination, d.Kind)
	}
	if strings.TrimSpace(d.Host) == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidSessionDestination)
	}
	if strings.ContainsAny(d.Host, " \t/") {
		return fmt.Errorf("%w: host %q contains a separator", ErrInvalidSessionDestination, d.Host)
	}
	if d.Address != "" && net.ParseIP(d.Address) == nil {
		return fmt.Errorf("%w: address %q is not an IP", ErrInvalidSessionDestination, d.Address)
	}
	if d.Port < 0 || d.Port > 65535 {
		return fmt.Errorf("%w: port %d out of range", ErrInvalidSessionDestination, d.Port)
	}
	if !d.Transport.valid() {
		return fmt.Errorf("%w: unknown transport %q", ErrInvalidSessionDestination, d.Transport)
	}
	if d.Transport.needsPort() && d.Port == 0 {
		return fmt.Errorf("%w: transport %q requires a port", ErrInvalidSessionDestination, d.Transport)
	}
	if strings.TrimSpace(d.Purpose) == "" {
		return fmt.Errorf("%w: missing purpose", ErrInvalidSessionDestination)
	}
	return nil
}

// Endpoint renders the resolved address when known, with its port.
func (d SessionDestination) Endpoint() string {
	host := d.Host
	if d.Address != "" {
		host = d.Address
	}
	if d.Port == 0 {
		return host
	}
	return net.JoinHostPort(host, strconv.Itoa(d.Port))
}

// DisplayHost renders the declared host with the resolved address beside it.
func (d SessionDestination) DisplayHost() string {
	if d.Address != "" && d.Address != d.Host {
		return d.Host + " (" + d.Address + ")"
	}
	return d.Host
}

// BuildSessionDestinations lists every endpoint the selected targets, the
// configured resolvers and the DoH comparison may contact in one session.
func BuildSessionDestinations(targets []ProbeTarget, steps map[string][]StepResult, documented []Destination) []SessionDestination {
	out := make([]SessionDestination, 0, len(targets)*3+len(documented))
	for _, target := range targets {
		address := targetAddress(target, steps[target.ID])
		ports := target.Ports
		if len(ports) == 0 {
			ports = []int{443}
		}
		protocols := target.Protocols
		if len(protocols) == 0 {
			protocols = []PresetProtocol{PresetTCP}
		}
		for _, protocol := range protocols {
			switch protocol {
			case PresetTCP:
				for _, port := range ports {
					out = append(out, targetDestination(target, address, port, SessionTCP, "TCP connect to the selected target"))
				}
			case PresetTLS:
				for _, port := range ports {
					out = append(out, targetDestination(target, address, port, SessionTLS, "TLS handshake to the selected target"))
				}
			case PresetHTTP:
				for _, port := range ports {
					out = append(out, targetDestination(target, address, port, SessionHTTP, "HTTP request to the selected target"))
				}
			case PresetICMP:
				out = append(out, targetDestination(target, address, 0, SessionICMP, "ICMP echo to the selected target"))
			}
		}
		// The ladder always appends a trace step, so its UDP probes belong in
		// the declared list even when the target declares no protocols.
		out = append(out, targetDestination(target, address, 0, SessionTraceUDP, "TTL-limited trace to the selected target"))
	}

	for _, destination := range documented {
		switch destination.Kind {
		case DestinationResolver:
			out = append(out, SessionDestination{
				Kind:      SessionResolver,
				Host:      destination.Host,
				Port:      destination.Port,
				Transport: SessionUDPAndTCP,
				Purpose:   "configured resolver for name lookups",
			})
		case DestinationDoH:
			out = append(out, SessionDestination{
				Kind:      SessionDoH,
				Host:      destination.Host,
				Port:      destination.Port,
				Transport: SessionTLS,
				Purpose:   "public DoH comparison for name lookups",
			})
		}
	}
	return normalizeSessionDestinations(out)
}

func targetDestination(target ProbeTarget, address string, port int, transport SessionTransport, purpose string) SessionDestination {
	return SessionDestination{
		Kind:      SessionTarget,
		Host:      target.Host,
		Address:   address,
		Port:      port,
		Transport: transport,
		Purpose:   purpose,
	}
}

// targetAddress returns the address the ladder actually probes when a step has
// already recorded it.
func targetAddress(target ProbeTarget, steps []StepResult) string {
	// The trace and ICMP steps carry the address the ladder actually probed;
	// the DNS answer is only a fallback when those steps have not run yet.
	for _, step := range steps {
		if step.Kind == StepTrace || step.Kind == StepICMP {
			if address := stepMeasurement(step, "address"); address != "" {
				return address
			}
		}
	}
	for _, step := range steps {
		if step.Kind == StepDNS {
			if answer := firstAnswer(stepMeasurement(step, "system.answers")); answer != "" {
				return answer
			}
		}
	}
	if net.ParseIP(target.Host) != nil {
		return target.Host
	}
	return ""
}

func normalizeSessionDestinations(destinations []SessionDestination) []SessionDestination {
	seen := make(map[string]bool, len(destinations))
	out := make([]SessionDestination, 0, len(destinations))
	for _, destination := range destinations {
		key := string(destination.Kind) + "\x00" + destination.Host + "\x00" +
			destination.Address + "\x00" + strconv.Itoa(destination.Port) + "\x00" + string(destination.Transport)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, destination)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if sessionKindOrder(out[i].Kind) != sessionKindOrder(out[j].Kind) {
			return sessionKindOrder(out[i].Kind) < sessionKindOrder(out[j].Kind)
		}
		if out[i].Host != out[j].Host {
			return out[i].Host < out[j].Host
		}
		if out[i].Port != out[j].Port {
			return out[i].Port < out[j].Port
		}
		return out[i].Transport < out[j].Transport
	})
	return out
}

func sessionKindOrder(kind SessionDestinationKind) int {
	switch kind {
	case SessionTarget:
		return 0
	case SessionResolver:
		return 1
	case SessionDoH:
		return 2
	default:
		return 3
	}
}

func (k SessionDestinationKind) valid() bool {
	switch k {
	case SessionTarget, SessionResolver, SessionDoH:
		return true
	default:
		return false
	}
}

func (t SessionTransport) valid() bool {
	switch t {
	case SessionUDPAndTCP, SessionTCP, SessionTLS, SessionHTTP, SessionICMP, SessionTraceUDP:
		return true
	default:
		return false
	}
}

func (t SessionTransport) needsPort() bool {
	switch t {
	case SessionTCP, SessionTLS, SessionHTTP:
		return true
	default:
		return false
	}
}

// DisplayEndpoint renders the declared host with the resolved address and port.
func (d SessionDestination) DisplayEndpoint() string {
	endpoint := d.DisplayHost()
	if d.Port == 0 {
		return endpoint
	}
	return endpoint + ":" + strconv.Itoa(d.Port)
}
