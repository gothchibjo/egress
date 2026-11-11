//go:build darwin || linux

package platform

import "testing"

func TestReadResolvers(t *testing.T) {
	resolvers, err := ReadResolvers()
	if err != nil {
		t.Fatalf("ReadResolvers() error = %v", err)
	}
	for _, resolver := range resolvers {
		if resolver.Address == "" {
			t.Fatalf("resolver has no address: %+v", resolver)
		}
	}
}
