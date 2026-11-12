---
type: reference
author: gothchibjo
date: 2026-11-12
---

# Proxy and tunnel context <!-- omit in toc -->

- [Purpose](#purpose)
- [Sources](#sources)
- [Detection](#detection)
- [Privacy](#privacy)

## Purpose

`egress` reports proxy environment variables and tunnel-like interfaces as
read-only context, required by [REQ-023]. The context never changes which probes
run, which destinations are contacted or how a result is classified. There is no
proxy bypass and no direct-versus-proxied comparison in v1.

The TUI shows it on the `c` page; the JSON envelope carries it in the `context`
field. When nothing is detected, both state the absence explicitly instead of
silently omitting the section.

The `c` page states the source rule once at the top: proxy values come from the
environment, tunnel names from the interface list, and a tunnel that carries the
default route is marked `[default route]`. The JSON keeps the full `source` on
every entry.

## Sources

| Context | Source                                            | Reported as                     |
| :------ | :------------------------------------------------ | :------------------------------ |
| proxy   | `HTTP_PROXY`, `http_proxy`                        | name, variable, redacted value  |
| proxy   | `HTTPS_PROXY`, `https_proxy`                      | name, variable, redacted value  |
| proxy   | `ALL_PROXY`, `all_proxy`                          | name, variable, redacted value  |
| proxy   | `NO_PROXY`, `no_proxy`                            | name, variable, value           |
| tunnel  | interface name starting with `utun`, `tun`        | kind `tun`                      |
| tunnel  | interface name starting with `tap`                | kind `tap`                      |
| tunnel  | interface name starting with `wg`                 | kind `wireguard`                |
| tunnel  | interface name starting with `ppp`                | kind `ppp`                      |
| tunnel  | interface name starting with `ipsec`              | kind `ipsec`                    |
| tunnel  | IPv4 or IPv6 default route through that interface | `default_route` flag and source |

Each tunnel entry also reports the link state (`up` or `down`) and its
addresses. The interface name is the first-name match; the default route is read
for both address families and only marks an interface, it never changes a probe.

## Detection

`platform.DetectNetworkContext` reads the interface list and the default routes
through the same capability abstraction the probes use. It sends no packets and
runs no external command beyond the platform route lookup. The JSON form is:

```json
{
  "context": {
    "proxies": [
      {
        "name": "HTTPS_PROXY",
        "source": "https_proxy",
        "value": "http://***@proxy.example:8080"
      }
    ],
    "vpn": [
      {
        "name": "utun3",
        "kind": "tun",
        "state": "up",
        "addresses": ["10.8.0.2/32"],
        "default_route": true,
        "source": "interface name and default route"
      }
    ]
  }
}
```

Absence is encoded as empty arrays:

```json
{ "context": { "proxies": [], "vpn": [] } }
```

## Privacy

Proxy values are reported with credentials redacted: the scheme, host and port
stay visible, the user-info part becomes `***`. No environment dump, no
credential store and no unrelated host state is read or exported.

<!-- refs -->

[REQ-023]:
  ../requirements/PCR-0001-baseline-network-diagnostics-tui/03-technical.yaml
