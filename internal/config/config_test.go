package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	cfg := Load()
	if cfg.Port != "8080" {
		t.Errorf("expected default port 8080, got %q", cfg.Port)
	}
	if cfg.PhoneRegion != "US" {
		t.Errorf("expected default phone region US, got %q", cfg.PhoneRegion)
	}
	if cfg.NLPWorkerAddr != "" {
		t.Errorf("expected empty default NLP worker address, got %q", cfg.NLPWorkerAddr)
	}
}

func TestLoadFromEnv(t *testing.T) {
	t.Setenv("PORT", "9090")
	t.Setenv("PHONE_DEFAULT_REGION", "GB")
	t.Setenv("NLP_WORKER_ADDR", "localhost:50051")

	cfg := Load()
	if cfg.Port != "9090" {
		t.Errorf("expected port 9090, got %q", cfg.Port)
	}
	if cfg.PhoneRegion != "GB" {
		t.Errorf("expected phone region GB, got %q", cfg.PhoneRegion)
	}
	if cfg.NLPWorkerAddr != "localhost:50051" {
		t.Errorf("expected NLP worker addr localhost:50051, got %q", cfg.NLPWorkerAddr)
	}
}
