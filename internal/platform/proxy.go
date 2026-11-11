// Package platform hosts OS and environment detection shared by probes.
package platform

import (
	"os"
	"strings"
)

// ProxySetting is one detected proxy environment variable.
type ProxySetting struct {
	Name   string `json:"name"`
	Source string `json:"source"`
	Value  string `json:"value"`
}

// ProxyConfig is the detected proxy environment. An empty Settings slice means
// no proxy variable is set.
type ProxyConfig struct {
	Settings []ProxySetting `json:"settings"`
}

// Empty reports whether no proxy variable is set.
func (c ProxyConfig) Empty() bool {
	return len(c.Settings) == 0
}

// DetectProxyEnv reads proxy variables from the process environment.
func DetectProxyEnv() ProxyConfig {
	return detectProxyEnv(os.LookupEnv)
}

func detectProxyEnv(lookup func(string) (string, bool)) ProxyConfig {
	variables := []struct {
		name    string
		aliases []string
	}{
		{name: "HTTP_PROXY", aliases: []string{"HTTP_PROXY", "http_proxy"}},
		{name: "HTTPS_PROXY", aliases: []string{"HTTPS_PROXY", "https_proxy"}},
		{name: "ALL_PROXY", aliases: []string{"ALL_PROXY", "all_proxy"}},
		{name: "NO_PROXY", aliases: []string{"NO_PROXY", "no_proxy"}},
	}

	config := ProxyConfig{Settings: []ProxySetting{}}
	for _, variable := range variables {
		for _, alias := range variable.aliases {
			value, ok := lookup(alias)
			if !ok || value == "" {
				continue
			}
			config.Settings = append(config.Settings, ProxySetting{
				Name:   variable.name,
				Source: alias,
				Value:  redactProxyValue(value),
			})
		}
	}
	return config
}

// redactProxyValue hides credentials embedded in a proxy URL while keeping the
// scheme, host and port visible.
func redactProxyValue(value string) string {
	scheme := strings.Index(value, "://")
	at := strings.LastIndex(value, "@")
	if scheme < 0 || at < scheme+3 {
		return value
	}
	return value[:scheme+3] + "***" + value[at:]
}
