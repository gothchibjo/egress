//go:build !darwin && !linux

package platform

// ReadDefaultRoute reports that this platform has no v1 route adapter.
func ReadDefaultRoute(Family) (DefaultRoute, bool, error) {
	return DefaultRoute{}, false, ErrUnsupported
}
