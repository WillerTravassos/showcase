// Package config reads the patient-web configuration from environment variables once,
// at startup.
package config

import "os"

const (
	// defaultAddr is the listen address when HTTP_ADDR is unset.
	defaultAddr = ":8080"
	// defaultProbeAddr is the probe listen address when PROBE_ADDR is unset.
	defaultProbeAddr = ":8081"
)

// Config is the patient-web configuration.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (HTTP_ADDR, default ":8080").
	Addr string
	// ProbeAddr is the TCP address of the liveness and readiness probes (PROBE_ADDR,
	// default ":8081"). Only the kubelet reaches it; the gateway never routes to it.
	ProbeAddr string
}

// Load returns the configuration from the environment.
func Load() Config {
	return Config{Addr: env("HTTP_ADDR", defaultAddr), ProbeAddr: env("PROBE_ADDR", defaultProbeAddr)}
}

// env returns the environment variable key, or fallback when it is unset or empty.
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}

	return fallback
}
