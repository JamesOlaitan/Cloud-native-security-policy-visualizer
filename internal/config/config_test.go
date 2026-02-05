package config

import (
	"testing"
)

func TestLoad(t *testing.T) {
	// Test defaults
	cfg := Load()

	if !cfg.Offline {
		t.Error("Expected Offline to default to true")
	}

	if cfg.OPAUrl == "" {
		t.Error("Expected OPAUrl to have a default")
	}

	if cfg.SQLitePath == "" {
		t.Error("Expected SQLitePath to have a default")
	}

	if cfg.Port == "" {
		t.Error("Expected Port to have a default")
	}
}

func TestLoadWithEnv(t *testing.T) {
	// Set environment variables (t.Setenv auto-cleans up)
	t.Setenv("OFFLINE", "false")
	t.Setenv("OPA_URL", "http://custom:9999/opa")
	t.Setenv("SQLITE_PATH", "/custom/path.db")
	t.Setenv("PORT", "9090")

	cfg := Load()

	if cfg.Offline {
		t.Error("Expected Offline to be false from env")
	}

	if cfg.OPAUrl != "http://custom:9999/opa" {
		t.Errorf("Expected custom OPA URL, got %s", cfg.OPAUrl)
	}

	if cfg.SQLitePath != "/custom/path.db" {
		t.Errorf("Expected custom SQLite path, got %s", cfg.SQLitePath)
	}

	if cfg.Port != "9090" {
		t.Errorf("Expected custom port, got %s", cfg.Port)
	}
}
