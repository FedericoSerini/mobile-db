package server_test

import (
	"os"
	"strings"
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

func TestJWTSecretTooShortFails(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD_HASH", "$2a$12$test")
	t.Setenv("JWT_SECRET", "tooshort")
	t.Setenv("DB_ENCRYPTION_KEY", "aaaabbbbccccddddeeeeffffgggghhhh")

	_, err := server.LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig must error when JWT_SECRET is shorter than 32 bytes")
	}
}

func TestNegativeCompactionThresholdFails(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD_HASH", "$2a$12$test")
	t.Setenv("JWT_SECRET", "aaaabbbbccccddddeeeeffffgggghhhhiiii")
	t.Setenv("DB_ENCRYPTION_KEY", "aaaabbbbccccddddeeeeffffgggghhhh")
	t.Setenv("COMPACTION_THRESHOLD", "-1")

	_, err := server.LoadConfig()
	if err == nil {
		t.Fatal("LoadConfig must error when COMPACTION_THRESHOLD is negative")
	}
}

func TestAdminAllowedIPsTrimmed(t *testing.T) {
	t.Setenv("ADMIN_PASSWORD_HASH", "$2a$12$test")
	t.Setenv("JWT_SECRET", "aaaabbbbccccddddeeeeffffgggghhhhiiii")
	t.Setenv("DB_ENCRYPTION_KEY", "aaaabbbbccccddddeeeeffffgggghhhh")
	t.Setenv("ADMIN_ALLOWED_IPS", "192.168.1.1,  192.168.1.2 , 10.0.0.1")

	cfg, err := server.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	for _, ip := range cfg.AdminAllowedIPs {
		if strings.TrimSpace(ip) != ip {
			t.Fatalf("AdminAllowedIPs entry has surrounding whitespace: %q", ip)
		}
	}
	if len(cfg.AdminAllowedIPs) != 3 {
		t.Fatalf("AdminAllowedIPs: want 3 entries, got %d", len(cfg.AdminAllowedIPs))
	}
}
