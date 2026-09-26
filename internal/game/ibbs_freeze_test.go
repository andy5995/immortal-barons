package game

import (
	"testing"
	"time"
)

// freezePair is a Coordinator and one member board sharing a roster and key.
func freezePair(t *testing.T) (lc, m *World) {
	t.Helper()
	roster := []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}
	priv, pub := testCoordKeys(t)
	cfgA := DefaultConfig()
	cfgA.BoardID, cfgA.IBBS = "AlphaBBS", true
	lc = NewWorldSeed(cfgA, 1)
	lc.LeagueNodes, lc.CoordKey, lc.CoordPub = roster, priv, pub
	cfgB := DefaultConfig()
	cfgB.BoardID, cfgB.IBBS = "BravoBBS", true
	m = NewWorldSeed(cfgB, 1)
	m.LeagueNodes, m.CoordPub = roster, pub
	return lc, m
}

// orderIn is the freeze order the Coordinator queued, addressed to board.
func orderIn(t *testing.T, lc *World, board string) Packet {
	t.Helper()
	lc.StampOutbox()
	for _, p := range lc.Outbox {
		// Unaddressed on a mesh roster, which copies every packet to everyone.
		if p.Freeze != nil && (p.ToBoard == board || p.ToBoard == "") {
			return p
		}
	}
	t.Fatalf("no freeze order for %s in the Coordinator's outbox", board)
	return Packet{}
}

// The Coordinator freezes the league, a member obeys the signed order once,
// and only the Coordinator can give one.
func TestLeagueFreezeReachesEveryBoard(t *testing.T) {
	lc, m := freezePair(t)
	if err := lc.DeclareLeagueFreeze(true, "Back in 2-48 hours."); err != nil {
		t.Fatalf("DeclareLeagueFreeze: %v", err)
	}
	if !lc.Frozen {
		t.Error("the Coordinator's own board did not freeze")
	}
	m.ApplyPacket(orderIn(t, lc, "BravoBBS"))
	if !m.Frozen || m.FreezeMessage != "Back in 2-48 hours." {
		t.Errorf("member frozen=%v message=%q", m.Frozen, m.FreezeMessage)
	}
	if err := m.DeclareLeagueFreeze(false, ""); err != ErrNotCoordinator {
		t.Errorf("a member board thawed the league: %v", err)
	}
	if err := lc.DeclareLeagueFreeze(true, ""); err != ErrAlreadyFrozen {
		t.Errorf("a second freeze was accepted: %v", err)
	}
}

// at holds the game clock at noon UTC on date for the rest of the test.
func at(t *testing.T, date string) time.Time {
	t.Helper()
	d, err := time.Parse("2006-01-02", date)
	if err != nil {
		t.Fatal(err)
	}
	now := d.Add(12 * time.Hour)
	restore := timeNow
	timeNow = func() time.Time { return now }
	t.Cleanup(func() { timeNow = restore })
	return now
}

// A frozen board runs no game day, and the thaw skips the frozen days rather
// than catching them up. A day the board owed from BEFORE the freeze is still
// owed: the freeze skips its own days and no others.
func TestAFrozenBoardRunsNoGameDay(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lastMain string // the board's last maintenance before the freeze
		owed     int    // days it still owes once thawed on the 3rd
	}{
		{"maintained the day it froze", "2026-09-01", 0},
		{"two days behind when it froze", "2026-08-30", 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, m := freezePair(t)
			at(t, tc.lastMain)
			m.DailyMaintenance(tc.lastMain)
			m.DailyMaintenance(tc.lastMain) // the first call only starts the clock
			day := m.GameDay
			at(t, "2026-09-01")
			m.applyLeagueFreeze(&LeagueFreeze{Serial: 1, Frozen: true})
			at(t, "2026-09-02")
			if rep := m.DailyMaintenance("2026-09-02"); !rep.Frozen || m.GameDay != day {
				t.Errorf("frozen maintenance: report %+v, game day %d -> %d", rep, day, m.GameDay)
			}
			at(t, "2026-09-03")
			m.applyLeagueFreeze(&LeagueFreeze{Serial: 2})
			m.DailyMaintenance("2026-09-03")
			if got := m.GameDay - day; got != tc.owed {
				t.Errorf("the thaw ran %d days, want %d", got, tc.owed)
			}
		})
	}
}

