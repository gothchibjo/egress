package domain

// SessionMode labels how much host capability a session has.
type SessionMode string

const (
	// ModeLimited is an unprivileged session: UDP probing only.
	ModeLimited SessionMode = "limited"
	// ModeElevated is a session started with sufficient privileges.
	ModeElevated SessionMode = "elevated"
)

// Hop is one TTL in the path table.
type Hop struct {
	TTL     int    `json:"ttl"`
	Address string `json:"address,omitempty"`
	// Addresses lists every distinct router that answered at this TTL.
	Addresses   []string `json:"addresses,omitempty"`
	Sent        int      `json:"sent"`
	Received    int      `json:"received"`
	RTTMinMS    int64    `json:"rtt_min_ms,omitempty"`
	RTTMedianMS int64    `json:"rtt_median_ms,omitempty"`
	RTTMaxMS    int64    `json:"rtt_max_ms,omitempty"`
}

// LossPercent is the share of probes without a reply.
func (h Hop) LossPercent() int {
	if h.Sent == 0 {
		return 0
	}
	return (h.Sent - h.Received) * 100 / h.Sent
}

// AllAddresses lists every router that answered at this TTL.
func (h Hop) AllAddresses() []string {
	if len(h.Addresses) > 0 {
		return h.Addresses
	}
	if h.Address != "" {
		return []string{h.Address}
	}
	return nil
}
