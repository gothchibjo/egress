package probe

import "github.com/gothchibjo/egress/internal/app"

// BaselineOptions configures the baseline check list.
type BaselineOptions struct {
	// Elevated enables raw-socket checks. It must come from an explicitly
	// elevated session; the baseline never escalates on its own.
	Elevated bool
}

// BaselineChecks returns the full baseline in path-layer order.
func BaselineChecks(opts BaselineOptions) []app.Check {
	checks := []app.Check{NewLocalCheck()}
	checks = append(checks, NewGatewayCheck(ICMPOptions{Elevated: opts.Elevated}))
	checks = append(checks, NewDNSCheck())
	checks = append(checks, NewAnchorChecks(ICMPOptions{Elevated: opts.Elevated})...)
	checks = append(checks, NewTCPChecks(TCPOptions{})...)
	checks = append(checks, NewMTUCheck(MTUOptions{Elevated: opts.Elevated}))
	checks = append(checks, NewPortalCheck())
	return checks
}
