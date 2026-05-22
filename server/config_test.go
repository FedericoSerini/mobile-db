package server_test

import (
	"os"
	"testing"

	"github.com/federicoserini/mobile-db/server"
)

func TestConfigLoadsDefaults(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD_HASH", "$2a$12$test")
	t.Setenv("JWT_SECRET", "aaaabbbbccccddddeeeeffffgggghhhhiiii")
	t.Setenv("DB_ENCRYPTION_KEY", "aaaabbbbccccddddeeeeffffgggghhhh")

	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if cfg.CompactionThreshold != 1000 {
		t.Fatalf("default CompactionThreshold: want 1000, got %d", cfg.CompactionThreshold)
	}
	if cfg.ListenAddr != ":8443" {
		t.Fatalf("default ListenAddr: want :8443, got %s", cfg.ListenAddr)
	}
}

func TestConfigMissingRequiredFails(t *testing.T) {
	os.Unsetenv("ADMIN_PASSWORD_HASH")
	os.Unsetenv("JWT_SECRET")
	os.Unsetenv("DB_ENCRYPTION_KEY")

	_, err := server.LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig must error when required env vars are absent")
	}
}
