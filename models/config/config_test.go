package config

import (
	"os"
	"path/filepath"
	"testing"
)

const sample = `
[database]
path = "/var/lib/inca/db"

[[account]]
name = "personal"
endpoint = "https://dav.example.com"
username = "alice"
password = "secret"

[[account]]
name = "work"
caldav_endpoint = "https://cal.work.example.com"
carddav_endpoint = "https://card.work.example.com"
username = "alice@work.example.com"
password = "hunter2"
`

func writeSample(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	if err := os.WriteFile(path, []byte(sample), 0o600); err != nil {
		t.Fatalf("writing sample config: %s", err)
	}
	return path
}

func TestLoadPlainPath(t *testing.T) {
	path := writeSample(t)

	cfg, err := New(path)
	if err != nil {
		t.Fatalf("New: %s", err)
	}

	if got := cfg.DatabasePath(); got != "/var/lib/inca/db" {
		t.Fatalf("DatabasePath = %q, want /var/lib/inca/db", got)
	}

	accounts, err := cfg.Accounts()
	if err != nil {
		t.Fatalf("Accounts: %s", err)
	}
	if len(accounts) != 2 {
		t.Fatalf("got %d accounts, want 2", len(accounts))
	}

	if accounts[0].Name != "personal" ||
		accounts[0].Endpoint != "https://dav.example.com" ||
		accounts[0].Username != "alice" {
		t.Fatalf("first account not parsed: %+v", accounts[0])
	}

	if accounts[1].CalDAVEndpoint != "https://cal.work.example.com" ||
		accounts[1].CardDAVEndpoint != "https://card.work.example.com" {
		t.Fatalf("second account endpoints not parsed: %+v", accounts[1])
	}
}

func TestLoadFileURL(t *testing.T) {
	path := writeSample(t)

	cfg, err := New("file://" + path)
	if err != nil {
		t.Fatalf("New with file URL: %s", err)
	}
	if got := cfg.DatabasePath(); got != "/var/lib/inca/db" {
		t.Fatalf("DatabasePath = %q, want /var/lib/inca/db", got)
	}
}

func TestUnsupportedScheme(t *testing.T) {
	if _, err := New("http://example.com/config.toml"); err == nil {
		t.Fatal("expected an error for an unsupported scheme")
	}
}
