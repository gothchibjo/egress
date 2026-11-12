package platform

// Capabilities is the single abstraction probes use for host mechanisms.
// Implementations select the OS-appropriate mechanism and never change the
// meaning of a check.
type Capabilities interface {
	Platform() string
	Interfaces() ([]Interface, error)
	DefaultRoute(Family) (DefaultRoute, bool, error)
	Resolvers() ([]Resolver, error)
	ICMP() ICMPCapability
	Tracing() TracingCapability
}

// ICMPCapability reports which ICMP socket kinds the platform provides.
type ICMPCapability struct {
	Datagram bool
	Raw      bool
	Reason   string
}

// TracingCapability reports whether a traceroute mechanism exists in v1.
type TracingCapability struct {
	Supported bool
	Reason    string
}

type systemCapabilities struct {
	platform string
}

func (c systemCapabilities) Platform() string {
	return c.platform
}

func (c systemCapabilities) Interfaces() ([]Interface, error) {
	return ReadInterfaces()
}

func (c systemCapabilities) DefaultRoute(family Family) (DefaultRoute, bool, error) {
	return ReadDefaultRoute(family)
}

func (c systemCapabilities) Resolvers() ([]Resolver, error) {
	return ReadResolvers()
}

func (c systemCapabilities) ICMP() ICMPCapability {
	return icmpCapability()
}

func (c systemCapabilities) Tracing() TracingCapability {
	return tracingCapability()
}
