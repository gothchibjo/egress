package domain

import (
	"errors"
	"testing"
)

func TestParseTargetValid(t *testing.T) {
	tests := []struct {
		input string
		want  Target
	}{
		{input: "example.com", want: Target{Host: "example.com"}},
		{input: "  Example.COM  ", want: Target{Host: "example.com"}},
		{input: "example.com:8443", want: Target{Host: "example.com", Port: 8443}},
		{input: "1.2.3.4:53", want: Target{Host: "1.2.3.4", Port: 53}},
		{input: "2001:db8::1", want: Target{Host: "2001:db8::1"}},
		{input: "[2001:db8::1]", want: Target{Host: "2001:db8::1"}},
		{input: "[2001:db8::1]:443", want: Target{Host: "2001:db8::1", Port: 443}},
		{input: "example.com@tcp", want: Target{Host: "example.com", Protocol: PresetTCP}},
		{input: "example.com:8443@tls", want: Target{Host: "example.com", Port: 8443, Protocol: PresetTLS}},
		{input: "localhost", want: Target{Host: "localhost"}},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			got, err := ParseTarget(test.input)
			if err != nil {
				t.Fatalf("ParseTarget(%q) error = %v", test.input, err)
			}
			if got != test.want {
				t.Fatalf("ParseTarget(%q) = %+v, want %+v", test.input, got, test.want)
			}
		})
	}
}

func TestParseTargetInvalid(t *testing.T) {
	tests := []string{
		"",
		"   ",
		"https://example.com",
		"http://example.com:8080/path",
		"example.com/path",
		"example.com?query",
		"example.com#fragment",
		"exa mple.com",
		"example.com:0",
		"example.com:65536",
		"example.com:abc",
		"example.com:",
		"example.com@quic",
		"-bad.example.com",
		"bad-.example.com",
		"a..b",
		"[not-ipv6]:443",
	}
	for _, input := range tests {
		t.Run(input, func(t *testing.T) {
			if _, err := ParseTarget(input); !errors.Is(err, ErrInvalidTarget) {
				t.Fatalf("ParseTarget(%q) error = %v, want %v", input, err, ErrInvalidTarget)
			}
		})
	}
}

func TestParseTargetRejectsURLs(t *testing.T) {
	_, err := ParseTarget("https://example.com")
	if err == nil || !errors.Is(err, ErrInvalidTarget) {
		t.Fatalf("error = %v", err)
	}
	if got := err.Error(); !contains(got, "full URLs") {
		t.Fatalf("error does not name the limitation: %q", got)
	}
}

func TestTargetKeyIsCaseInsensitive(t *testing.T) {
	first, err := ParseTarget("Example.COM:443@TLS")
	if err != nil {
		t.Fatalf("ParseTarget() error = %v", err)
	}
	second, err := ParseTarget("example.com:443@tls")
	if err != nil {
		t.Fatalf("ParseTarget() error = %v", err)
	}
	if first.Key() != second.Key() {
		t.Fatalf("keys differ: %q vs %q", first.Key(), second.Key())
	}
}

func TestTargetStringRoundTrip(t *testing.T) {
	inputs := []string{"example.com", "example.com:8443", "[2001:db8::1]:443", "1.2.3.4:53@tcp"}
	for _, input := range inputs {
		target, err := ParseTarget(input)
		if err != nil {
			t.Fatalf("ParseTarget(%q) error = %v", input, err)
		}
		again, err := ParseTarget(target.String())
		if err != nil {
			t.Fatalf("ParseTarget(%q) error = %v", target.String(), err)
		}
		if again != target {
			t.Fatalf("round trip changed %+v into %+v", target, again)
		}
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}
