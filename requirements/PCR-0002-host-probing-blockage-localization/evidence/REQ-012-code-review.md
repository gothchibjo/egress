# REQ-012 code review sign-off

Date: 2026-11-19 Reviewer sign-off: pending

## Outbound call sites

```text
internal/probe/dns.go:322:	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
internal/probe/icmp.go:267:	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
internal/probe/icmp_client.go:32:	conn, err := icmp.ListenPacket(network, "")
internal/probe/icmp_client.go:86:	if _, err := c.conn.WriteTo(payload, destination); err != nil {
internal/probe/ladder.go:410:		request, err := http.NewRequestWithContext(stepCtx, http.MethodGet, url, nil)
internal/probe/ladder.go:488:	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
internal/probe/mtu_prober.go:24:	packetConn, err := net.ListenPacket("ip4:icmp", "0.0.0.0")
internal/probe/mtu_prober.go:84:	if _, err := p.conn.WriteTo(payload, &net.IPAddr{IP: target}); err != nil {
internal/probe/portal.go:96:	request, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
internal/probe/tcp.go:72:		dialer := net.Dialer{Timeout: timeout}
internal/probe/tcp.go:73:		return dialer.DialContext(ctx, network, address)
internal/probe/trace.go:112:	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
internal/probe/trace.go:135:	ping, err := icmp.ListenPacket(network, "")
internal/probe/trace.go:155:			udp, dialErr := net.DialUDP(network, nil, &net.UDPAddr{IP: target, Port: port})
internal/probe/trace.go:377:	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
internal/probe/trace.go:388:	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
internal/probe/trace.go:406:	conn, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0")
internal/probe/trace.go:455:			if _, writeErr := packetConn.WriteTo(payload, nil, &net.IPAddr{IP: target}); writeErr != nil {
internal/tui/picker.go:214:	addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
```

Every call site belongs to a documented probe: baseline DNS/DoH, ICMP, TCP, path
MTU, captive portal, or the session ladder and traceroute. Session traffic is
limited to the selected targets, configured resolvers and the DoH endpoint.

## Telemetry and retention scan

```text
$ grep -rniE "telemetry|analytics|sentry|posthog|upload|crash report|enrichment" --include=*.go internal cmd
(no matches outside user-facing copy and tests)

$ grep -rn "os.Getenv" --include=*.go internal | grep -v _test.go
internal/tui/dashboard.go:259:	if value := os.Getenv("NO_COLOR"); value != "" {
internal/tui/dashboard.go:262:	return !strings.EqualFold(os.Getenv("TERM"), "dumb")
```

The only environment reads are NO_COLOR and TERM for rendering. No module,
client or goroutine performs telemetry, uploads or background enrichment.

## Persistent identifiers

No identifier is generated or stored. The trace sequence counter and ICMP echo
id separate concurrent probes in memory only; they never enter the export.
Target ids come from the static catalog or the typed target.

## Guard tests

- internal/domain/session_destination_test.go: targets, resolvers and DoH only;
  duplicate rows removed; invalid endpoints rejected.
- internal/probe/destinations_test.go: baseline rows stay inside the documented
  anchor and service set.
- internal/report/session_test.go: no environment data in the export.
