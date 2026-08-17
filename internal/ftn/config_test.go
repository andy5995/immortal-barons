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
	if cfg.AttachDir != "" || cfg.SubjectMode != SubjectAbsolute {
		t.Errorf("an ftn.cfg without the new keys changed behaviour: AttachDir = %q, SubjectMode = %v",
			cfg.AttachDir, cfg.SubjectMode)
	}
	if got := cfg.attachPath(filepath.Join(data, "o"), "p.brp"); got != filepath.Join(data, "o", "fido", "p.brp") {
		t.Errorf("attachPath = %q, want the outbound fido child", got)
	}
}

func TestLoadConfigAttachmentSettings(t *testing.T) {
	data := t.TempDir()
	if err := os.Mkdir(filepath.Join(data, "netmail"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "NetmailDir netmail\nAttachDir attach\nSubjectPath Basename\n"
	if err := os.WriteFile(filepath.Join(data, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(data, "attach"); cfg.AttachDir != want {
		t.Errorf("AttachDir = %q, want %q resolved against the data directory", cfg.AttachDir, want)
	}
	if cfg.SubjectMode != SubjectBasename {
		t.Errorf("SubjectMode = %v, want SubjectBasename", cfg.SubjectMode)
	}
	if got := cfg.attachPath(filepath.Join(data, "o"), "p.brp"); got != filepath.Join(data, "attach", "p.brp") {
		t.Errorf("attachPath = %q, want the configured AttachDir", got)
	}

	body = "NetmailDir netmail\nSubjectPath ../fileboxes/ib\n"
	if err := os.WriteFile(filepath.Join(data, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.SubjectMode != SubjectPrefixed || cfg.SubjectPrefix != "../fileboxes/ib" {
		t.Errorf("SubjectPath = %v %q, want a prefix", cfg.SubjectMode, cfg.SubjectPrefix)
	}
}
