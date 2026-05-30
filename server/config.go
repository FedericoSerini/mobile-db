package server

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AdminPasswordHash   string
	DBEncryptionKey     []byte
	KeycloakURL         string
	KeycloakRealm       string
	KeycloakClientID    string
	BackupDestination   string
	BackupInterval      string
	LogLevel            string
	CompactionThreshold int
	AdminAllowedIPs     []string
	DataDir             string
	ListenAddr          string
	CRSQLiteExtPath     string
}

func LoadConfig() (*Config, error) {
	var missing []string
	require := func(key string) string {
		v := os.Getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	opt := func(key, def string) string {
		if v := os.Getenv(key); v != "" {
			return v
		}
		return def
	}

	cfg := &Config{
		AdminPasswordHash: require("ADMIN_PASSWORD_HASH"),
		DBEncryptionKey:   []byte(require("DB_ENCRYPTION_KEY")),
		KeycloakURL:       require("KEYCLOAK_URL"),
		KeycloakRealm:     require("KEYCLOAK_REALM"),
		KeycloakClientID:  opt("KEYCLOAK_CLIENT_ID", ""),
		BackupDestination: opt("BACKUP_DESTINATION", ""),
		BackupInterval:    opt("BACKUP_INTERVAL", "24h"),
		LogLevel:          opt("LOG_LEVEL", "info"),
		DataDir:           opt("DATA_DIR", "./data"),
		ListenAddr:        opt("LISTEN_ADDR", ":8443"),
		CRSQLiteExtPath:   opt("CRSQLITE_EXT_PATH", "./crsqlite.so"),
	}

	if raw := os.Getenv("COMPACTION_THRESHOLD"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil {
			return nil, errors.New("COMPACTION_THRESHOLD must be an integer")
		}
		if n < 0 {
			return nil, errors.New("COMPACTION_THRESHOLD must be non-negative")
		}
		cfg.CompactionThreshold = n
	} else {
		cfg.CompactionThreshold = 1000
	}

	if raw := os.Getenv("ADMIN_ALLOWED_IPS"); raw != "" {
		parts := strings.Split(raw, ",")
		for i, p := range parts {
			parts[i] = strings.TrimSpace(p)
		}
		cfg.AdminAllowedIPs = parts
	}

	if len(missing) > 0 {
		return nil, errors.New("missing required env vars: " + strings.Join(missing, ", "))
	}

	if len(cfg.DBEncryptionKey) < 32 {
		return nil, errors.New("DB_ENCRYPTION_KEY must be at least 32 bytes")
	}

	return cfg, nil
}
