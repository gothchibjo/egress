package domain

import (
	"testing"
)

func sessionDNSStep(answer string) StepResult {
	return StepResult{
		ID: "dns", Kind: StepDNS, Status: StatusPass,
		Measurements: []Measurement{{Name: "system.answers", Value: answer}},
	}
}

func TestBuildSessionDestinationsListsTargetsAndResolvers(t *testing.T) {
	target := ProbeTarget{
		ID: "alpha", Label: "Alpha", Host: "alpha.example",
		Ports:     []int{443},
		Protocols: []PresetProtocol{PresetTCP, PresetTLS, PresetHTTP, PresetICMP},
	}
	documented := []Destination{
		{Kind: DestinationResolver, Host: "192.0.2.53", Port: 53, Transport: TransportUDPAndTCP, Purpose: "resolver"},
		{Kind: DestinationAnchor, Host: "1.1.1.1", Transport: TransportICMP, Purpose: "anchor"},
		{Kind: DestinationDoH, Host: "cloudflare-dns.com", Port: 443, Transport: TransportHTTPS, Purpose: "doh"},
	}
	destinations := BuildSessionDestinations(
		[]ProbeTarget{target},
		map[string][]StepResult{target.ID: {sessionDNSStep("203.0.113.10")}},
		documented,
	)

	targets := 0
	kinds := make(map[SessionDestinationKind]int)
	for _, destination := range destinations {
		kinds[destination.Kind]++
		if destination.Kind == SessionTarget {
			targets++
			if destination.Address != "203.0.113.10" {
				t.Fatalf("target address = %q", destination.Address)
			}
		}
		if string(destination.Kind) == "anchor" {
			t.Fatal("the session lists a baseline anchor")
		}
	}
	if targets != 5 {
		t.Fatalf("target rows = %d, want tcp/tls/http/icmp/udp: %+v", targets, destinations)
	}
	if kinds[SessionResolver] != 1 || kinds[SessionDoH] != 1 {
		t.Fatalf("resolver rows = %+v", destinations)
	}
	for _, destination := range destinations {
		if err := destination.Validate(); err != nil {
			t.Fatalf("invalid destination %+v: %v", destination, err)
		}
	}
}

func TestBuildSessionDestinationsUsesAnIPLiteralAsTheAddress(t *testing.T) {
	target := ProbeTarget{ID: "ip", Host: "198.51.100.7", Ports: []int{443}, Protocols: []PresetProtocol{PresetTCP}}
	destinations := BuildSessionDestinations([]ProbeTarget{target}, nil, nil)

	if len(destinations) != 2 {
		t.Fatalf("destinations = %+v", destinations)
	}
	if destinations[0].Address != "198.51.100.7" || destinations[0].Host != "198.51.100.7" {
		t.Fatalf("IP literal target = %+v", destinations[0])
	}
}

func TestBuildSessionDestinationsPrefersTheTracedAddress(t *testing.T) {
	target := ProbeTarget{ID: "alpha", Host: "alpha.example", Ports: []int{443}, Protocols: []PresetProtocol{PresetTCP}}
	steps := []StepResult{
		sessionDNSStep("203.0.113.10, 203.0.113.11"),
		{ID: "trace", Kind: StepTrace, Status: StatusPass,
			Measurements: []Measurement{{Name: "address", Value: "203.0.113.11"}}},
	}
	destinations := BuildSessionDestinations([]ProbeTarget{target}, map[string][]StepResult{target.ID: steps}, nil)
	if destinations[0].Address != "203.0.113.11" {
		t.Fatalf("target address = %q", destinations[0].Address)
	}
}

func TestBuildSessionDestinationsSortsAndDeduplicates(t *testing.T) {
	target := ProbeTarget{ID: "alpha", Host: "alpha.example", Ports: []int{443}, Protocols: []PresetProtocol{PresetTCP}}
	destinations := BuildSessionDestinations(
		[]ProbeTarget{target, target},
		nil,
		[]Destination{
			{Kind: DestinationDoH, Host: "cloudflare-dns.com", Port: 443, Transport: TransportHTTPS, Purpose: "doh"},
			{Kind: DestinationResolver, Host: "192.0.2.53", Port: 53, Transport: TransportUDPAndTCP, Purpose: "resolver"},
		},
	)

	if len(destinations) != 4 {
		t.Fatalf("duplicates were not removed: %+v", destinations)
	}
	wantKinds := []SessionDestinationKind{SessionTarget, SessionTarget, SessionResolver, SessionDoH}
	for i, want := range wantKinds {
		if destinations[i].Kind != want {
			t.Fatalf("destination %d = %q, want %q", i, destinations[i].Kind, want)
		}
	}
}

func TestSessionDestinationValidate(t *testing.T) {
	valid := SessionDestination{
		Kind: SessionTarget, Host: "alpha.example", Address: "203.0.113.10", Port: 443,
		Transport: SessionTLS, Purpose: "TLS handshake",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid destination rejected: %v", err)
	}
	cases := map[string]SessionDestination{
		"kind":      {Kind: "anchor", Host: "alpha.example", Transport: SessionICMP, Purpose: "x"},
		"host":      {Kind: SessionTarget, Host: "", Transport: SessionICMP, Purpose: "x"},
		"address":   {Kind: SessionTarget, Host: "alpha.example", Address: "not-an-ip", Transport: SessionICMP, Purpose: "x"},
		"port":      {Kind: SessionTarget, Host: "alpha.example", Port: 70000, Transport: SessionTCP, Purpose: "x"},
		"transport": {Kind: SessionTarget, Host: "alpha.example", Transport: "icmp-df", Purpose: "x"},
		"purpose":   {Kind: SessionTarget, Host: "alpha.example", Transport: SessionICMP},
		"no port":   {Kind: SessionTarget, Host: "alpha.example", Transport: SessionTCP, Purpose: "x"},
	}
	for name, destination := range cases {
		t.Run(name, func(t *testing.T) {
			if err := destination.Validate(); err == nil {
				t.Fatalf("accepted invalid destination %+v", destination)
			}
		})
	}
}
