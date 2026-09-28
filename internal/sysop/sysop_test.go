package sysop

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/store"
)

// A child run of this test binary prints its arguments and exits, so Start is
// tested against a real process without the game binary being built.
func TestMain(m *testing.M) {
	if os.Getenv("IB_SYSOP_HELPER") == "1" {
		fmt.Println("args:", strings.Join(os.Args[1:], " "))
		fmt.Fprintln(os.Stderr, "to stderr")
		os.Exit(3)
	}
	os.Exit(m.Run())
}

func leagueBoard(t *testing.T) game.Config {
	t.Helper()
	cfg := game.DefaultConfig()
	cfg.DataDir = t.TempDir()
	cfg.IBBS = true
	cfg.BoardID = "Home BBS"
	cfg.LeagueNumber = 900
	cfg.LostForcesDays = 3
	w := store.NewGame(cfg)
	w.LeagueNodes = []game.LeagueNode{
		{Number: 1, Name: "Home BBS", Address: "1:1/1", City: "A", State: "B", Country: "C"},
		{Number: 4, Name: "Far BBS", Address: "1:1/4", City: "A", State: "B", Country: "C"},
	}
	w.GameDay = 10
	e := w.AddHuman("alice", "Alethia")
	w.InFlight = []game.InFlightStrike{{ID: 7, Kind: "terror", TargetBoard: "Far BBS",
		TargetEmpire: "Rome", Owner: e.Owner, Agents: 5, LaunchedDay: 9, TerrorOp: game.TerrorOpSpy}}
	w.LastPacketFrom = map[string]string{"Far BBS": game.StoredStamp(time.Now().Add(-49 * time.Hour))}
	w.BoardVersion = map[string]string{"Far BBS": "0.2.0"}
	w.TravelTimes = map[string]float64{"Far BBS": 1.5}
	if err := store.Save(w, cfg); err != nil {
		t.Fatal(err)
	}
	if err := store.WriteNodeList(filepath.Join(cfg.DataDir, store.NodeListFile), w.LeagueNodes); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, store.BoardConfigFile), []byte(store.BoardConfigText(cfg)), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestReadGathersTheBoard(t *testing.T) {
	cfg := leagueBoard(t)
	before, err := os.ReadFile(filepath.Join(cfg.DataDir, "world.json"))
	if err != nil {
		t.Fatal(err)
	}
	s, err := Read(cfg.DataDir, time.Now())
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if !s.League || !s.Coordinator || s.BoardID != "Home BBS" || s.GameDay != 10 {
		t.Errorf("snapshot header wrong: %+v", s)
	}
	if len(s.Boards) != 1 {
		t.Fatalf("boards = %+v, want Far BBS alone", s.Boards)
	}
	b := s.Boards[0]
	if b.Number != 4 || b.Name != "Far BBS" || b.Version != "0.2.0" || b.SilentDays != 2 || b.LastHeard.IsZero() || b.RoundTrip != 1.5 || b.Status() != "ok" {
		t.Errorf("board row wrong: %+v (status %q)", b, b.Status())
	}
	if len(s.InFlight) != 1 {
		t.Fatalf("in flight = %+v", s.InFlight)
	}
	f := s.InFlight[0]
	if f.Owner != "Alethia" || f.What != "Send Spy (5 agents)" || f.Waiting != 1 || !f.Recovers || f.DaysLeft != 2 {
		t.Errorf("in-flight row wrong: %+v", f)
	}
	after, err := os.ReadFile(filepath.Join(cfg.DataDir, "world.json"))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("Read rewrote world.json")
	}
}

// Opening a mistyped path must not create it, which taking the lock would.
func TestOpenRefusesAndCreatesNothing(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "typo")
	if _, err := Read(missing, time.Now()); !errors.Is(err, ErrNoWorld) {
		t.Errorf("Read of a missing directory returned %v, want ErrNoWorld", err)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Errorf("Read created %s", missing)
	}
	if _, err := Open(t.TempDir()); !errors.Is(err, ErrNoWorld) {
		t.Errorf("Open of an empty directory returned %v, want ErrNoWorld", err)
	}
}

// A door's folder opens its data directory.
func TestOpenFindsTheDataDirectoryUnderADoorFolder(t *testing.T) {
	door := t.TempDir()
	data := filepath.Join(door, "data")
	if err := os.MkdirAll(data, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(data, "world.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got, err := Open(door); err != nil || got != data {
		t.Errorf("Open(door) = %q, %v; want %q", got, err, data)
	}
}

func TestBoardStatusIsWords(t *testing.T) {
	b := Board{BBSInfoRow: game.BBSInfoRow{BelowMin: true, OtherRules: true}, Held: true}
	if got := b.Status(); got != "never heard, below min, other rules, held" {
		t.Errorf("Status = %q", got)
	}
}

func command(flag string) Command {
	for _, c := range Commands {
		if c.Flag == flag {
			return c
		}
	}
	panic(flag)
}

func TestArgs(t *testing.T) {
	got, err := command("-maint").Args("/d", true, "")
	if err != nil || strings.Join(got, " ") != "-data /d -maint -detailed" {
		t.Errorf("-maint detailed = %v, %v", got, err)
	}
	got, _ = command("-ftn-status").Args("/d", true, "")
	if strings.Join(got, " ") != "-data /d -ftn-status" {
		t.Errorf("-detailed rode a mode that ignores it: %v", got)
	}
	if _, err := command("-league-reset").Args("/d", false, "next week"); err == nil {
		t.Error("-league-reset took a value that is not a date")
	}
	if _, err := command("-league-freeze").Args("/d", false, "  "); err == nil {
		t.Error("-league-freeze ran with no message")
	}
	got, _ = command("-league-reset").Args("/d", false, "2026-10-01")
	if strings.Join(got, " ") != "-data /d -league-reset 2026-10-01" {
		t.Errorf("-league-reset = %v", got)
	}
	for _, c := range Commands {
		if c.Flag == "-reset" {
			t.Error("-reset is interactive and must not be offered")
		}
		if c.Confirm != "" && !c.Coordinator {
			t.Errorf("%s asks for confirmation but is not a Coordinator command", c.Flag)
		}
	}
}

func TestStartStreamsOutputAndStatus(t *testing.T) {
	if runtime.GOOS == "js" {
		t.Skip("no processes")
	}
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("IB_SYSOP_HELPER", "1")
	var lines []string
	r, err := Start(self, t.TempDir(), []string{"-data", "x", "-maint"}, func(l string) { lines = append(lines, l) })
	if err != nil {
		t.Fatal(err)
	}
	err = r.Err()
	var exit interface{ ExitCode() int }
	if !errors.As(err, &exit) || exit.ExitCode() != 3 {
		t.Errorf("exit status = %v, want 3", err)
	}
	out := strings.Join(lines, "\n")
	if !strings.Contains(out, "args: -data x -maint") || !strings.Contains(out, "to stderr") {
		t.Errorf("output not delivered:\n%s", out)
	}
}
