package domain

import "testing"

func TestBaselineAnchors(t *testing.T) {
	anchors := BaselineAnchors()
	if len(anchors) != 4 {
		t.Fatalf("anchors = %d, want 4", len(anchors))
	}

	wantHosts := map[string]bool{
		"1.1.1.1":           true,
		"8.8.8.8":           true,
		"captive.apple.com": true,
		"example.com":       true,
	}
	seenHosts := make(map[string]bool, len(anchors))
	seenIDs := make(map[string]bool, len(anchors))
	vendors := make(map[string]bool, len(anchors))

	for _, anchor := range anchors {
		if anchor.ID == "" || anchor.Vendor == "" || anchor.DisplayName == "" || anchor.Host == "" || anchor.Purpose == "" {
			t.Errorf("anchor has an empty required field: %+v", anchor)
		}
		if seenIDs[anchor.ID] {
			t.Errorf("duplicate anchor id %q", anchor.ID)
		}
		seenIDs[anchor.ID] = true
		seenHosts[anchor.Host] = true
		vendors[anchor.Vendor] = true
	}

	for host := range wantHosts {
		if !seenHosts[host] {
			t.Errorf("missing anchor host %q", host)
		}
	}
	if len(vendors) < 3 {
		t.Errorf("anchors use %d vendors, want a multi-vendor set", len(vendors))
	}
}

func TestBaselineAnchorsReturnsCopy(t *testing.T) {
	first := BaselineAnchors()
	first[0].Host = "mutated.example"

	second := BaselineAnchors()
	if second[0].Host == "mutated.example" {
		t.Fatal("BaselineAnchors() returned shared mutable state")
	}
}
