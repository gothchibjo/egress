// Package probe implements the baseline checks. It depends on the domain model
// and read-only platform adapters.
package probe

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gothchibjo/egress/internal/domain"
	"github.com/gothchibjo/egress/internal/platform"
)

// LinkSource supplies read-only host network state.
type LinkSource interface {
	Platform() string
	Interfaces() ([]platform.Interface, error)
	DefaultRoute(platform.Family) (platform.DefaultRoute, bool, error)
}

var _ LinkSource = (platform.Capabilities)(nil)

// LocalCheck reports interfaces, link state, addresses and the default route.
type LocalCheck struct {
	source LinkSource
	now    func() time.Time
}

// NewLocalCheck returns the local link check backed by the host platform.
func NewLocalCheck() LocalCheck {
	return LocalCheck{source: platform.System(), now: time.Now}
}

// NewLocalCheckWithSource builds the check with an injected source and clock.
func NewLocalCheckWithSource(source LinkSource, now func() time.Time) LocalCheck {
	if source == nil {
		source = platform.System()
	}
	if now == nil {
		now = time.Now
	}
	return LocalCheck{source: source, now: now}
}

// ID returns the stable check id used by the dashboard and JSON output.
func (c LocalCheck) ID() domain.CheckID {
	return "local.interfaces"
}

// Run inspects the host read-only and reports every observation.
func (c LocalCheck) Run(ctx context.Context) domain.Result {
	result := domain.Result{
		CheckID:   c.ID(),
		Target:    "local host",
		Method:    "interface and route inspection",
		Timestamp: c.now(),
	}
	if err := ctx.Err(); err != nil {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "inspection cancelled before it started"
		return result
	}

	result.Measurements = append(result.Measurements, domain.Measurement{Name: "platform", Value: c.source.Platform()})

	var issues []string
	interfaces, interfaceErr := c.source.Interfaces()
	if errors.Is(interfaceErr, platform.ErrUnsupported) {
		result.Status = domain.StatusSkipped
		result.Measurements = append(result.Measurements, domain.Measurement{Name: "interfaces", Value: "unsupported"})
		result.Diagnostic = "interface inspection is not supported on this platform"
		return result
	}
	active := false
	for _, iface := range interfaces {
		state := "down"
		if iface.Up {
			state = "up"
		}
		result.Measurements = append(result.Measurements,
			domain.Measurement{Name: "interface." + iface.Name + ".state", Value: state},
			domain.Measurement{Name: "interface." + iface.Name + ".addresses", Value: joinOrNone(iface.Addresses)},
		)
		if iface.Up && !iface.Loopback && len(iface.Addresses) > 0 {
			active = true
		}
	}
	if interfaceErr != nil {
		result.Measurements = append(result.Measurements, domain.Measurement{Name: "interfaces", Value: "unavailable"})
		issues = append(issues, "interfaces unavailable")
		result.Error = interfaceErr.Error()
	}
	if !active && interfaceErr == nil {
		issues = append(issues, "no active non-loopback interface")
	}

	hasRoute := false
	unsupportedRoutes := 0
	var routeErrors []string
	for _, family := range []platform.Family{platform.FamilyIPv4, platform.FamilyIPv6} {
		route, found, err := c.source.DefaultRoute(family)
		switch {
		case errors.Is(err, platform.ErrUnsupported):
			unsupportedRoutes++
			result.Measurements = append(result.Measurements, domain.Measurement{
				Name:  "default_route." + family.String(),
				Value: "unsupported",
			})
		case err != nil:
			routeErrors = append(routeErrors, fmt.Sprintf("%s: %v", family, err))
			result.Measurements = append(result.Measurements, domain.Measurement{
				Name:  "default_route." + family.String(),
				Value: "unavailable",
			})
		case found:
			hasRoute = true
			value := "on-link via " + route.Interface
			if route.Gateway != "" {
				value = route.Gateway + " via " + route.Interface
			}
			result.Measurements = append(result.Measurements, domain.Measurement{
				Name:  "default_route." + family.String(),
				Value: value,
			})
		default:
			result.Measurements = append(result.Measurements, domain.Measurement{
				Name:  "default_route." + family.String(),
				Value: "none",
			})
		}
	}
	if unsupportedRoutes == 2 {
		result.Status = domain.StatusSkipped
		result.Diagnostic = "default route inspection is not supported on this platform"
		return result
	}
	if len(routeErrors) > 0 {
		if result.Error != "" {
			result.Error += "; "
		}
		result.Error += strings.Join(routeErrors, "; ")
	}
	if !hasRoute && len(routeErrors) == 0 {
		issues = append(issues, "no default route")
	}

	if len(issues) > 0 || result.Error != "" {
		result.Status = domain.StatusFail
		result.Diagnostic = strings.Join(issues, "; ")
	} else {
		result.Status = domain.StatusPass
	}
	return result
}

func joinOrNone(values []string) string {
	if len(values) == 0 {
		return "none"
	}
	return strings.Join(values, ", ")
}
