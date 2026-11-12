package platform

// Privilege describes how the process may probe the host.
type Privilege struct {
	Elevated bool
	RawICMP  bool
	Reason   string
}
