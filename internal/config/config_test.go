package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "DATABASE_URL=postgres://test:test@localhost/test")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.AppEnv != "development" {
		t.Fatalf("AppEnv = %q, want %q", cfg.AppEnv, "development")
	}
	if cfg.Port != "8080" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "8080")
	}
	if !cfg.AutoMigrate {
		t.Fatal("AutoMigrate = false, want true")
	}
	if cfg.DatabaseURL != "postgres://test:test@localhost/test" {
		t.Fatalf("DatabaseURL = %q, want postgres://test:test@localhost/test", cfg.DatabaseURL)
	}
}

func TestLoad_EnvOverride(t *testing.T) {
	dir := t.TempDir()
	writeEnv(t, dir, "DATABASE_URL=postgres://test:test@localhost/test\nPORT=9090\nAPP_ENV=production\nAUTO_MIGRATE=false")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.Port != "9090" {
		t.Fatalf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.AppEnv != "production" {
		t.Fatalf("AppEnv = %q, want %q", cfg.AppEnv, "production")
	}
	if cfg.AutoMigrate {
		t.Fatal("AutoMigrate = true, want false")
	}
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	// Write an env file without DATABASE_URL.
	writeEnv(t, dir, "PORT=8080")

	// Clear environment variable in case the test runner has it set.
	t.Setenv("DATABASE_URL", "")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}

func TestLoad_NoConfigFile(t *testing.T) {
	dir := t.TempDir()
	// No .env file at all — should still use defaults and fail on missing DATABASE_URL.
	t.Setenv("DATABASE_URL", "")

	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}

func TestLoad_NoConfigFile_WithEnvVar(t *testing.T) {
	dir := t.TempDir()
	// No .env file, but DATABASE_URL is set via environment.
	t.Setenv("DATABASE_URL", "postgres://env:env@localhost/envdb")

	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.DatabaseURL != "postgres://env:env@localhost/envdb" {
		t.Fatalf("DatabaseURL = %q, want postgres://env:env@localhost/envdb", cfg.DatabaseURL)
	}
}

func writeEnv(t *testing.T, dir, content string) {
	t.Helper()
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write .env: %v", err)
	}
}
