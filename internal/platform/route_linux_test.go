//go:build linux

package platform

import "testing"

const linuxRoute4 = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
en0	00000000	0101A8C0	0003	0	0	0	00000000	0	0	0
en0	0001A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`

const linuxRoute4NoDefault = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
en0	0001A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
`

const linuxRoute4Down = `Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
en0	00000000	0101A8C0	0002	0	0	0	00000000	0	0	0
`

const linuxRoute6 = `00000000000000000000000000000000 00 00000000000000000000000000000000 00 fe800000000000000000000000000001 00000400 00000000 00000000 00000003 eth0
`

const linuxRoute6OnLink = `00000000000000000000000000000000 00 00000000000000000000000000000000 00 00000000000000000000000000000000 00000400 00000000 00000000 00000003 wg0
`

func TestParseLinuxRoute4(t *testing.T) {
	route, found, err := parseLinuxRoute4([]byte(linuxRoute4))
	if err != nil {
		t.Fatalf("parseLinuxRoute4() error = %v", err)
	}
	if !found {
		t.Fatal("parseLinuxRoute4() found no default route")
	}
	if route.Gateway != "192.168.1.1" || route.Interface != "en0" || route.Family != FamilyIPv4 {
		t.Fatalf("route = %+v", route)
	}
}

func TestParseLinuxRoute4NoDefault(t *testing.T) {
	_, found, err := parseLinuxRoute4([]byte(linuxRoute4NoDefault))
	if err != nil {
		t.Fatalf("parseLinuxRoute4() error = %v", err)
	}
	if found {
		t.Fatal("parseLinuxRoute4() invented a default route")
	}
}

func TestParseLinuxRoute4SkipsDownRoute(t *testing.T) {
	_, found, err := parseLinuxRoute4([]byte(linuxRoute4Down))
	if err != nil {
		t.Fatalf("parseLinuxRoute4() error = %v", err)
	}
	if found {
		t.Fatal("parseLinuxRoute4() accepted a route without the UP flag")
	}
}

func TestParseLinuxRoute4Malformed(t *testing.T) {
	if _, _, err := parseLinuxRoute4([]byte("header\nbroken\n")); err == nil {
		t.Fatal("parseLinuxRoute4() accepted a malformed line")
	}
}

func TestParseLinuxRoute6(t *testing.T) {
	route, found, err := parseLinuxRoute6([]byte(linuxRoute6))
	if err != nil {
		t.Fatalf("parseLinuxRoute6() error = %v", err)
	}
	if !found {
		t.Fatal("parseLinuxRoute6() found no default route")
	}
	if route.Gateway != "fe80::1" || route.Interface != "eth0" || route.Family != FamilyIPv6 {
		t.Fatalf("route = %+v", route)
	}
}

func TestParseLinuxRoute6OnLink(t *testing.T) {
	route, found, err := parseLinuxRoute6([]byte(linuxRoute6OnLink))
	if err != nil {
		t.Fatalf("parseLinuxRoute6() error = %v", err)
	}
	if !found {
		t.Fatal("parseLinuxRoute6() found no on-link default route")
	}
	if route.Gateway != "" || route.Interface != "wg0" {
		t.Fatalf("route = %+v", route)
	}
}
