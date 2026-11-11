//go:build !darwin && !linux

package platform

// ReadResolvers reports that this platform has no v1 resolver adapter.
func ReadResolvers() ([]Resolver, error) {
	return nil, ErrUnsupported
}
