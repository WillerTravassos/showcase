// Package config reads the patient-web configuration from environment variables once,
// at startup.
package config

import "os"

// defaultAddr is the listen address when HTTP_ADDR is unset.
const defaultAddr = ":8080"

// Config is the patient-web configuration.
type Config struct {
	// Addr is the TCP address the HTTP server listens on (HTTP_ADDR, default ":8080").
	Addr string
}

// Load returns the configuration from the environment.
func Load() Config {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = defaultAddr
	}

	return Config{Addr: addr}
}
