// Package config loads runtime configuration from the environment.
package config

import "os"

// Config holds the settings needed to wire up the redaction engine.
type Config struct {
	// Port is the HTTP listen port for the API server.
	Port string
	// PhoneRegion is the default ISO 3166-1 alpha-2 region used to
	// interpret phone numbers written without an explicit country code.
	PhoneRegion string
	// NLPWorkerAddr is the address of the Python NLP worker (Presidio)
	// gRPC service. It is reserved for the next phase of this project;
	// while empty, the engine runs with grpcclient.NoOpClient and detects
	// only structured PII.
	NLPWorkerAddr string
}

// Load reads Config from environment variables, falling back to sane
// defaults for local development.
func Load() Config {
	return Config{
		Port:          getEnv("PORT", "8080"),
		PhoneRegion:   getEnv("PHONE_DEFAULT_REGION", "US"),
		NLPWorkerAddr: getEnv("NLP_WORKER_ADDR", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
