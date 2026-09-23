package ftn

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/store"
)

func TestLoadConfig(t *testing.T) {
	data := t.TempDir()
	netmail := filepath.Join(data, "netmail")
	if err := os.Mkdir(netmail, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "# FTN transport only\nUnknown future-value\nOutgoingNetmailDir netmail\nMailer Binkley\n"
	if err := os.WriteFile(filepath.Join(data, store.BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OutgoingNetmailDir != netmail {
		t.Errorf("OutgoingNetmailDir = %q, want %q", cfg.OutgoingNetmailDir, netmail)
	}
	if !cfg.Binkley {
		t.Error("Binkley = false, want true")
	}
	if cfg.AttachDir != "" || cfg.SubjectMode != SubjectAbsolute {
		t.Errorf("a bbs.cfg without the attach keys changed behavior: AttachDir = %q, SubjectMode = %v",
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
			t.Fatal("documented transport example has no closing fence")
		}
		data := t.TempDir()
		if err := os.WriteFile(filepath.Join(data, store.BoardConfigFile), []byte(body+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(data)
		if err != nil {
			t.Errorf("documented example %d: %v\n%s", found+1, err, body)
		} else if !cfg.Receives() || !cfg.Sends() {
			// Every example names both halves; one that sets up neither is
			// written in keys this version no longer reads.
			t.Errorf("documented example %d configures no transport:\n%s", found+1, body)
		}
		found++
		rest = tail
	}
	if found != 4 {
		t.Fatalf("checked %d documented transport examples, want 4", found)
	}
}

func TestLoadConfigAttachmentSettings(t *testing.T) {
	data := t.TempDir()
	if err := os.Mkdir(filepath.Join(data, "netmail"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "OutgoingNetmailDir netmail\nAttachDir attach\nSubjectPath Basename\n"
	if err := os.WriteFile(filepath.Join(data, store.BoardConfigFile), []byte(body), 0o644); err != nil {
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

	body = "OutgoingNetmailDir netmail\nSubjectPath ../fileboxes/ib\n"
	if err := os.WriteFile(filepath.Join(data, store.BoardConfigFile), []byte(body), 0o644); err != nil {
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
	body := "OutgoingNetmailDir netmail\nIncomingFileDir in\nOboxMeshFanout No\n" +
		"Link 2 Attach\nLink 3 Obox obox\nLink 4 BSO bso Crash\n"
	if err := os.WriteFile(filepath.Join(data, store.BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.OboxMeshFanout || cfg.IncomingFileDir != filepath.Join(data, "in") {
		t.Fatalf("inbound/fanout = %q/%v", cfg.IncomingFileDir, cfg.OboxMeshFanout)
	}
	if cfg.Links[2].Mode != LinkAttach || cfg.Links[3].Mode != LinkObox || cfg.Links[4].Mode != LinkBSO || cfg.Links[4].Flavor != "Continuous" {
		t.Fatalf("links = %#v", cfg.Links)
	}
}

// A file-box board that only RECEIVES has no netmail directory to name, and
// the unwrap step never writes netmail. Refusing to load its config told it to
// fix the one setting its runs never touch (three-board rig, 2026-08-27).
func TestConfigLoadsWithoutOutgoingNetmailDirForAReceiveOnlyBoard(t *testing.T) {
	dir := t.TempDir()
	inbound := filepath.Join(dir, "in")
	if err := os.MkdirAll(inbound, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte("IncomingFileDir "+inbound+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("a receive-only config was refused: %v", err)
	}
	// But the board that actually has an attach handoff to make is still told,
	// at the point where it matters.
	if err := RequireNetmail(cfg, dir); err == nil {
		t.Error("an attach handoff with no OutgoingNetmailDir was accepted")
	} else if !strings.Contains(err.Error(), store.BoardConfigFile) {
		t.Errorf("the refusal does not name the file to fix: %v", err)
	}
}

// Raw composes with every handoff mode, because the envelope is what an old
// peer cannot parse, not the way the file travels. It is read off the end of
// the line so BSO's optional flavor keeps its own position — and it is the
// default, so a link with no keyword is raw and `Bundled` is the opt-out.
func TestRawComposesWithEveryLinkMode(t *testing.T) {
	dir := t.TempDir()
	for _, sub := range []string{"in", "netmail", "obox", "bso"} {
		if err := os.MkdirAll(filepath.Join(dir, sub), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	body := "IncomingFileDir " + filepath.Join(dir, "in") + "\n" +
		"OutgoingNetmailDir " + filepath.Join(dir, "netmail") + "\n" +
		"Link 1 Attach Raw\n" +
		"Link 2 Obox obox raw\n" +
		"Link 3 BSO bso Crash Raw\n" +
		"Link 4 Obox obox\n" +
		"Link 5 Obox obox Bundled\n"
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(body), 0o644); err != nil {
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
		if got.Mode != want.mode || got.Raw != want.raw || got.Flavor != want.flav {
			t.Errorf("node %d = %+v, want mode %v raw %v flavor %q", node, got, want.mode, want.raw, want.flav)
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

// Mailer takes the original's BBS.CFG line-7 list in any case. Binkley is the
// one that changes the envelope; None writes no netmail; anything else is
// refused, because a misspelled None would go on writing netmail.
func TestMailerTakesTheOriginalsList(t *testing.T) {
	for _, tc := range []struct {
		value           string
		binkley, silent bool
		refused         bool
	}{
		{"FRONTDOOR", false, false, false},
		{"binkley", true, false, false},
		{"DBridge", false, false, false},
		{"InterMail", false, false, false},
		{"dbridgeold", false, false, false},
		{"Other", false, false, false},
		{"NONE", false, true, false},
		{"Nnoe", false, false, true},
		{"Yes", false, false, true},
	} {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "netmail"), 0o755); err != nil {
			t.Fatal(err)
		}
		body := "OutgoingNetmailDir netmail\nMailer " + tc.value + "\n"
		if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := LoadConfig(dir)
		if tc.refused {
			if err == nil || !strings.Contains(err.Error(), "Mailer") {
				t.Errorf("Mailer %s: got %v, want a refusal naming the setting", tc.value, err)
			}
			continue
		}
		if err != nil {
			t.Errorf("Mailer %s: %v", tc.value, err)
			continue
		}
		if cfg.Binkley != tc.binkley || cfg.NoNetmail != tc.silent {
			t.Errorf("Mailer %s: Binkley %v NoNetmail %v", tc.value, cfg.Binkley, cfg.NoNetmail)
		}
		// None with nothing but a netmail directory has nothing to hand off.
		if cfg.Sends() == tc.silent {
			t.Errorf("Mailer %s: Sends() = %v", tc.value, cfg.Sends())
		}
	}
}

// Mailer None and an attach link contradict each other, so the pair is refused
// at load rather than met at the first handoff.
func TestMailerNoneRefusesAnAttachLink(t *testing.T) {
	dir := t.TempDir()
	body := "Mailer None\nLink 2 Attach\n"
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfig(dir); err == nil || !strings.Contains(err.Error(), "Link 2 Attach") {
		t.Errorf("Mailer None with an attach link: %v", err)
	}
}

// A board with no transport lines configures nothing, and a missing bbs.cfg
// is not an error: the game's own modes ask on every run.
func TestNoTransportLinesMeansNoTransport(t *testing.T) {
	dir := t.TempDir()
	cfg, err := LoadConfig(dir)
	if err != nil || cfg.Receives() || cfg.Sends() {
		t.Fatalf("no bbs.cfg: %+v, %v", cfg, err)
	}
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte("BoardID A\nGameInbound in\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err = LoadConfig(dir)
	if err != nil || cfg.Receives() || cfg.Sends() {
		t.Fatalf("game lines only: %+v, %v", cfg, err)
	}
}

// The transport's lines are read the same way as the board's: a tab or a run of
// spaces separates key from value.
func TestConfigAcceptsTabsAndRunsOfSpaces(t *testing.T) {
	dir := t.TempDir()
	body := "IncomingFileDir\tmailer in\nOutgoingNetmailDir     netmail\nMailer\t\tBinkley\n"
	if err := os.WriteFile(filepath.Join(dir, store.BoardConfigFile), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "mailer in"); cfg.IncomingFileDir != want {
		t.Errorf("IncomingFileDir = %q, want %q", cfg.IncomingFileDir, want)
	}
	if want := filepath.Join(dir, "netmail"); cfg.OutgoingNetmailDir != want {
		t.Errorf("OutgoingNetmailDir = %q, want %q", cfg.OutgoingNetmailDir, want)
	}
	if !cfg.Binkley {
		t.Error("Mailer<TAB><TAB>Binkley was not read")
	}
}
