package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/game"
)

// A packet whose board name holds a control character is deleted before
// anything prints or logs the name, and the log line that records it carries
// no raw control character.
func TestReadInboundDeletesControlInBoardName(t *testing.T) {
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.BoardID = "Home"
	w := game.NewWorldSeed(cfg, 1)
	inbound := t.TempDir()
	data, err := json.Marshal(game.Packet{FromBoard: "Evil\x1b[2JBBS", Seq: 1, Date: "2026-10-01",
		Scores: []game.RemoteScore{{Empire: "Spoof"}}})
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(inbound, "in.brp")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	result, err := readInboundAndRemove(w, inbound, false)
	if err != nil {
		t.Fatal(err)
	}
	if result.Refused != 1 || result.Applied != 0 || len(w.RemoteBoards) != 0 {
		t.Errorf("refused=%d applied=%d boards=%d", result.Refused, result.Applied, len(w.RemoteBoards))
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the packet was left in inbound")
	}
	if len(result.Log) != 1 || !strings.Contains(result.Log[0], "control character") {
		t.Fatalf("log = %q", result.Log)
	}
	for _, n := range w.SysopNotices {
		if strings.ContainsRune(n, 0x1b) {
			t.Errorf("a sysop notice carries the raw name: %q", n)
		}
	}
}
