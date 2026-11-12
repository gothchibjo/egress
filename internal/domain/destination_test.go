package domain

import (
	"errors"
	"testing"
)

func TestDestinationValidate(t *testing.T) {
	valid := Destination{
		Kind:      DestinationAnchor,
		Host:      "1.1.1.1",
		Port:      443,
		Transport: TransportTCP,
		Purpose:   "TCP reachability to the anchor",
	}
	if err := valid.Validate(); err != nil {
		t.Fatalf("valid destination rejected: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*Destination)
	}{
		{name: "unknown kind", mutate: func(d *Destination) { d.Kind = "telemetry" }},
		{name: "missing host", mutate: func(d *Destination) { d.Host = " " }},
		{name: "host with separator", mutate: func(d *Destination) { d.Host = "1.1.1.1/path" }},
		{name: "port out of range", mutate: func(d *Destination) { d.Port = 70000 }},
		{name: "negative port", mutate: func(d *Destination) { d.Port = -1 }},
		{name: "unknown transport", mutate: func(d *Destination) { d.Transport = "quic" }},
		{name: "missing port", mutate: func(d *Destination) { d.Port = 0 }},
		{name: "missing purpose", mutate: func(d *Destination) { d.Purpose = "" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			destination := valid
			test.mutate(&destination)
			err := destination.Validate()
			if !errors.Is(err, ErrInvalidDestination) {
				t.Fatalf("Validate() error = %v, want %v", err, ErrInvalidDestination)
			}
		})
	}
}

func TestDestinationValidateAllowsPortlessICMP(t *testing.T) {
	destination := Destination{
		Kind:      DestinationAnchor,
		Host:      "example.com",
		Transport: TransportICMP,
		Purpose:   "ICMP reachability and latency sample",
	}
	if err := destination.Validate(); err != nil {
		t.Fatalf("portless ICMP destination rejected: %v", err)
	}
}

func TestDestinationEndpoint(t *testing.T) {
	tests := []struct {
		destination Destination
		want        string
	}{
		{destination: Destination{Host: "1.1.1.1", Port: 443}, want: "1.1.1.1:443"},
		{destination: Destination{Host: "example.com"}, want: "example.com"},
	}
	for _, test := range tests {
		if got := test.destination.Endpoint(); got != test.want {
			t.Fatalf("Endpoint() = %q, want %q", got, test.want)
		}
	}
}
