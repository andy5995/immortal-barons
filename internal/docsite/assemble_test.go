package docsite

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFixture builds a minimal repo tree under root for the assembler to read.
func writeFixture(t *testing.T, root string) {
	t.Helper()
	files := map[string]string{
		"README.md":                                   "# Immortal Barons\n\nOverview. See [door setup](docs/door-setup.md).\n",
		"docs/playing.md":                             "# Playing\n\nHow to play.\n",
		"docs/door-setup.md":                          "# Door Setup\n\nSetup. See [leagues](docs/inter-bbs.md).\n",
		"docs/inter-bbs.md":                           "# Inter-BBS Leagues\n\nLeague play.\n",
		"docs/ftn-transport.md":                       "# FTN Transport\n\nTransport setup.\n",
		"docs/command-reference.md":                   "# Command Reference\n\nAll options.\n",
		"docs/download.md":                            "# Download\n\nReleases and snapshots.\n",
		"docs/faq.md":                                 "# FAQ\n\nQuestions.\n",
		"docs/translating.md":                         "# Translating\n\nHow to translate.\n",
		"docs/charset.md":                             "# Character Set\n\nCP437 and UTF-8.\n",
		"docs/manual-vs-code.md":                      "# Manual vs. Code\n\nWhere the original's docs and its code disagree.\n",
		"docs/dev/packets.md":                         "# Packet Format\n\nDev reference.\n",
		"internal/help/content/economy/regions.md":    "---\ntitle: Regions\ncategory: economy\norder: 1\nin_game: true\n---\n# Regions\n\nBuy land.\n",
		"internal/help/content/controls/interface.md": "---\ntitle: Menus\ncategory: controls\norder: 1\nin_game: true\n---\n# Menus\n\nNavigate.\n",
		// German: only the one topic is translated.
		"internal/help/content.de/economy/regions.md": "---\ntitle: Regionen\ncategory: economy\norder: 1\nin_game: true\n---\n# Regionen\n\nLand kaufen.\n",
	}
	for rel, body := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// content.ru dir with no files, so loadTopics finds an empty set (not an error).
	if err := os.MkdirAll(filepath.Join(root, "internal/help/content.ru"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestAssembleLayout(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	writeFixture(t, root)

	if err := Assemble(root, out, ""); err != nil {
		t.Fatal(err)
	}

	must := []string{
		"site-src/en/index.md",                    // README -> Home
		"site-src/en/guide/index.md",              // playing -> Guide intro
		"site-src/en/guide/economy/regions.md",    // help topic
		"site-src/en/guide/controls/interface.md", // help topic
		"site-src/en/door-setup/index.md",         // door setup (was sysop guide)
		"site-src/en/inter-bbs/index.md",          // inter-BBS leagues, split out of door setup
		"site-src/en/ftn-transport/index.md",      // detailed FTN transport guide
		"site-src/en/command-reference/index.md",  // command reference (#34)
		"site-src/en/download/index.md",           // download page
		"site-src/en/faq/index.md",                // faq
		"site-src/en/translating/index.md",        // translating guide
		"site-src/en/developers/packets.md",       // dev doc (en only)
		"site-src/de/guide/economy/regions.md",    // translated topic
		"mkdocs.yml",
	}
	for _, rel := range must {
		if _, err := os.Stat(filepath.Join(out, rel)); err != nil {
			t.Errorf("expected %s: %v", rel, err)
		}
	}

	// German has no translated README/playing/sysop, so those files must be
	// absent (the i18n plugin falls back to English at build time).
	for _, rel := range []string{"site-src/de/index.md", "site-src/de/door-setup/index.md", "site-src/de/inter-bbs/index.md", "site-src/de/command-reference/index.md", "site-src/de/faq/index.md", "site-src/de/translating/index.md", "site-src/de/developers/packets.md"} {
		if _, err := os.Stat(filepath.Join(out, rel)); err == nil {
			t.Errorf("did not expect %s (should fall back to English)", rel)
		}
	}
}

func TestAssembleNavAndConfig(t *testing.T) {
	root := t.TempDir()
	out := t.TempDir()
	writeFixture(t, root)
	if err := Assemble(root, out, ""); err != nil {
		t.Fatal(err)
	}
	yml, err := os.ReadFile(filepath.Join(out, "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(yml)

	for _, want := range []string{
		"theme:\n  name: material",
		"docs_structure: folder",
		"locale: en",
		"locale: de",
		"locale: ru",
		`- "Home": index.md`,
		`- "Regions": guide/economy/regions.md`,
		`- "Game Instructions":`, // the one collapsible nav section; matches the in-game label
		`- "Door Setup": door-setup/index.md`,
		`- "FTN Transport": ftn-transport/index.md`,
		`- "Command Reference": command-reference/index.md`,
		`- "Download": download/index.md`,
		`- "FAQ": faq/index.md`,
		`- "Translating": translating/index.md`,
		`- "Packet Format": developers/packets.md`, // dev nav titled from its H1
	} {
		if !strings.Contains(s, want) {
			t.Errorf("mkdocs.yml missing %q\n---\n%s", want, s)
		}
	}

	// Controls (order 1 in CategoryOrder) must appear before Economy in the nav.
	if strings.Index(s, `"Controls"`) > strings.Index(s, `"Economy"`) {
		t.Error("categories out of order: Controls should precede Economy")
	}
}
