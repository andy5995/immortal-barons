package ftn

import (
	"os"
	"path/filepath"
	"strings"
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

func TestDocumentedStandaloneConfigsParse(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "ftn-transport.md"))
	if err != nil {
		t.Fatal(err)
	}
	const marker = "<!-- test-ftn-config -->\n```ini\n"
	rest := string(doc)
	found := 0
	for {
		_, after, ok := strings.Cut(rest, marker)
		if !ok {
			break
		}
		body, tail, ok := strings.Cut(after, "\n```")
		if !ok {
			t.Fatal("documented ftn.cfg example has no closing fence")
		}
		data := t.TempDir()
		if err := os.WriteFile(filepath.Join(data, ConfigFile), []byte(body+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadConfig(data); err != nil {
			t.Errorf("documented ftn.cfg example %d: %v\n%s", found+1, err, body)
		}
		found++
		rest = tail
	}
	if found != 3 {
		t.Fatalf("checked %d documented ftn.cfg examples, want 3", found)
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

func TestLoadConfigMixedLinks(t *testing.T) {
	data := t.TempDir()
	for _, dir := range []string{"netmail", "in", "obox", "bso"} {
		if err := os.Mkdir(filepath.Join(data, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	body := "NetmailDir netmail\nInboundDir in\nOboxMeshFanout No\n" +
		"Link 2 Attach\nLink 3 Obox obox\nLink 4 BSO bso Crash\n"
	if err := os.WriteFile(filepath.Join(data, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OboxMeshFanout || cfg.InboundDir != filepath.Join(data, "in") {
		t.Fatalf("inbound/fanout = %q/%v", cfg.InboundDir, cfg.OboxMeshFanout)
	}
	if cfg.Links[2].Mode != LinkAttach || cfg.Links[3].Mode != LinkObox || cfg.Links[4].Mode != LinkBSO || cfg.Links[4].Flavour != "Continuous" {
		t.Fatalf("links = %#v", cfg.Links)
	}
}
