package domain

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

// DestinationKind groups documented destinations by the probe that contacts
// them.
type DestinationKind string

const (
	// DestinationResolver is a nameserver configured on this host.
	DestinationResolver DestinationKind = "resolver"
	// DestinationDoH is the public DNS-over-HTTPS comparison endpoint.
	DestinationDoH DestinationKind = "doh"
	// DestinationAnchor is a curated reachability anchor.
	DestinationAnchor DestinationKind = "anchor"
	// DestinationCaptive is the captive-portal HTTP canary.
	DestinationCaptive DestinationKind = "captive-portal"
	// DestinationPathMTU is the destination of the don't-fragment probe.
	DestinationPathMTU DestinationKind = "path-mtu"
)

// Transport names the protocol that leaves the machine.
type Transport string

const (
	TransportUDPAndTCP Transport = "udp+tcp"
	TransportTCP       Transport = "tcp"
	TransportHTTPS     Transport = "https"
	TransportHTTP      Transport = "http"
	TransportICMP      Transport = "icmp"
	TransportICMPDF    Transport = "icmp-df"
)

// Destination is one documented outbound endpoint. The baseline may contact
// these endpoints and no others; the list is rendered by the TUI and included
// in the JSON envelope.
type Destination struct {
	Kind      DestinationKind `json:"kind"`
	Host      string          `json:"host"`
	Port      int             `json:"port,omitempty"`
	Transport Transport       `json:"transport"`
	Purpose   string          `json:"purpose"`
}

// ErrInvalidDestination marks a destination that cannot be documented.
var ErrInvalidDestination = errors.New("invalid destination")

// Validate reports whether the destination can be documented and reviewed.
func (d Destination) Validate() error {
	if !d.Kind.valid() {
		return fmt.Errorf("%w: unknown kind %q", ErrInvalidDestination, d.Kind)
	}
	if strings.TrimSpace(d.Host) == "" {
		return fmt.Errorf("%w: missing host", ErrInvalidDestination)
	}
	if strings.ContainsAny(d.Host, " \t/") {
		return fmt.Errorf("%w: host %q contains a separator", ErrInvalidDestination, d.Host)
	}
	if d.Port < 0 || d.Port > 65535 {
		return fmt.Errorf("%w: port %d out of range", ErrInvalidDestination, d.Port)
	}
	if !d.Transport.valid() {
		return fmt.Errorf("%w: unknown transport %q", ErrInvalidDestination, d.Transport)
	}
	if d.Transport.needsPort() && d.Port == 0 {
		return fmt.Errorf("%w: transport %q requires a port", ErrInvalidDestination, d.Transport)
	}
	if strings.TrimSpace(d.Purpose) == "" {
		return fmt.Errorf("%w: missing purpose", ErrInvalidDestination)
	}
	return nil
}

// Endpoint renders host:port for port-based destinations.
func (d Destination) Endpoint() string {
	if d.Port == 0 {
		return d.Host
	}
	return net.JoinHostPort(d.Host, fmt.Sprint(d.Port))
}

func (k DestinationKind) valid() bool {
	switch k {
	case DestinationResolver, DestinationDoH, DestinationAnchor, DestinationCaptive, DestinationPathMTU:
		return true
	default:
		return false
	}
}

func (t Transport) valid() bool {
	switch t {
	case TransportUDPAndTCP, TransportTCP, TransportHTTPS, TransportHTTP, TransportICMP, TransportICMPDF:
		return true
	default:
		return false
	}
}

func (t Transport) needsPort() bool {
	switch t {
	case TransportTCP, TransportHTTPS, TransportHTTP:
		return true
	default:
		return false
	}
}
