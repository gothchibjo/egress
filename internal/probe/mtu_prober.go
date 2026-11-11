package probe

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

var errPacketTooBig = errors.New("packet too big")

// defaultMTUProberFactory opens a raw IPv4 ICMP socket with the don't-fragment
// flag. It refuses to open one unless the session is explicitly elevated.
func defaultMTUProberFactory(elevated bool, timeout time.Duration) (MTUProber, error) {
	if !elevated {
		return nil, errRawSocketsRequired
	}
	packetConn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
	if err != nil {
		return nil, err
	}
	ipConn, ok := packetConn.(*net.IPConn)
	if !ok {
		packetConn.Close()
		return nil, errors.New("unexpected raw socket type")
	}
	raw, err := ipConn.SyscallConn()
	if err != nil {
		ipConn.Close()
		return nil, err
	}
	var socketErr error
	if err := raw.Control(func(fd uintptr) {
		socketErr = setDontFragment(int(fd))
	}); err != nil {
		ipConn.Close()
		return nil, err
	}
	if socketErr != nil {
		ipConn.Close()
		return nil, socketErr
	}
	return &rawMTUProber{conn: ipConn, timeout: timeout, id: uint16(os.Getpid())}, nil
}

type rawMTUProber struct {
	conn    *net.IPConn
	timeout time.Duration
	id      uint16
}

func (p *rawMTUProber) Close() error {
	return p.conn.Close()
}

// Probe sends one echo request with the requested packet size and waits for
// the reply. A fragmentation-needed error is reported as errPacketTooBig.
func (p *rawMTUProber) Probe(ctx context.Context, target net.IP, mtu int) error {
	dataLength := mtu - 28
	if dataLength < 8 {
		dataLength = 8
	}
	message := &icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   int(p.id),
			Seq:  mtu,
			Data: bytes.Repeat([]byte("x"), dataLength),
		},
	}
	payload, err := message.Marshal(nil)
	if err != nil {
		return err
	}

	start := time.Now()
	if _, err := p.conn.WriteTo(payload, &net.IPAddr{IP: target}); err != nil {
		return err
	}
	deadline := start.Add(p.timeout)
	if contextDeadline, ok := ctx.Deadline(); ok && contextDeadline.Before(deadline) {
		deadline = contextDeadline
	}
	if err := p.conn.SetReadDeadline(deadline); err != nil {
		return err
	}

	buffer := make([]byte, 65535)
	for {
		n, _, err := p.conn.ReadFrom(buffer)
		if err != nil {
			return err
		}
		parsed, err := icmp.ParseMessage(1, buffer[:n])
		if err != nil {
			continue
		}
		switch parsed.Type {
		case ipv4.ICMPTypeDestinationUnreachable:
			if parsed.Code == 4 {
				return errPacketTooBig
			}
			return fmt.Errorf("destination unreachable (code %d)", parsed.Code)
		case ipv4.ICMPTypeEchoReply:
			echo, ok := parsed.Body.(*icmp.Echo)
			if ok && echo.ID == int(p.id) && echo.Seq == mtu {
				return nil
			}
		}
	}
}
