package probe

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"sort"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/gothchibjo/egress/internal/domain"
)

const (
	defaultMaxHops     = 30
	defaultHopProbes   = 3
	defaultProbeWindow = time.Second
	tracePortBase      = 33434
)

// Tracer discovers the path to a host.
type Tracer interface {
	Method() string
	Trace(ctx context.Context, host string) ([]domain.Hop, error)
}

// TracerInfo describes the selected tracing method for the session.
type TracerInfo struct {
	Method string
	Mode   domain.SessionMode
	Reason string
}

// NewTracer selects ICMP tracing for an elevated session and UDP tracing
// otherwise. If a raw socket cannot be opened, it falls back to UDP with a
// stated reason instead of failing the run.
func NewTracer(elevated bool) (Tracer, TracerInfo) {
	if !elevated {
		return UDPTracer{}, TracerInfo{
			Method: "udp",
			Mode:   domain.ModeLimited,
			Reason: "unprivileged session uses UDP tracing",
		}
	}
	if tracer, err := newICMPTracer(); err == nil {
		return tracer, TracerInfo{Method: "icmp", Mode: domain.ModeElevated}
	} else {
		return UDPTracer{}, TracerInfo{
			Method: "udp",
			Mode:   domain.ModeLimited,
			Reason: "raw ICMP socket unavailable: " + err.Error(),
		}
	}
}

// UDPTracer sends UDP probes with growing TTL values and reads the ICMP errors
// through an unprivileged ping socket.
type UDPTracer struct {
	MaxHops  int
	Probes   int
	Window   time.Duration
	PortBase int
}

func (t UDPTracer) Method() string { return "udp" }

func (t UDPTracer) normalized() UDPTracer {
	if t.MaxHops <= 0 {
		t.MaxHops = defaultMaxHops
	}
	if t.Probes <= 0 {
		t.Probes = defaultHopProbes
	}
	if t.Window <= 0 {
		t.Window = defaultProbeWindow
	}
	if t.PortBase <= 0 {
		t.PortBase = tracePortBase
	}
	return t
}

func (t UDPTracer) Trace(ctx context.Context, host string) ([]domain.Hop, error) {
	t = t.normalized()
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	target := addresses[0].IP

	network, protocol := "udp4", 1
	if target.To4() == nil {
		network, protocol = "udp6", 58
	}
	ping, err := icmp.ListenPacket(network, "")
	if err != nil {
		return nil, fmt.Errorf("open ICMP socket: %w", err)
	}
	defer ping.Close()

	hops := make([]domain.Hop, 0, t.MaxHops)
	for ttl := 1; ttl <= t.MaxHops; ttl++ {
		hop := domain.Hop{TTL: ttl}
		rtts := make([]int64, 0, t.Probes)
		reached := false

		for probe := 0; probe < t.Probes; probe++ {
			if ctx.Err() != nil {
				return hops, ctx.Err()
			}
			port := t.PortBase + ttl*10 + probe
			probeStart := time.Now()
			udp, dialErr := net.DialUDP(network, nil, &net.UDPAddr{IP: target, Port: port})
			if dialErr != nil {
				hop.Sent++
				continue
			}
			if target.To4() == nil {
				_ = ipv6.NewConn(udp).SetHopLimit(ttl)
			} else {
				_ = ipv4.NewConn(udp).SetTTL(ttl)
			}
			hop.Sent++
			_, _ = udp.Write([]byte("egress-trace"))

			peer, targetReached, readErr := awaitICMP(ctx, ping, protocol, port, time.Now().Add(t.Window), target)
			_ = udp.Close()
			if readErr != nil {
				continue
			}
			rtts = append(rtts, time.Since(probeStart).Milliseconds())
			hop.Received++
			if hop.Address == "" {
				hop.Address = peer
			}
			if targetReached {
				reached = true
			}
		}

		hop.RTTMinMS, hop.RTTMedianMS, hop.RTTMaxMS = rttStats(rtts)
		hops = append(hops, hop)
		if reached {
			break
		}
	}
	return hops, nil
}

