package domain

// CatalogVersion identifies the built-in preset target catalog. It is reported
// in every session and in the JSON export.
const CatalogVersion = "v1"

// PresetProtocol names one probe protocol enabled by default for a preset.
type PresetProtocol string

const (
	PresetICMP PresetProtocol = "icmp"
	PresetTCP  PresetProtocol = "tcp"
	PresetTLS  PresetProtocol = "tls"
	PresetHTTP PresetProtocol = "http"
)

// Preset is one curated entry of the built-in target catalog.
type Preset struct {
	ID          string           `json:"id"`
	DisplayName string           `json:"display_name"`
	Host        string           `json:"host"`
	Category    string           `json:"category"`
	Ports       []int            `json:"ports"`
	Protocols   []PresetProtocol `json:"protocols"`
}

// PresetCatalog returns the built-in target catalog. The returned slice and its
// nested slices are copies: callers cannot mutate the shared catalog. The
// catalog is compiled into the binary and performs no I/O.
func PresetCatalog() []Preset {
	presets := []Preset{
		{
			ID:          "youtube",
			DisplayName: "YouTube",
			Host:        "youtube.com",
			Category:    "video",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "telegram",
			DisplayName: "Telegram",
			Host:        "telegram.org",
			Category:    "messaging",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "discord",
			DisplayName: "Discord",
			Host:        "discord.com",
			Category:    "messaging",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "github",
			DisplayName: "GitHub",
			Host:        "github.com",
			Category:    "developer",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "google",
			DisplayName: "Google",
			Host:        "google.com",
			Category:    "search",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "cloudflare",
			DisplayName: "Cloudflare",
			Host:        "cloudflare.com",
			Category:    "infrastructure",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "wikipedia",
			DisplayName: "Wikipedia",
			Host:        "wikipedia.org",
			Category:    "reference",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "x",
			DisplayName: "X",
			Host:        "x.com",
			Category:    "social",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "tor",
			DisplayName: "Tor",
			Host:        "torproject.org",
			Category:    "anonymity",
			Ports:       []int{443},
			Protocols:   webProtocols(),
		},
		{
			ID:          "cloudflare-dns",
			DisplayName: "Cloudflare DNS",
			Host:        "1.1.1.1",
			Category:    "infrastructure",
			Ports:       []int{443, 53},
			Protocols:   webProtocols(),
		},
		{
			ID:          "google-dns",
			DisplayName: "Google DNS",
			Host:        "8.8.8.8",
			Category:    "infrastructure",
			Ports:       []int{443, 53},
			Protocols:   webProtocols(),
		},
	}

	out := make([]Preset, len(presets))
	for i, preset := range presets {
		preset.Ports = append([]int(nil), preset.Ports...)
		preset.Protocols = append([]PresetProtocol(nil), preset.Protocols...)
		out[i] = preset
	}
	return out
}

func webProtocols() []PresetProtocol {
	return []PresetProtocol{PresetICMP, PresetTCP, PresetTLS, PresetHTTP}
}