// The freeze is not silence: after a thaw neither the sysop's alarm nor a
// player's warning counts it against boards that could not send while frozen.
func TestTheThawRaisesNoSilenceAlarm(t *testing.T) {
	_, m := freezePair(t)
	start := at(t, "2026-09-01")
	m.LastPacketFrom = map[string]string{"AlphaBBS": Recorded(start)}
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 1, Frozen: true})
	thaw := at(t, "2026-09-20")
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 2})
	m.NoteSilentLinks(thaw)
	if len(m.SysopNotices) != 0 {
		t.Errorf("the thaw raised a silence alarm: %q", m.SysopNotices)
	}
	if m.LinkQuiet("AlphaBBS", thaw) {
		t.Error("a player is warned the planet went quiet when it was only frozen")
	}
	m.NoteSilentLinks(thaw.Add(time.Duration(LinkSilentAlarmDays+1) * 24 * time.Hour))
	if len(m.SysopNotices) != 1 {
		t.Errorf("a board silent long after the thaw was not reported: %q", m.SysopNotices)
	}
}

// A quiet report that could not be addressed — no roster yet — is tried again
// on the next run instead of being marked sent.
func TestAnUnaddressedQuietReportIsRetried(t *testing.T) {
	_, m := freezePair(t)
	nodes := m.LeagueNodes
	m.LeagueNodes = nil
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 1, Frozen: true})
	m.ReportQuiet()
	if len(m.Outbox) != 0 {
		t.Fatalf("a report was queued with no Coordinator to address: %+v", m.Outbox)
	}
	m.LeagueNodes = nodes
	m.ReportQuiet()
	if len(m.Outbox) != 1 {
		t.Errorf("the report was not retried once the roster arrived")
	}
}

// Deadlines kept as instants are moved on by exactly the time spent frozen,
// and a replayed order does nothing.
func TestTheThawMovesDeadlinesOnByTheFreeze(t *testing.T) {
	_, m := freezePair(t)
	start := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	depart := start.Add(6 * time.Hour)
	m.GroupAttacks = []GroupAttack{{ID: 1, DepartAt: depart}}
	m.Threats = []Threat{{FromBoard: "AlphaBBS", Kind: "group", At: ThreatAt(depart)}}
	restore := timeNow
	defer func() { timeNow = restore }()

	timeNow = func() time.Time { return start }
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 1, Frozen: true})
	timeNow = func() time.Time { return start.Add(30 * time.Hour) }
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 2})
	if got, want := m.GroupAttacks[0].DepartAt, depart.Add(30*time.Hour); !got.Equal(want) {
		t.Errorf("DepartAt = %v, want %v", got, want)
	}
	if got, want := m.Threats[0].When(), depart.Add(30*time.Hour); !got.Equal(want) {
		t.Errorf("threat At = %v, want %v", got, want)
	}
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 1, Frozen: true})
	if m.Frozen {
		t.Error("a replayed freeze order froze the board again")
	}
}

// A frozen board reports to the Coordinator when it last had real traffic, and
// reports again only when that changes.
func TestAFrozenBoardReportsWhenItGoesQuiet(t *testing.T) {
	lc, m := freezePair(t)
	if err := lc.DeclareLeagueFreeze(true, ""); err != nil {
		t.Fatal(err)
	}
	m.ApplyPacket(orderIn(t, lc, "BravoBBS"))
	m.Outbox = nil
	m.ReportQuiet()
	if len(m.Outbox) != 1 || m.Outbox[0].Quiet == nil || m.Outbox[0].ToBoard != "AlphaBBS" {
		t.Fatalf("no quiet report queued for the Coordinator: %+v", m.Outbox)
	}
	m.ReportQuiet()
	if len(m.Outbox) != 1 {
		t.Errorf("an unchanged quiet time was reported twice")
	}
	report := m.Outbox[0]
	report.Seq = 1
	lc.ApplyPacket(report)
	if lc.QuietBoards["BravoBBS"] == "" {
		t.Errorf("the Coordinator did not file the report: %v", lc.QuietBoards)
	}
	lc.ReportQuiet()
	if lc.QuietBoards["AlphaBBS"] == "" {
		t.Errorf("the Coordinator did not file its own report: %v", lc.QuietBoards)
	}
}

// Only the freeze's own packets leave a frozen board.
func TestFrozenSendable(t *testing.T) {
	if FrozenSendable(Packet{Scores: []RemoteScore{{Empire: "x"}}}) {
		t.Error("a scores packet may not leave a frozen board")
	}
	if !FrozenSendable(Packet{Quiet: &QuietReport{}}) || !FrozenSendable(Packet{Freeze: &LeagueFreeze{}}) {
		t.Error("the freeze order and the quiet report must still go out")
	}
	if !FrozenSendable(Packet{LeagueConfig: &LeagueConfig{}}) {
		t.Error("a ruleset the Coordinator sends while frozen must go out, not wait for the thaw")
	}
}

