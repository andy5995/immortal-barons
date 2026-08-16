package ftn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	data := t.TempDir()
	netmail := filepath.Join(data, "netmail")
	if err := os.Mkdir(netmail, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# FTN transport only\nUnknown future-value\nNetmailDir netmail\nBinkley Yes\n"
	if err := os.WriteFile(filepath.Join(data, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.NetmailDir != netmail {
		t.Errorf("NetmailDir = %q, want %q", cfg.NetmailDir, netmail)
	}
	if !cfg.Binkley {
		t.Error("Binkley = false, want true")
	}
}
