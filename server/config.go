package server

import (
	"errors"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	AdminPasswordHash    string
	JWTSecret           []byte
	DBEncryptionKey     []byte
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
		JWTSecret:         []byte(require("JWT_SECRET")),
		DBEncryptionKey:   []byte(require("DB_ENCRYPTION_KEY")),
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
		cfg.CompactionThreshold = n
	} else {
		cfg.CompactionThreshold = 1000
	}

	if raw := os.Getenv("ADMIN_ALLOWED_IPS"); raw != "" {
		cfg.AdminAllowedIPs = strings.Split(raw, ",")
	}

	if len(missing) > 0 {
		return nil, errors.New("missing required env vars: " + strings.Join(missing, ", "))
	}
	return cfg, nil
}