// The Coordinator's signature covers the freeze order, so one attached to a
// genuinely signed packet afterwards is refused rather than obeyed.
func TestAFreezeAddedToASignedPacketIsRefused(t *testing.T) {
	lc, m := freezePair(t)
	p := Packet{FromBoard: "AlphaBBS", Seq: 5, LeagueConfig: lc.Config.leagueRuleset()}
	if err := lc.SignAsCoordinator(&p); err != nil {
		t.Fatal(err)
	}
	if !m.VerifyCoordinatorOrders(p) {
		t.Fatal("the genuine packet did not verify")
	}
	p.Freeze = &LeagueFreeze{Serial: 1, Frozen: true, Message: "forged"}
	if m.VerifyCoordinatorOrders(p) {
		t.Error("a freeze order nobody signed verified")
	}
}

// A frozen board's held packets are numbered when they go out, after the thaw,
// so they arrive ABOVE the quiet report the Coordinator has already applied.
// Numbered while held, they sat below it and the Coordinator dropped every one
// as a replay.
func TestHeldPacketsAreNotDroppedAsReplaysAfterTheThaw(t *testing.T) {
	lc, m := freezePair(t)
	if err := lc.DeclareLeagueFreeze(true, ""); err != nil {
		t.Fatal(err)
	}
	m.ApplyPacket(orderIn(t, lc, "BravoBBS"))
	m.Outbox = []Packet{{FromBoard: "BravoBBS", ToBoard: "AlphaBBS", Notice: "a result held by the freeze"}}
	m.ReportQuiet()
	m.StampOutbox()
	var sent []Packet
	for _, p := range m.Outbox {
		if FrozenSendable(p) {
			sent = append(sent, p)
		}
	}
	m.Outbox = m.Outbox[:1]
	for _, p := range sent {
		lc.ApplyPacket(p)
	}
	m.applyLeagueFreeze(&LeagueFreeze{Serial: m.FreezeSerial + 1})
	m.StampOutbox()
	held := m.Outbox[0]
	if lc.SeenPacket(held) {
		t.Errorf("the held packet (seq %d) was taken for a replay at the Coordinator", held.Seq)
	}
}

// A freeze order that reaches a board still frozen from an earlier freeze,
// because the thaw between them was lost, leaves it frozen.
func TestAFreezeOverAFreezeStaysFrozen(t *testing.T) {
	_, m := freezePair(t)
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 1, Frozen: true, Message: "first"})
	m.ReportQuiet()
	m.Outbox = nil
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 3, Frozen: true, Message: "second"})
	if !m.Frozen || m.FreezeMessage != "second" {
		t.Errorf("frozen=%v message=%q, want still frozen with the new message", m.Frozen, m.FreezeMessage)
	}
	// Its report answered serial 1, which the Coordinator no longer files, so it
	// has to report again under 3.
	m.ReportQuiet()
	if len(m.Outbox) != 1 || m.Outbox[0].Quiet == nil || m.Outbox[0].Quiet.Serial != 3 {
		t.Errorf("no fresh quiet report under the new freeze: %+v", m.Outbox)
	}
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 2})
	if !m.Frozen {
		t.Error("the late thaw from before the second freeze opened the board")
	}
}

// A new season is refused while frozen, and a reset order that reaches a board
// already frozen (sent before the freeze, delivered after) leaves it frozen.
func TestAResetDoesNotThawTheLeague(t *testing.T) {
	lc, m := freezePair(t)
	if err := lc.DeclareLeagueFreeze(true, ""); err != nil {
		t.Fatal(err)
	}
	if err := lc.DeclareLeagueReset("2026-10-01", ""); err != ErrLeagueFrozen {
		t.Errorf("a new season was declared on a frozen league: %v", err)
	}
	m.applyLeagueFreeze(&LeagueFreeze{Serial: 1, Frozen: true, Message: "hold"})
	m.applyLeagueReset(&LeagueReset{Season: m.Season + 1, OnDate: "2026-10-01"})
	if !m.Frozen || m.FreezeSerial != 1 || m.FreezeMessage != "hold" {
		t.Errorf("the reset lost the freeze: frozen=%v serial=%d message=%q", m.Frozen, m.FreezeSerial, m.FreezeMessage)
	}
}
