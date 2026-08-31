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
	if found != 4 {
		t.Fatalf("checked %d documented ftn.cfg examples, want 4", found)
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

// A file-box board that only RECEIVES has no netmail directory to name, and
// -in never writes netmail. Refusing to load its config told it to fix the one
// setting its runs never touch (three-board rig, 2026-08-27).
func TestConfigLoadsWithoutNetmailDirForAReceiveOnlyBoard(t *testing.T) {
	dir := t.TempDir()
	inbound := filepath.Join(dir, "in")
	if err := os.MkdirAll(inbound, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte("InboundDir "+inbound+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("a receive-only config was refused: %v", err)
	}
	// But the board that actually has an attach handoff to make is still told,
	// at the point where it matters.
	if err := RequireNetmail(cfg, dir); err == nil {
		t.Error("an attach handoff with no NetmailDir was accepted")
	} else if !strings.Contains(err.Error(), ConfigFile) {
		t.Errorf("the refusal does not name the file to fix: %v", err)
	}
}

// Raw composes with every handoff mode, because the envelope is what an old
// peer cannot parse, not the way the file travels. It is read off the end of
// the line so BSO's optional flavour keeps its own position — and it is the
// default, so a link with no keyword is raw and `Bundled` is the opt-out.
func TestRawComposesWithEveryLinkMode(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"in", "netmail", "obox", "bso"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	body := "InboundDir " + filepath.Join(dir, "in") + "\n" +
		"NetmailDir " + filepath.Join(dir, "netmail") + "\n" +
		"Link 1 Attach Raw\n" +
		"Link 2 Obox obox raw\n" +
		"Link 3 BSO bso Crash Raw\n" +
		"Link 4 Obox obox\n" +
		"Link 5 Obox obox Bundled\n"
	if err := os.WriteFile(filepath.Join(dir, ConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	for node, want := range map[int]struct {
		mode LinkMode
		raw  bool
		flav string
	}{
		1: {LinkAttach, true, "Normal"},
		2: {LinkObox, true, "Normal"},
		3: {LinkBSO, true, "Continuous"}, // "Crash" is the BSO spelling; normalFlavour folds it
		4: {LinkObox, false, "Normal"},   // states nothing; the posture answers for it
		5: {LinkObox, false, "Normal"},   // Bundled is how a sysop opts out
	} {
		got := cfg.Links[node]
		if got.Mode != want.mode || got.Raw != want.raw || got.Flavour != want.flav {
			t.Errorf("node %d = %+v, want mode %v raw %v flavour %q", node, got, want.mode, want.raw, want.flav)
		}
	}

	// Link.Raw is what the LINE said; rawFor is what applies. The posture
	// answers for a link that stated nothing and never overrides one that did,
	// and raw is the default, so a board configuring nothing keeps sending what
	// every board can already read (#230).
	if !rawFor(cfg, cfg.Links[4]) {
		t.Error("a link stating nothing did not take the default raw posture")
	}
	bundled := cfg
	bundled.Bundled = true
	if rawFor(bundled, bundled.Links[4]) {
		t.Error("a link stating nothing ignored the bundled posture")
	}
	if !rawFor(bundled, bundled.Links[1]) {
		t.Error("an explicit Raw link was overridden by the bundled posture")
	}
	if rawFor(cfg, cfg.Links[5]) {
		t.Error("an explicit Bundled link was overridden by the raw posture")
	}
}
