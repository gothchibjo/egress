package domain

// Anchor is a curated destination used by the baseline.
type Anchor struct {
	ID          string `json:"id"`
	Vendor      string `json:"vendor"`
	DisplayName string `json:"display_name"`
	Host        string `json:"host"`
	Purpose     string `json:"purpose"`
}

// BaselineAnchors returns the curated multi-vendor anchor set. The returned
// slice is a copy; callers cannot mutate the shared catalog.
func BaselineAnchors() []Anchor {
	return []Anchor{
		{
			ID:          "cloudflare-dns",
			Vendor:      "Cloudflare",
			DisplayName: "Cloudflare DNS",
			Host:        "1.1.1.1",
			Purpose:     "public resolver and reachability anchor",
		},
		{
			ID:          "google-dns",
			Vendor:      "Google",
			DisplayName: "Google DNS",
			Host:        "8.8.8.8",
			Purpose:     "public resolver and reachability anchor",
		},
		{
			ID:          "apple-captive",
			Vendor:      "Apple",
			DisplayName: "Apple captive-portal canary",
			Host:        "captive.apple.com",
			Purpose:     "captive-portal HTTP canary",
		},
		{
			ID:          "example-com",
			Vendor:      "IANA",
			DisplayName: "example.com",
			Host:        "example.com",
			Purpose:     "neutral DNS and HTTP anchor",
		},
	}
}
