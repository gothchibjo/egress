package platform

import (
	"testing"
)

func lookupFrom(values map[string]string) func(string) (string, bool) {
	return func(name string) (string, bool) {
		value, ok := values[name]
		return value, ok
	}
}

func TestDetectProxyEnvEmpty(t *testing.T) {
	config := detectProxyEnv(lookupFrom(nil))
	if !config.Empty() {
		t.Fatalf("settings = %+v, want none", config.Settings)
	}
	if config.Settings == nil {
		t.Fatal("settings must be an empty slice, not nil")
	}
}

func TestDetectProxyEnvUppercase(t *testing.T) {
	config := detectProxyEnv(lookupFrom(map[string]string{
		"HTTP_PROXY":  "http://proxy.example:8080",
		"HTTPS_PROXY": "http://secure.example:8443",
		"ALL_PROXY":   "socks5://socks.example:1080",
		"NO_PROXY":    "localhost,127.0.0.1",
	}))

	if len(config.Settings) != 4 {
		t.Fatalf("settings = %d, want 4", len(config.Settings))
	}
	want := map[string]string{
		"HTTP_PROXY":  "http://proxy.example:8080",
		"HTTPS_PROXY": "http://secure.example:8443",
		"ALL_PROXY":   "socks5://socks.example:1080",
		"NO_PROXY":    "localhost,127.0.0.1",
	}
	for _, setting := range config.Settings {
		if setting.Value != want[setting.Name] {
			t.Errorf("%s = %q, want %q", setting.Name, setting.Value, want[setting.Name])
		}
		if setting.Source != setting.Name {
			t.Errorf("%s source = %q", setting.Name, setting.Source)
		}
	}
}

func TestDetectProxyEnvLowercaseSources(t *testing.T) {
	config := detectProxyEnv(lookupFrom(map[string]string{
		"http_proxy":  "http://lower.example:8080",
		"https_proxy": "http://lower-secure.example:8443",
	}))

	if len(config.Settings) != 2 {
		t.Fatalf("settings = %d, want 2", len(config.Settings))
	}
	if config.Settings[0].Source != "http_proxy" || config.Settings[0].Name != "HTTP_PROXY" {
		t.Fatalf("first setting = %+v", config.Settings[0])
	}
	if config.Settings[1].Source != "https_proxy" || config.Settings[1].Name != "HTTPS_PROXY" {
		t.Fatalf("second setting = %+v", config.Settings[1])
	}
}

func TestDetectProxyEnvReportsBothCaseVariants(t *testing.T) {
	config := detectProxyEnv(lookupFrom(map[string]string{
		"HTTP_PROXY": "http://upper.example:8080",
		"http_proxy": "http://lower.example:8080",
	}))

	if len(config.Settings) != 2 {
		t.Fatalf("settings = %d, want 2", len(config.Settings))
	}
	if config.Settings[0].Value != "http://upper.example:8080" {
		t.Fatalf("uppercase value = %q", config.Settings[0].Value)
	}
	if config.Settings[1].Value != "http://lower.example:8080" {
		t.Fatalf("lowercase value = %q", config.Settings[1].Value)
	}
}

func TestDetectProxyEnvRedactsCredentials(t *testing.T) {
	config := detectProxyEnv(lookupFrom(map[string]string{
		"HTTPS_PROXY": "http://user:secret@proxy.example:8443",
	}))

	if len(config.Settings) != 1 {
		t.Fatalf("settings = %d, want 1", len(config.Settings))
	}
	if config.Settings[0].Value != "http://***@proxy.example:8443" {
		t.Fatalf("value = %q, credentials were not redacted", config.Settings[0].Value)
	}
}

func TestDetectProxyEnvReadsProcessEnvironment(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://process.example:8080")

	config := DetectProxyEnv()
	found := false
	for _, setting := range config.Settings {
		if setting.Source == "HTTP_PROXY" && setting.Value == "http://process.example:8080" {
			found = true
		}
	}
	if !found {
		t.Fatalf("HTTP_PROXY not read from the process environment: %+v", config.Settings)
	}
}