// awaitICMP waits for the ICMP error that quotes the probe port.
func awaitICMP(ctx context.Context, conn *icmp.PacketConn, protocol, port int, deadline time.Time, target net.IP) (string, bool, error) {
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return "", false, err
	}

	buffer := make([]byte, 1500)
	for {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		n, peer, err := conn.ReadFrom(buffer)
		if err != nil {
			return "", false, err
		}
		message, err := icmp.ParseMessage(protocol, buffer[:n])
		if err != nil {
			continue
		}
		data, ok := icmpErrorData(message)
		if !ok {
			continue
		}
		quoted, ok := quotedUDPPort(data, protocol)
		if !ok || quoted != port {
			continue
		}
		address := addrString(peer)
		reached := message.Code == portUnreachableCode(protocol)
		if peerIP := net.ParseIP(address); peerIP != nil && peerIP.Equal(target) {
			// Any ICMP message from the target means the path reached it; the
			// port-unreachable reply is often rate-limited.
			reached = true
		}
		return address, reached, nil
	}
}

// icmpErrorData returns the quoted packet of a time-exceeded or
// destination-unreachable message.
func icmpErrorData(message *icmp.Message) ([]byte, bool) {
	switch body := message.Body.(type) {
	case *icmp.TimeExceeded:
		return body.Data, true
	case *icmp.DstUnreach:
		return body.Data, true
	default:
		return nil, false
	}
}

// quotedUDPPort extracts the destination UDP port from the quoted packet.
func quotedUDPPort(data []byte, protocol int) (int, bool) {
	if len(data) < 4 {
		return 0, false
	}
	if protocol == 1 {
		if data[0]>>4 != 4 {
			return 0, false
		}
		headerLength := int(data[0]&0x0f) * 4
		if headerLength < 20 || len(data) < headerLength+4 {
			return 0, false
		}
		if data[9] != 17 { // UDP
			return 0, false
		}
		return int(binary.BigEndian.Uint16(data[headerLength+2 : headerLength+4])), true
	}
	if len(data) < 44 {
		return 0, false
	}
	if data[0]>>4 != 6 {
		return 0, false
	}
	if data[6] != 17 { // UDP
		return 0, false
	}
	return int(binary.BigEndian.Uint16(data[42:44])), true
}

func portUnreachableCode(protocol int) int {
	if protocol == 58 {
		return 4 // ICMPv6 destination unreachable, port unreachable
	}
	return 3 // ICMP destination unreachable, port unreachable
}

