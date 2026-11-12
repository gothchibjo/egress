package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestClassifyRun(t *testing.T) {
	tests := []struct {
		name    string
		results []Result
		scope   FailureScope
		summary string
	}{
		{
			name: "local link failure",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusFail),
				verdictResult("gateway.reach", "192.168.1.1", StatusPass),
			},
			scope:   ScopeLocalLink,
			summary: "Local link",
		},
		{
			name: "gateway failure",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusPass),
				verdictResult("gateway.reach", "192.168.1.1", StatusFail),
				verdictResult("dns.canary", "example.com", StatusPass),
			},
			scope:   ScopeGateway,
			summary: "gateway",
		},
		{
			name: "dns failure",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusPass),
				verdictResult("gateway.reach", "192.168.1.1", StatusPass),
				verdictResult("dns.canary", "example.com", StatusFail),
			},
			scope:   ScopeDNS,
			summary: "name resolution",
		},
		{
			name: "dns warning",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusPass),
				verdictResult("dns.canary", "example.com", StatusWarn),
			},
			scope:   ScopeDNS,
			summary: "name resolution",
		},
		{
			name: "upstream transport failure",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusPass),
				verdictResult("gateway.reach", "192.168.1.1", StatusPass),
				verdictResult("dns.canary", "example.com", StatusPass),
				verdictResult("tcp.anchor.example-com.443", "1.1.1.1:443", StatusFail),
			},
			scope:   ScopeUpstream,
			summary: "remote probes",
		},
		{
			name: "upstream http failure",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusPass),
				verdictResult("http.captive", "captive.apple.com", StatusFail),
			},
			scope:   ScopeUpstream,
			summary: "remote probes",
		},
		{
			name: "no failures",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusPass),
				verdictResult("gateway.reach", "192.168.1.1", StatusPass),
				verdictResult("dns.canary", "example.com", StatusPass),
			},
			scope:   ScopeNone,
			summary: "completed without failures",
		},
		{
			name: "skipped checks do not fail a layer",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusSkipped),
				verdictResult("dns.canary", "example.com", StatusPass),
			},
			scope:   ScopeNone,
			summary: "completed without failures",
		},
		{
			name: "local failure outranks gateway",
			results: []Result{
				verdictResult("local.interfaces", "en0", StatusFail),
				verdictResult("gateway.reach", "192.168.1.1", StatusFail),
			},
			scope: ScopeLocalLink,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			classification, err := ClassifyRun(test.results)
			if err != nil {
				t.Fatalf("ClassifyRun() error = %v", err)
			}
			if classification.Scope != test.scope {
				t.Fatalf("scope = %q, want %q", classification.Scope, test.scope)
			}
			if classification.Summary == "" {
				t.Fatal("classification has no summary")
			}
			if test.summary != "" && !strings.Contains(classification.Summary, test.summary) {
				t.Fatalf("summary = %q, want %q", classification.Summary, test.summary)
			}
		})
	}
}

func TestClassifyRunRequiresResults(t *testing.T) {
	if _, err := ClassifyRun(nil); !errors.Is(err, ErrNoResults) {
		t.Fatalf("ClassifyRun(nil) error = %v, want %v", err, ErrNoResults)
	}
}

func TestClassificationIgnoresUnknownChecks(t *testing.T) {
	classification, err := ClassifyRun([]Result{
		verdictResult("custom.check", "localhost", StatusFail),
		verdictResult("local.interfaces", "en0", StatusPass),
	})
	if err != nil {
		t.Fatalf("ClassifyRun() error = %v", err)
	}
	if classification.Scope != ScopeNone {
		t.Fatalf("scope = %q, want none", classification.Scope)
	}
}

func TestClassificationAvoidsCensorshipClaims(t *testing.T) {
	for _, results := range [][]Result{
		{verdictResult("local.interfaces", "en0", StatusFail)},
		{verdictResult("gateway.reach", "192.168.1.1", StatusFail)},
		{verdictResult("dns.canary", "example.com", StatusFail)},
		{verdictResult("tcp.anchor.example-com.443", "1.1.1.1:443", StatusFail)},
		{verdictResult("local.interfaces", "en0", StatusPass)},
	} {
		classification, err := ClassifyRun(results)
		if err != nil {
			t.Fatalf("ClassifyRun() error = %v", err)
		}
		summary := strings.ToLower(classification.Summary)
		for _, forbidden := range []string{"censor", "censorship", "blocked by"} {
			if strings.Contains(summary, forbidden) {
				t.Fatalf("summary claims %q: %q", forbidden, classification.Summary)
			}
		}
	}
}
