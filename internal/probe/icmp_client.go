package probe

import (
	"context"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"

	"github.com/gothchibjo/egress/internal/platform"
)

// dialICMP opens an unprivileged datagram socket by default and a raw socket
// only when the session is explicitly elevated.
func dialICMP(family platform.Family, elevated bool, timeout time.Duration) (echoConn, error) {
	network := "udp4"
	if family == platform.FamilyIPv6 {
		network = "udp6"
	}
	datagram := true
	if elevated {
		datagram = false
		network = "ip4:icmp"
		if family == platform.FamilyIPv6 {
			network = "ip6:ipv6-icmp"
		}
	}

	conn, err := icmp.ListenPacket(network, "")
	if err != nil {
		return nil, err
	}
	return &icmpEchoConn{
		conn:     conn,
		family:   family,
		datagram: datagram,
		id:       uint16(os.Getpid()),
		timeout:  timeout,
	}, nil
}

type icmpEchoConn struct {
	conn     *icmp.PacketConn
	family   platform.Family
	datagram bool
	id       uint16
	timeout  time.Duration
}

func (c *icmpEchoConn) Close() error {
	return c.conn.Close()
}

// Exchange sends one echo request and waits for the matching reply.
func (c *icmpEchoConn) Exchange(ctx context.Context, target net.IP, seq int) (time.Duration, error) {
	protocol := 1
	requestType := icmp.Type(ipv4.ICMPTypeEcho)
	replyType := icmp.Type(ipv4.ICMPTypeEchoReply)
	if c.family == platform.FamilyIPv6 {
		protocol = 58
		requestType = ipv6.ICMPTypeEchoRequest
		replyType = ipv6.ICMPTypeEchoReply
	}

	message := &icmp.Message{
		Type: requestType,
		Code: 0,
		Body: &icmp.Echo{ID: int(c.id), Seq: seq, Data: []byte("egress")},
	}
	payload, err := message.Marshal(nil)
	if err != nil {
		return 0, err
	}

	var destination net.Addr
	if c.datagram {
		destination = &net.UDPAddr{IP: target}
	} else {
		destination = &net.IPAddr{IP: target}
	}

	start := time.Now()
	if _, err := c.conn.WriteTo(payload, destination); err != nil {
		return 0, err
	}

	deadline := start.Add(c.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := c.conn.SetReadDeadline(deadline); err != nil {
		return 0, err
	}

	buffer := make([]byte, 1500)
	for {
		n, _, err := c.conn.ReadFrom(buffer)
		if err != nil {
			return 0, err
		}
		parsed, err := icmp.ParseMessage(protocol, buffer[:n])
		if err != nil {
			continue
		}
		if parsed.Type != replyType {
			continue
		}
		echo, ok := parsed.Body.(*icmp.Echo)
		if !ok || echo.ID != int(c.id) || echo.Seq != seq {
			continue
		}
		return time.Since(start), nil
	}
}
