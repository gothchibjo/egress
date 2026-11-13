package domain

// ProbeTarget is one destination the session probes.
type ProbeTarget struct {
	ID        string
	Label     string
	Host      string
	Ports     []int
	Protocols []PresetProtocol
}
