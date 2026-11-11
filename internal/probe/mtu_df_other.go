//go:build !darwin && !linux

package probe

import "errors"

// setDontFragment reports that this platform has no v1 DF socket option.
func setDontFragment(int) error {
	return errors.New("DF socket option is not supported on this platform")
}
