package game

import (
	"reflect"
	"strings"
	"testing"
)

func TestImportBoardAppendsNewBoard(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.ImportBoard(RemoteBoard{
		BoardID: "wildside",
		Date:    "2026-07-01",
		Scores:  []RemoteScore{{Empire: "Iron Dominion", NetWorth: 50000, Land: 120}},
	})

	if len(w.RemoteBoards) != 1 {
		t.Fatalf("got %d remote boards, want 1", len(w.RemoteBoards))
	}
	if w.RemoteBoards[0].BoardID != "wildside" {
		t.Fatalf("got BoardID %q, want wildside", w.RemoteBoards[0].BoardID)
	}
}

func TestImportBoardReplacesSameBoardID(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.ImportBoard(RemoteBoard{
		BoardID: "wildside",
		Date:    "2026-07-01",
		Scores:  []RemoteScore{{Empire: "Iron Dominion", NetWorth: 50000, Land: 120}},
	})
	w.ImportBoard(RemoteBoard{
		BoardID: "wildside",
		Date:    "2026-07-03",
		Scores:  []RemoteScore{{Empire: "Iron Dominion", NetWorth: 90000, Land: 150}},
	})

	if len(w.RemoteBoards) != 1 {
		t.Fatalf("got %d remote boards, want 1 (should replace, not duplicate)", len(w.RemoteBoards))
	}
	got := w.RemoteBoards[0]
	if got.Date != "2026-07-03" || got.Scores[0].NetWorth != 90000 {
		t.Fatalf("got %+v, want updated board with Date 2026-07-03 and NetWorth 90000", got)
	}
}

func TestImportBoardDifferentBoardIDsBothKept(t *testing.T) {
	w := NewWorld(DefaultConfig())
	w.ImportBoard(RemoteBoard{BoardID: "wildside", Date: "2026-07-01"})
	w.ImportBoard(RemoteBoard{BoardID: "otherboard", Date: "2026-07-02"})

	if len(w.RemoteBoards) != 2 {
		t.Fatalf("got %d remote boards, want 2", len(w.RemoteBoards))
	}
}

