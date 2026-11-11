//go:build darwin || linux

package platform

import "testing"

func TestReadDefaultRoute(t *testing.T) {
	for _, family := range []Family{FamilyIPv4, FamilyIPv6} {
		route, found, err := ReadDefaultRoute(family)
		if err != nil {
			t.Fatalf("ReadDefaultRoute(%s) error = %v", family, err)
		}
		if !found {
			continue
		}
		if route.Interface == "" {
			t.Fatalf("ReadDefaultRoute(%s) returned no interface: %+v", family, route)
		}
		if route.Family != family {
			t.Fatalf("ReadDefaultRoute(%s) family = %v", family, route.Family)
		}
	}
}
