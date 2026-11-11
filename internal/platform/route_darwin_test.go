//go:build darwin

package platform

import "testing"

const darwinRouteIPv4 = `   route to: default
destination: default
       mask: default
    gateway: 192.168.1.1
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,GLOBAL>
 recvpipe  sendpipe  ssthresh  rtt,msec    rttvar  hopcount      mtu     expire
    2308         0         0         0         0         0      1500         0
`

const darwinRouteIPv6 = `   route to: ::
destination: ::
       mask: default
    gateway: fe80::1%en0
  interface: en0
      flags: <UP,GATEWAY,DONE,STATIC,PRCLONING,GLOBAL>
`

func TestParseDarwinRouteIPv4(t *testing.T) {
	route, found, err := parseDarwinRoute(darwinRouteIPv4, FamilyIPv4)
	if err != nil {
		t.Fatalf("parseDarwinRoute() error = %v", err)
	}
	if !found {
		t.Fatal("parseDarwinRoute() found no route")
	}
	if route.Gateway != "192.168.1.1" || route.Interface != "en0" || route.Family != FamilyIPv4 {
		t.Fatalf("route = %+v", route)
	}
}

func TestParseDarwinRouteIPv6(t *testing.T) {
	route, found, err := parseDarwinRoute(darwinRouteIPv6, FamilyIPv6)
	if err != nil {
		t.Fatalf("parseDarwinRoute() error = %v", err)
	}
	if !found {
		t.Fatal("parseDarwinRoute() found no route")
	}
	if route.Gateway != "fe80::1%en0" || route.Interface != "en0" || route.Family != FamilyIPv6 {
		t.Fatalf("route = %+v", route)
	}
}

func TestParseDarwinRouteWithoutGateway(t *testing.T) {
	_, found, err := parseDarwinRoute("destination: default\n   mask: default\n", FamilyIPv4)
	if err != nil {
		t.Fatalf("parseDarwinRoute() error = %v", err)
	}
	if found {
		t.Fatal("parseDarwinRoute() reported a route without a gateway")
	}
}

func TestParseDarwinRouteIncomplete(t *testing.T) {
	_, _, err := parseDarwinRoute("gateway: 192.168.1.1\n", FamilyIPv4)
	if err == nil {
		t.Fatal("parseDarwinRoute() accepted a route without an interface")
	}
}