func TestGroupAttackRoundTrip(t *testing.T) {
	// Board A assembles a group attack (leader + ally) against an empire on
	// board B; it departs, becomes an outbound packet, and board B resolves it.
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA := NewWorldSeed(cfgA, 1)
	leader := wA.AddHuman("leader", "Alpha")
	ally := wA.AddHuman("ally", "Ally")

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB := NewWorldSeed(cfgB, 1)
	target := wB.AddHuman("victim", "Victim")
	target.Protection = 0 // past new-realm protection, so the strike can land
	target.Regions = RegionMix{Coastal: 100}
	target.syncLand()
	target.Troopers, target.Turrets, target.Tanks = 0, 0, 0 // defenseless

	leader.Troopers, ally.Troopers = 1_000_000, 1_000_000 // troops to commit
	ga, err := wA.CreateGroupAttack(leader, "boardB", "Victim", wA.GameDay+1, AttackForce{Troopers: 100_000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := wA.JoinGroupAttack(ally, ga.ID, AttackForce{Troopers: 50_000}); err != nil {
		t.Fatalf("join: %v", err)
	}

	wA.GameDay++ // reach departure day
	wA.LaunchDueGroupAttacks()
	if len(wA.Outbox) != 1 || len(wA.Outbox[0].Attacks) != 1 {
		t.Fatalf("expected one outbound attack, got %+v", wA.Outbox)
	}
	// 100,000 + 50,000 committed troopers = 150,000 offense (1 each).
	if got := wA.Outbox[0].Attacks[0].Offense; got != 150_000 {
		t.Errorf("combined offense: want 150000, got %d", got)
	}
	if len(wA.GroupAttacks) != 0 {
		t.Errorf("departed attack should be removed from the pending list")
	}

	result := wB.ApplyPacket(wA.Outbox[0])
	if target.Land >= 100 {
		t.Errorf("target should have lost land, still has %d", target.Land)
	}
	if len(result.Results) != 1 || !result.Results[0].Won {
		t.Errorf("attack should have won: %+v", result.Results)
	}
	if result.ToBoard != "boardA" {
		t.Errorf("result should route back to boardA, got %q", result.ToBoard)
	}
}

func TestTerrorOpDestroysForces(t *testing.T) {
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA := NewWorldSeed(cfgA, 1)
	attacker := wA.AddHuman("att", "Attacker")
	attacker.Agents = 10

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB := NewWorldSeed(cfgB, 1)
	target := wB.AddHuman("victim", "Victim")
	target.Protection = 0
	// Units in every slot so each random hit destroys something.
	target.Troopers, target.Jets, target.Turrets = 5000, 700, 700
	target.Tanks, target.Bombers, target.Carriers = 700, 700, 700
	totalBefore := target.Troopers + target.Jets + target.Turrets + target.Tanks + target.Bombers + target.Carriers

	if err := wA.SendTerror(attacker, "boardB", "Victim", 4); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	if attacker.Agents != 6 {
		t.Errorf("agents: want 6 after committing 4, got %d", attacker.Agents)
	}
	if len(wA.Outbox) != 1 || len(wA.Outbox[0].Terrors) != 1 {
		t.Fatalf("expected one outbound terror op, got %+v", wA.Outbox)
	}

	result := wB.ApplyPacket(wA.Outbox[0])
	res := result.Results[0]
	if !res.Won || res.Kind != "terror" || res.LandTaken <= 0 {
		t.Errorf("expected a won terror result with forces destroyed, got %+v", res)
	}
	totalAfter := target.Troopers + target.Jets + target.Turrets + target.Tanks + target.Bombers + target.Carriers
	if totalAfter >= totalBefore {
		t.Errorf("target should have lost forces: before %d, after %d", totalBefore, totalAfter)
	}
}

func TestTerrorOpBlockedByProtection(t *testing.T) {
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA := NewWorldSeed(cfgA, 1)
	attacker := wA.AddHuman("att", "Attacker")
	attacker.Agents = 10

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB := NewWorldSeed(cfgB, 1)
	target := wB.AddHuman("victim", "Victim")
	target.Protection = 3
	target.Troopers = 5000

	if err := wA.SendTerror(attacker, "boardB", "Victim", 4); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	result := wB.ApplyPacket(wA.Outbox[0])
	if target.Troopers != 5000 {
		t.Errorf("protected target should keep all troopers, got %d", target.Troopers)
	}
	if result.Results[0].Won {
		t.Errorf("terror op against a protected target should not win")
	}
}

func TestRemoteAttackBlockedByProtection(t *testing.T) {
	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB := NewWorldSeed(cfgB, 1)
	target := wB.AddHuman("victim", "Victim")
	target.Protection = 3
	target.Troopers = 100 // weak defense: without the guard the strike would take land
	landBefore := target.Land

	// An overwhelming interplanetary strike lands on the protected realm.
	pkt := Packet{FromBoard: "boardA", ToBoard: "boardB", Attacks: []RemoteAttack{{
		ID: 1, FromBoard: "boardA", TargetEmpire: "Victim", Offense: 1_000_000,
	}}}
	result := wB.ApplyPacket(pkt)

	if target.Land != landBefore {
		t.Errorf("protected target should lose no land, %d -> %d", landBefore, target.Land)
	}
	if result.Results[0].Won || result.Results[0].LandTaken != 0 {
		t.Errorf("remote attack against a protected target should not win, got %+v", result.Results[0])
	}
	if n := len(target.Events); n == 0 || !strings.Contains(target.Events[n-1].Text, "New Realm Protection") {
		t.Errorf("protected target should be told the strike was stopped, events: %v", target.Events)
	}
}

func TestJoinDepartedAttackFails(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	e := w.AddHuman("e", "E")
	e.Troopers = 10_000
	ga, _ := w.CreateGroupAttack(e, "boardB", "", w.GameDay, AttackForce{Troopers: 1000}) // departs today
	if err := w.JoinGroupAttack(e, ga.ID, AttackForce{Troopers: 500}); err != ErrDeparted {
		t.Errorf("joining a departed attack should fail with ErrDeparted, got %v", err)
	}
	if err := w.JoinGroupAttack(e, 999, AttackForce{Troopers: 500}); err != ErrNoAttack {
		t.Errorf("unknown id should fail with ErrNoAttack, got %v", err)
	}
}

func TestGroupAttackReturnsSurvivors(t *testing.T) {
	cfgA := DefaultConfig()
	cfgA.BoardID = "boardA"
	wA := NewWorldSeed(cfgA, 1)
	leader := wA.AddHuman("leader", "Leader")
	leader.Troopers, leader.Tanks = 100_000, 1000

	cfgB := DefaultConfig()
	cfgB.BoardID = "boardB"
	wB := NewWorldSeed(cfgB, 1)
	target := wB.AddHuman("victim", "Victim")
	target.Protection = 0 // past new-realm protection, so the strike can land
	target.Regions = RegionMix{Coastal: 100}
	target.syncLand()
	target.Troopers, target.Turrets, target.Tanks = 0, 0, 0 // defenseless

	_, err := wA.CreateGroupAttack(leader, "boardB", "Victim", wA.GameDay+1, AttackForce{Troopers: 100_000, Tanks: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if leader.Troopers != 0 || leader.Tanks != 0 {
		t.Fatalf("committed units should be deducted, have %d troopers %d tanks", leader.Troopers, leader.Tanks)
	}

	wA.GameDay++
	wA.LaunchDueGroupAttacks()
	result := wB.ApplyPacket(wA.Outbox[0]) // B resolves, returns survivors to A
	wA.ApplyPacket(result)                 // A restores survivors

	// 15% lost, 85% return: 85,000 troopers and 850 tanks.
	if leader.Troopers != 85_000 || leader.Tanks != 850 {
		t.Errorf("survivors returned wrong: have %d troopers, %d tanks (want 85000, 850)", leader.Troopers, leader.Tanks)
	}
}

func TestBBSCoordinatorElection(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 1)
	a := w.AddHuman("a", "Alpha")
	b := w.AddHuman("b", "Bravo")
	c := w.AddHuman("c", "Charlie")

	if w.BBSCoordinator() != nil {
		t.Error("no votes cast yet -> no coordinator")
	}
	w.VoteCoordinator(a, "b") // Bravo: 1
	w.VoteCoordinator(c, "b") // Bravo: 2
	w.VoteCoordinator(b, "a") // Alpha: 1
	if co := w.BBSCoordinator(); co != b {
		t.Errorf("Bravo has the most votes and should be coordinator, got %v", co)
	}
	// Changing a vote re-elects.
	w.VoteCoordinator(c, "a") // now Alpha 2, Bravo 1
	if co := w.BBSCoordinator(); co != a {
		t.Errorf("after re-vote Alpha should be coordinator, got %v", co)
	}
}

func TestLeagueConfigOnlyFromCoordinator(t *testing.T) {
	roster := []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}

	// Coordinator board (Alpha = node #1) authors the league config.
	cfgA := DefaultConfig()
	cfgA.BoardID, cfgA.GameLength, cfgA.TurnsPerDay = "AlphaBBS", 42, 15
	wA := NewWorldSeed(cfgA, 1)
	wA.LeagueNodes = roster
	if !wA.IsLeagueCoordinator() {
		t.Fatal("Alpha (node #1) should be the League Coordinator")
	}
	wA.ExportLeagueConfig()
	pkt := wA.Outbox[0]

	// Member board (Bravo) adopts the coordinator's config.
	cfgB := DefaultConfig()
	cfgB.BoardID, cfgB.GameLength, cfgB.TurnsPerDay = "BravoBBS", 0, 10
	wB := NewWorldSeed(cfgB, 1)
	wB.LeagueNodes = roster
	wB.ApplyPacket(pkt)
	if wB.Config.GameLength != 42 || wB.Config.TurnsPerDay != 15 {
		t.Errorf("member should adopt LC config, got length=%d turns=%d", wB.Config.GameLength, wB.Config.TurnsPerDay)
	}

	// A config packet from a NON-coordinator board must be ignored.
	cfgC := DefaultConfig()
	cfgC.BoardID, cfgC.GameLength = "CharlieBBS", 5
	wC := NewWorldSeed(cfgC, 1)
	wC.LeagueNodes = roster
	wC.ApplyPacket(Packet{FromBoard: "BravoBBS", LeagueConfig: &LeagueConfig{GameLength: 999}})
	if wC.Config.GameLength != 5 {
		t.Errorf("config from a non-coordinator board must be ignored, got %d", wC.Config.GameLength)
	}
}

// TestLeagueRulesetCarriesEveryField guards the one silent failure this mapping
// has: a field added to LeagueConfig but missed in leagueRuleset or
// applyLeagueRuleset still compiles, and simply never reaches the other boards.
// Walking the struct by reflection means a field added later is covered the day
// it is added, without anyone remembering to extend a list.
func TestLeagueRulesetCarriesEveryField(t *testing.T) {
	lc := reflect.TypeOf(LeagueConfig{})
	for i := 0; i < lc.NumField(); i++ {
		name := lc.Field(i).Name
		src := DefaultConfig()
		field := reflect.ValueOf(&src).Elem().FieldByName(name)
		if !field.IsValid() {
			t.Errorf("LeagueConfig.%s has no Config field of the same name", name)
			continue
		}
		want := distinctFrom(t, field)
		field.Set(want)

		var dst Config
		dst.applyLeagueRuleset(src.leagueRuleset())

		got := reflect.ValueOf(dst).FieldByName(name)
		if !reflect.DeepEqual(got.Interface(), want.Interface()) {
			t.Errorf("%s does not survive the league broadcast: got %v, want %v", name, got, want)
		}
	}
}

// distinctFrom returns a value of v's type that differs from what v holds, so a
// field that is silently dropped cannot pass by coincidence.
func distinctFrom(t *testing.T, v reflect.Value) reflect.Value {
	t.Helper()
	out := reflect.New(v.Type()).Elem()
	switch v.Kind() {
	case reflect.Bool:
		out.SetBool(!v.Bool())
	case reflect.Int:
		out.SetInt(v.Int() + 4242)
	case reflect.String:
		out.SetString(v.String() + "-broadcast-probe")
	default:
		t.Fatalf("distinctFrom: unhandled kind %s", v.Kind())
	}
	return out
}

// A league board never gets computer barons, and none can be injected into one
// later — an inter-BBS game is played between the boards' human realms.
func TestLeagueBoardGetsNoAIBarons(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.AICount = 5
	w := NewWorldSeed(cfg, 1)
	if got := len(w.Empires); got != 0 {
		t.Errorf("a league board seeded %d empires, want 0", got)
	}
	if added := w.AddAIEmpires(3); added != 0 {
		t.Errorf("injected %d AI barons into a league board, want 0", added)
	}
	if got := len(w.Empires); got != 0 {
		t.Errorf("league board has %d empires after an injection attempt, want 0", got)
	}

	// The same config off a league seeds normally, so the guard is the league
	// flag and not something else.
	cfg.IBBS = false
	if solo := NewWorldSeed(cfg, 1); len(solo.Empires) != 5 {
		t.Errorf("stand-alone board seeded %d AI barons, want 5", len(solo.Empires))
	}
}

// A strike whose result packet never comes home gives its forces back after the
// configured wait, so a lost packet does not cost a player their army (#96).
func TestLostForcesComeHome(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.LostForcesDays = 3
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Troopers, e.Tanks, e.Agents = 1000, 40, 20

	f := AttackForce{Troopers: 600, Tanks: 25}
	if _, err := w.CreateGroupAttack(e, "faraway", "Rome", w.GameDay, f); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	if err := w.SendTerror(e, "faraway", "Rome", 5); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	w.LaunchDueGroupAttacks()
	if e.Troopers != 400 || e.Tanks != 15 || e.Agents != 15 {
		t.Fatalf("after launching: troopers=%d tanks=%d agents=%d, want 400/15/15", e.Troopers, e.Tanks, e.Agents)
	}
	if len(w.InFlight) != 2 {
		t.Fatalf("in flight = %d, want 2 (the strike and the terror op)", len(w.InFlight))
	}

	// Too soon: the forces are still out there.
	w.GameDay += 2
	if got := w.ReturnLostForces(); got != 0 {
		t.Errorf("recovered %d strikes after 2 days, want 0 — the wait is %d", got, cfg.LostForcesDays)
	}
	if e.Troopers != 400 {
		t.Errorf("troopers came home early: %d, want 400", e.Troopers)
	}

	// The wait is up and nothing came back, so everything returns.
	w.GameDay++
	if got := w.ReturnLostForces(); got != 2 {
		t.Errorf("recovered %d strikes, want 2", got)
	}
	if e.Troopers != 1000 || e.Tanks != 40 || e.Agents != 20 {
		t.Errorf("after recovery: troopers=%d tanks=%d agents=%d, want 1000/40/20", e.Troopers, e.Tanks, e.Agents)
	}
	if len(w.InFlight) != 0 {
		t.Errorf("%d strikes still waiting after recovery, want 0", len(w.InFlight))
	}
}

// A result that does arrive clears the strike, so the timer never fires for it
// and the forces are not returned twice.
func TestAnsweredStrikeDoesNotAlsoTimeOut(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.LostForcesDays = 1
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Troopers = 500

	if _, err := w.CreateGroupAttack(e, "faraway", "Rome", w.GameDay, AttackForce{Troopers: 300}); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	w.LaunchDueGroupAttacks()
	id := w.InFlight[0].ID

	w.ApplyPacket(Packet{
		FromBoard: "faraway",
		Results: []AttackResult{{
			ID: id, TargetBoard: "faraway", TargetEmpire: "Rome",
			Survivors: []Contribution{{Owner: "alice", AttackForce: AttackForce{Troopers: 200}}},
		}},
	})
	if e.Troopers != 400 {
		t.Fatalf("survivors returned: troopers=%d, want 400 (200 home, 100 lost)", e.Troopers)
	}
	w.GameDay += 5
	if got := w.ReturnLostForces(); got != 0 {
		t.Errorf("an answered strike timed out anyway (%d recovered)", got)
	}
	if e.Troopers != 400 {
		t.Errorf("troopers returned twice: %d, want 400", e.Troopers)
	}
}
