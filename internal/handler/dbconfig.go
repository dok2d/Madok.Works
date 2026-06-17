package handler

import (
	"encoding/json"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"kanban/internal/db"
)

// First-run DB selection: when the operator picks an external PostgreSQL on the
// setup page, the chosen DSN is persisted here so subsequent starts use it.
const dbConfigFile = "db.json"

type dbConfig struct {
	DSN string `json:"dsn"`
}

func dbConfigPath(dataDir string) string { return filepath.Join(dataDir, dbConfigFile) }

// readDBConfig returns a DSN persisted by a previous first-run setup, if any.
func readDBConfig(dataDir string) (string, bool) {
	raw, err := os.ReadFile(dbConfigPath(dataDir))
	if err != nil {
		return "", false
	}
	var c dbConfig
	if json.Unmarshal(raw, &c) != nil || strings.TrimSpace(c.DSN) == "" {
		return "", false
	}
	return c.DSN, true
}

// writeDBConfig persists the chosen DSN (0600 — it contains credentials).
func writeDBConfig(dataDir, dsn string) error {
	raw, _ := json.Marshal(dbConfig{DSN: dsn})
	return os.WriteFile(dbConfigPath(dataDir), raw, 0600)
}

// ResolveDSN picks the effective database target at startup:
//  1. an explicit postgres:// / postgresql:// -db flag always wins;
//  2. otherwise a DSN persisted by first-run setup (<data-dir>/db.json);
//  3. otherwise the -db value, treated as a SQLite file path.
func ResolveDSN(dbFlag, dataDir string) string {
	if db.IsPostgresURL(dbFlag) {
		return dbFlag
	}
	if dsn, ok := readDBConfig(dataDir); ok {
		return dsn
	}
	return dbFlag
}

// buildPostgresDSN assembles a DSN from the discrete fields entered on the setup
// page, URL-encoding credentials. sslmode=disable suits a trusted LAN/private
// network; operators needing TLS can pass a full -db URL instead.
func buildPostgresDSN(host, port, name, user, pass string) string {
	if strings.TrimSpace(port) == "" {
		port = "5432"
	}
	u := url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, pass),
		Host:     host + ":" + port,
		Path:     "/" + name,
		RawQuery: "sslmode=disable",
	}
	return u.String()
}
