package config_test

import (
	"testing"

	"github.com/blacken57/heimdall/internal/config"
)

func TestLoad_NoServices(t *testing.T) {
	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error when no services configured")
	}
}

func TestLoad_SingleService(t *testing.T) {
	t.Setenv("SERVICE_1_NAME", "Test")
	t.Setenv("SERVICE_1_URL", "https://example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(cfg.Services))
	}
	if cfg.Services[0].Name != "Test" {
		t.Errorf("Name: got %q, want %q", cfg.Services[0].Name, "Test")
	}
	if cfg.Services[0].URL != "https://example.com" {
		t.Errorf("URL: got %q, want %q", cfg.Services[0].URL, "https://example.com")
	}
}

func TestLoad_MultipleServices(t *testing.T) {
	t.Setenv("SERVICE_1_NAME", "Alpha")
	t.Setenv("SERVICE_1_URL", "https://alpha.example.com")
	t.Setenv("SERVICE_2_NAME", "Beta")
	t.Setenv("SERVICE_2_URL", "https://beta.example.com")
	t.Setenv("SERVICE_3_NAME", "Gamma")
	t.Setenv("SERVICE_3_URL", "https://gamma.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 3 {
		t.Fatalf("expected 3 services, got %d", len(cfg.Services))
	}
}

func TestLoad_StopsAtGap(t *testing.T) {
	t.Setenv("SERVICE_1_NAME", "First")
	t.Setenv("SERVICE_1_URL", "https://first.example.com")
	// SERVICE_2 intentionally missing
	t.Setenv("SERVICE_3_NAME", "Third")
	t.Setenv("SERVICE_3_URL", "https://third.example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Services) != 1 {
		t.Errorf("expected 1 service (gap stops scan), got %d", len(cfg.Services))
	}
}

func TestLoad_InvalidPollInterval(t *testing.T) {
	t.Setenv("SERVICE_1_NAME", "Test")
	t.Setenv("SERVICE_1_URL", "https://example.com")
	t.Setenv("POLL_INTERVAL", "abc")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid POLL_INTERVAL")
	}
}

func TestLoad_InvalidHTTPTimeout(t *testing.T) {
	t.Setenv("SERVICE_1_NAME", "Test")
	t.Setenv("SERVICE_1_URL", "https://example.com")
	t.Setenv("HTTP_TIMEOUT", "abc")

	_, err := config.Load()
	if err == nil {
		t.Fatal("expected error for invalid HTTP_TIMEOUT")
	}
}

func TestLoad_Defaults(t *testing.T) {
	t.Setenv("SERVICE_1_NAME", "Test")
	t.Setenv("SERVICE_1_URL", "https://example.com")

	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.PollInterval != 60 {
		t.Errorf("PollInterval: got %d, want 60", cfg.PollInterval)
	}
	if cfg.HTTPTimeout != 10 {
		t.Errorf("HTTPTimeout: got %d, want 10", cfg.HTTPTimeout)
	}
	if cfg.DBPath != "./heimdall.db" {
		t.Errorf("DBPath: got %q, want %q", cfg.DBPath, "./heimdall.db")
	}
	if cfg.DataRetentionDays != 90 {
		t.Errorf("DataRetentionDays: got %d, want 90", cfg.DataRetentionDays)
	}
	if cfg.Port != "8080" {
		t.Errorf("Port: got %q, want %q", cfg.Port, "8080")
	}
}

func TestBasicAuthEnabled(t *testing.T) {
	both := &config.Config{HeimdallUser: "user", HeimdallPassword: "pass"}
	if !both.BasicAuthEnabled() {
		t.Error("expected BasicAuthEnabled()=true when both set")
	}

	onlyUser := &config.Config{HeimdallUser: "user"}
	if onlyUser.BasicAuthEnabled() {
		t.Error("expected BasicAuthEnabled()=false when only user set")
	}

	onlyPass := &config.Config{HeimdallPassword: "pass"}
	if onlyPass.BasicAuthEnabled() {
		t.Error("expected BasicAuthEnabled()=false when only password set")
	}

	neither := &config.Config{}
	if neither.BasicAuthEnabled() {
		t.Error("expected BasicAuthEnabled()=false when neither set")
	}
}