func addrString(addr net.Addr) string {
	switch value := addr.(type) {
	case *net.IPAddr:
		return value.IP.String()
	case *net.UDPAddr:
		return value.IP.String()
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}

func rttStats(values []int64) (min, median, max int64) {
	if len(values) == 0 {
		return 0, 0, 0
	}
	sorted := append([]int64(nil), values...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	return sorted[0], sorted[len(sorted)/2], sorted[len(sorted)-1]
}

// ICMPTracer sends ICMP echo probes with growing TTL values. It needs a raw
// socket, so it is only available to an elevated session.
type ICMPTracer struct {
	conn    *icmp.PacketConn
	MaxHops int
	Probes  int
	Window  time.Duration
}

func newICMPTracer() (ICMPTracer, error) {
	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return ICMPTracer{}, err
	}
	_ = conn.Close()
	return ICMPTracer{MaxHops: defaultMaxHops, Probes: defaultHopProbes, Window: defaultProbeWindow}, nil
}

func (t ICMPTracer) Method() string { return "icmp" }

func (t ICMPTracer) Trace(ctx context.Context, host string) ([]domain.Hop, error) {
	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("resolve %q: %w", host, err)
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("resolve %q: no addresses", host)
	}
	target := addresses[0].IP
	if target.To4() == nil {
		return nil, errors.New("ICMP tracing supports IPv4 targets in v1")
	}

	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	packetConn := conn.IPv4PacketConn()

	maxHops, probes, window := t.MaxHops, t.Probes, t.Window
	if maxHops <= 0 {
		maxHops = defaultMaxHops
	}
	if probes <= 0 {
		probes = defaultHopProbes
	}
	if window <= 0 {
		window = defaultProbeWindow
	}
	id := uint16(os.Getpid() & 0xffff)

	hops := make([]domain.Hop, 0, maxHops)
	for ttl := 1; ttl <= maxHops; ttl++ {
		hop := domain.Hop{TTL: ttl}
		rtts := make([]int64, 0, probes)
		reached := false

		for probe := 0; probe < probes; probe++ {
			if ctx.Err() != nil {
				return hops, ctx.Err()
			}
			seq := ttl*10 + probe
			message := icmp.Message{
				Type: ipv4.ICMPTypeEcho,
				Code: 0,
				Body: &icmp.Echo{ID: int(id), Seq: seq, Data: []byte("egress")},
			}
			payload, marshalErr := message.Marshal(nil)
			if marshalErr != nil {
				hop.Sent++
				continue
			}
			start := time.Now()
			hop.Sent++
			if _, writeErr := packetConn.WriteTo(payload, &ipv4.ControlMessage{TTL: ttl}, &net.IPAddr{IP: target}); writeErr != nil {
				continue
			}

			peer, targetReached, readErr := awaitEcho(ctx, conn, id, seq, time.Now().Add(window))
			if readErr != nil {
				continue
			}
			rtts = append(rtts, time.Since(start).Milliseconds())
			hop.Received++
			if hop.Address == "" {
				hop.Address = peer
			}
			if targetReached {
				reached = true
			}
		}

		hop.RTTMinMS, hop.RTTMedianMS, hop.RTTMaxMS = rttStats(rtts)
		hops = append(hops, hop)
		if reached {
			break
		}
	}
	return hops, nil
}

// awaitEcho waits for the echo reply or the ICMP error that quotes our probe.
func awaitEcho(ctx context.Context, conn *icmp.PacketConn, id uint16, seq int, deadline time.Time) (string, bool, error) {
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	if err := conn.SetReadDeadline(deadline); err != nil {
		return "", false, err
	}

	buffer := make([]byte, 1500)
	for {
		if err := ctx.Err(); err != nil {
			return "", false, err
		}
		n, peer, err := conn.ReadFrom(buffer)
		if err != nil {
			return "", false, err
		}
		message, err := icmp.ParseMessage(1, buffer[:n])
		if err != nil {
			continue
		}
		if reply, ok := message.Body.(*icmp.Echo); ok && message.Type == ipv4.ICMPTypeEchoReply {
			if uint16(reply.ID) == id && reply.Seq == seq {
				return addrString(peer), true, nil
			}
			continue
		}
		data, ok := icmpErrorData(message)
		if !ok {
			continue
		}
		quotedID, quotedSeq, ok := quotedICMPIdentity(data)
		if !ok || quotedID != id || quotedSeq != seq {
			continue
		}
		return addrString(peer), false, nil
	}
}

// quotedICMPIdentity extracts the echo id and sequence from a quoted packet.
func quotedICMPIdentity(data []byte) (id uint16, seq int, ok bool) {
	if len(data) < 28 || data[0]>>4 != 4 {
		return 0, 0, false
	}
	headerLength := int(data[0]&0x0f) * 4
	if headerLength < 20 || len(data) < headerLength+8 {
		return 0, 0, false
	}
	if data[9] != 1 { // ICMP
		return 0, 0, false
	}
	return binary.BigEndian.Uint16(data[headerLength+4 : headerLength+6]),
		int(binary.BigEndian.Uint16(data[headerLength+6 : headerLength+8])), true
}
