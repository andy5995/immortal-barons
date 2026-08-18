package game

import (
	"crypto/ed25519"
	"crypto/rand"
	"reflect"
	"strings"
	"testing"
	"time"
)

// afterDeparture is a moment past a group attack filed with the shortest
// permitted delay, so a test can watch it leave without waiting twelve hours.
func afterDeparture() time.Time {
	return time.Now().Add((GroupAttackHoursMin + 1) * time.Hour)
}

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
	ga, err := wA.CreateGroupAttack(leader, "boardB", "Victim", GroupAttackHoursMin, AttackForce{Troopers: 100_000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := wA.JoinGroupAttack(ally, ga.ID, AttackForce{Troopers: 50_000}); err != nil {
		t.Fatalf("join: %v", err)
	}

	wA.LaunchDueGroupAttacksAt(afterDeparture()) // the delay has run out
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
	ga, _ := w.CreateGroupAttack(e, "boardB", "", GroupAttackHoursMin, AttackForce{Troopers: 1000})
	w.GroupAttacks[0].DepartAt = time.Now().Add(-time.Hour) // its force has left
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

	_, err := wA.CreateGroupAttack(leader, "boardB", "Victim", GroupAttackHoursMin, AttackForce{Troopers: 100_000, Tanks: 1000})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if leader.Troopers != 0 || leader.Tanks != 0 {
		t.Fatalf("committed units should be deducted, have %d troopers %d tanks", leader.Troopers, leader.Tanks)
	}

	wA.LaunchDueGroupAttacksAt(afterDeparture())
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
	priv, pub := testCoordKeys(t)
	wA.CoordKey, wA.CoordPub = priv, pub
	wA.ExportLeagueConfig()
	wA.StampOutbox() // numbers and signs it, as the planetary step does
	pkt := wA.Outbox[0]

	// Member board (Bravo) adopts the coordinator's config.
	cfgB := DefaultConfig()
	cfgB.BoardID, cfgB.GameLength, cfgB.TurnsPerDay = "BravoBBS", 0, 10
	wB := NewWorldSeed(cfgB, 1)
	wB.LeagueNodes = roster
	wB.CoordPub = pub
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
	if _, err := w.CreateGroupAttack(e, "faraway", "Rome", GroupAttackHoursMin, f); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	if err := w.SendTerror(e, "faraway", "Rome", 5); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}
	w.LaunchDueGroupAttacksAt(afterDeparture())
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

	if _, err := w.CreateGroupAttack(e, "faraway", "Rome", GroupAttackHoursMin, AttackForce{Troopers: 300}); err != nil {
		t.Fatalf("CreateGroupAttack: %v", err)
	}
	w.LaunchDueGroupAttacksAt(afterDeparture())
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

// The Coordinator broadcasts the league roster and members adopt it, so boards
// joining or moving do not mean every sysop editing their own node list (#64).
func TestCoordinatorBroadcastsTheRoster(t *testing.T) {
	roster := []LeagueNode{
		{Number: 1, Name: "GraveyardBBS", Address: "1:20/100", City: "Graveyard", State: "XX", Country: "USA"},
		{Number: 2, Name: "Wildside", Address: "1:20/101", City: "Testville", State: "XX", Country: "USA"},
	}

	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "GraveyardBBS" // node 1 = the Coordinator
	lc := NewWorldSeed(cfg, 1)
	lc.LeagueNodes = roster
	priv, pub := testCoordKeys(t)
	lc.CoordKey, lc.CoordPub = priv, pub
	lc.ExportNodeList()
	lc.StampOutbox()
	if len(lc.Outbox) != 1 || len(lc.Outbox[0].LeagueNodes) != 2 {
		t.Fatalf("coordinator queued %d packets, want 1 carrying the roster", len(lc.Outbox))
	}

	memberCfg := cfg
	memberCfg.BoardID = "Wildside"
	member := NewWorldSeed(memberCfg, 1)
	member.LeagueNodes = roster // knows the roster well enough to name the LC
	member.CoordPub = pub
	member.ApplyPacket(lc.Outbox[0])
	if len(member.LeagueNodes) != 2 || member.LeagueNodes[1].Name != "Wildside" {
		t.Errorf("member did not adopt the roster: %+v", member.LeagueNodes)
	}

	// A member must not be able to push a roster onto other boards.
	member.Outbox = nil
	member.ExportNodeList()
	if len(member.Outbox) != 0 {
		t.Errorf("a member board broadcast a roster: %+v", member.Outbox)
	}

	// Nor may a roster from anyone but the Coordinator be adopted.
	before := len(member.LeagueNodes)
	member.ApplyPacket(Packet{FromBoard: "Impostor", LeagueNodes: []LeagueNode{{Number: 9, Name: "Bogus"}}})
	if len(member.LeagueNodes) != before {
		t.Errorf("member adopted a roster from a non-coordinator board: %+v", member.LeagueNodes)
	}
}

// An individual interplanetary attack leaves at once, spends one of the day's
// individual attacks, and must name its target (#62).
func TestIndividualInterplanetaryAttack(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.MaxIndividualAttacks = 1
	w := NewWorldSeed(cfg, 1)
	e := w.AddHuman("alice", "Alethia")
	e.Troopers, e.Tanks = 900, 30

	if _, err := w.CreateIndividualAttack(e, "faraway", "", NormalAttack, AttackForce{Troopers: 100}); err != ErrNoTarget {
		t.Errorf("a whole-planet individual attack should be refused, got %v", err)
	}

	id, err := w.CreateIndividualAttack(e, "faraway", "Rome", NormalAttack, AttackForce{Troopers: 500, Tanks: 20})
	if err != nil {
		t.Fatalf("CreateIndividualAttack: %v", err)
	}
	if e.Troopers != 400 || e.Tanks != 10 {
		t.Errorf("force not committed: troopers=%d tanks=%d, want 400/10", e.Troopers, e.Tanks)
	}
	// It is away immediately — queued for the target board, and on the waiting
	// list so it can time out like any other strike.
	if len(w.Outbox) != 1 || len(w.Outbox[0].Attacks) != 1 || w.Outbox[0].Attacks[0].ID != id {
		t.Fatalf("strike was not queued for the target board: %+v", w.Outbox)
	}
	if len(w.InFlight) != 1 || w.InFlight[0].ID != id {
		t.Errorf("strike is not on the waiting list: %+v", w.InFlight)
	}

	// The day's one individual attack is spent.
	if _, err := w.CreateIndividualAttack(e, "faraway", "Rome", NormalAttack, AttackForce{Troopers: 10}); err != ErrAttacksExhausted {
		t.Errorf("second attack should exhaust the daily allowance, got %v", err)
	}
	if e.Troopers != 400 {
		t.Errorf("a refused attack still took troopers: %d, want 400", e.Troopers)
	}
}

// Cross-board scouting is a real round trip: the request travels out, the target
// board answers with figures read on its own side, and the answer lands in the
// asking board's Spy Database (#61).
func TestReconRoundTrip(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "Wildside"
	asker := NewWorldSeed(cfg, 1)
	scout := asker.AddHuman("alice", "Alethia")
	scout.Agents = 3

	if err := asker.SendRecon(scout, "faraway", "Rome"); err != nil {
		t.Fatalf("SendRecon: %v", err)
	}
	if scout.Agents != 2 {
		t.Errorf("recon cost %d agents, want 1", 3-scout.Agents)
	}
	if len(asker.Outbox) != 1 || len(asker.Outbox[0].Recon) != 1 {
		t.Fatalf("no recon request was queued: %+v", asker.Outbox)
	}

	// The far board answers from its own state, not from anything the asker sent.
	farCfg := DefaultConfig()
	farCfg.IBBS = true
	farCfg.BoardID = "faraway"
	far := NewWorldSeed(farCfg, 1)
	rome := far.AddHuman("bob", "Rome")
	rome.Land, rome.Gold, rome.Troopers = 4321, 99000, 5000
	reply := far.ApplyPacket(asker.Outbox[0])
	if len(reply.ReconReports) != 1 {
		t.Fatalf("target board sent %d reports, want 1", len(reply.ReconReports))
	}
	if got := reply.ReconReports[0]; got.Land != 4321 || got.Gold != 99000 || got.Offense == 0 {
		t.Errorf("report does not carry the target's real figures: %+v", got)
	}
	// The scouted baron notices.
	if len(rome.Events) == 0 {
		t.Error("the scouted realm was told nothing about foreign agents")
	}

	// And the answer files itself on the asking board.
	asker.ApplyPacket(reply)
	if len(asker.SpyDatabase) != 1 || asker.SpyDatabase[0].Land != 4321 {
		t.Errorf("Spy Database did not receive the report: %+v", asker.SpyDatabase)
	}
}

// The Clingy Annihilator runs the original's whole lifecycle: a planet starts one,
// its barons fund it between them, and only then can it launch (#16).
func TestClingyAnnihilatorLifecycle(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "Wildside"
	w := NewWorldSeed(cfg, 1)
	founder := w.AddHuman("alice", "Alethia")
	backer := w.AddHuman("bob", "Bobland")
	founder.Land, backer.Land = 5000, 5000
	w.ImportBoard(RemoteBoard{BoardID: "faraway", Scores: []RemoteScore{{Empire: "Rome", Land: 20000}}})

	if err := w.StartAnnihilator(founder, "faraway"); err != nil {
		t.Fatalf("StartAnnihilator: %v", err)
	}
	if err := w.StartAnnihilator(founder, "faraway"); err != ErrAnnihilatorExists {
		t.Errorf("a planet built a second weapon: %v", err)
	}
	cost := w.Annihilator.CostMillion
	if cost <= 0 {
		t.Fatalf("weapon costs %d million", cost)
	}
	// It cannot fly on a promise.
	if err := w.LaunchAnnihilator(founder); err != ErrAnnihilatorUnfunded {
		t.Errorf("an unfunded weapon launched: %v", err)
	}

	// Two barons fund it between them — that is the point of the thing.
	founder.Gold = int64(cost / 2 * AnnihilatorMillion)
	backer.Gold = int64(cost * AnnihilatorMillion)
	if _, err := w.FundAnnihilator(founder, cost/2); err != nil {
		t.Fatalf("founder funding: %v", err)
	}
	if w.Annihilator.Funded {
		t.Error("weapon reported complete while only half paid for")
	}
	if _, err := w.FundAnnihilator(backer, cost); err != nil {
		t.Fatalf("backer funding: %v", err)
	}
	if !w.Annihilator.Funded {
		t.Fatal("weapon is fully paid for but not complete")
	}
	if w.Annihilator.PaidMillion != cost {
		t.Errorf("took %d million, want exactly %d", w.Annihilator.PaidMillion, cost)
	}

	// Only its creator may launch it.
	if err := w.LaunchAnnihilator(backer); err != ErrNotYours {
		t.Errorf("a baron launched someone else's weapon: %v", err)
	}
	if err := w.LaunchAnnihilator(founder); err != nil {
		t.Fatalf("LaunchAnnihilator: %v", err)
	}
	if w.Annihilator.ArrivesDay != w.GameDay+AnnihilatorFlightDays {
		t.Errorf("arrives day %d, want %d", w.Annihilator.ArrivesDay, w.GameDay+AnnihilatorFlightDays)
	}
	if err := w.DismantleAnnihilator(founder); err != ErrAnnihilatorFlying {
		t.Errorf("a weapon in flight was dismantled: %v", err)
	}
}

// The target planet is told the weapon exists while it is still being built, and
// again when it launches, and its jets can shoot it down in flight (#63, #16).
func TestAnnihilatorIsVisibleAndCanBeShotDown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "faraway"
	target := NewWorldSeed(cfg, 1)
	defender := target.AddHuman("bob", "Rome")
	defender.Land, defender.Jets = 9000, 30_000

	// Under construction: a warning, and nothing to shoot at yet.
	target.ApplyPacket(Packet{FromBoard: "Wildside", Annihilator: &AnnihilatorStatus{FromBoard: "Wildside"}})
	if target.Incoming == nil {
		t.Fatal("target was not told about the weapon being built")
	}
	if target.Incoming.Launched {
		t.Error("a weapon still under construction is marked as flying")
	}

	// Launched: now it is in the air.
	target.ApplyPacket(Packet{FromBoard: "Wildside", Annihilator: &AnnihilatorStatus{
		FromBoard: "Wildside", Funded: true, Launched: true, ArrivesDay: target.GameDay + 2, Intact: 100,
	}})
	if !target.Incoming.Launched {
		t.Fatal("target does not know the weapon has launched")
	}

	// Jets, and only jets, whittle it down.
	knocked, err := target.InterceptAnnihilator(defender, 5000)
	if err != nil {
		t.Fatalf("InterceptAnnihilator: %v", err)
	}
	if knocked != 5000/AnnihilatorJetsPerPercent {
		t.Errorf("5,000 jets knocked %d%% off, want %d%%", knocked, 5000/AnnihilatorJetsPerPercent)
	}
	if defender.Jets != 25_000 {
		t.Errorf("jets not spent: %d, want 25,000", defender.Jets)
	}
	if target.Incoming.Intact != 100-knocked {
		t.Errorf("weapon is %d%% intact, want %d%%", target.Incoming.Intact, 100-knocked)
	}

	// Enough jets destroy it outright, and then nothing arrives.
	defender.Jets = 100_000
	if _, err := target.InterceptAnnihilator(defender, 100_000); err != nil {
		t.Fatalf("InterceptAnnihilator: %v", err)
	}
	if target.Incoming != nil {
		t.Fatalf("weapon survived a full interception: %+v", target.Incoming)
	}
	before := defender.Land
	target.GameDay += 5
	target.ArriveAnnihilator()
	if defender.Land != before {
		t.Errorf("a destroyed weapon still detonated: land %d, was %d", defender.Land, before)
	}
}

// A weapon that gets through takes the same share of every realm's land. The
// SDI is no defence against it: only jets can reach the thing (#111).
func TestAnnihilatorDetonationHitsThePlanet(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	w := NewWorldSeed(cfg, 1)
	plain := w.AddHuman("bob", "Rome")
	shielded := w.AddHuman("carol", "Carthage")
	for _, e := range []*Empire{plain, shielded} {
		e.Regions = RegionMix{Agricultural: 10_000}
		e.syncLand()
	}
	shielded.SDI = SDIMax

	w.Incoming = &Annihilator{Creator: "Wildside", Launched: true, ArrivesDay: w.GameDay, Intact: 100}
	w.ArriveAnnihilator()

	if plain.Land != 9000 {
		t.Errorf("unshielded realm has %d land, want 9,000 (10%% lost)", plain.Land)
	}
	if shielded.Land != 9000 {
		t.Errorf("shielded realm has %d land, want 9,000 — the SDI must not blunt it", shielded.Land)
	}
	if w.Incoming != nil {
		t.Error("the weapon was not consumed on arrival")
	}
}

// testCoordKeys makes a throwaway Coordinator key pair for the league tests.
func testCoordKeys(t *testing.T) (priv, pub []byte) {
	t.Helper()
	pk, sk, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating a coordinator key: %v", err)
	}
	return sk, pk
}

// Only the Coordinator can give the league orders, and only once. An unsigned
// order, one signed with the wrong key, and a replayed copy of a genuine one are
// all refused (#53).
func TestLeagueOrdersNeedTheCoordinatorsSignature(t *testing.T) {
	roster := []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}
	priv, pub := testCoordKeys(t)

	newMember := func() *World {
		cfg := DefaultConfig()
		cfg.BoardID, cfg.IBBS, cfg.GameLength = "BravoBBS", true, 7
		w := NewWorldSeed(cfg, 1)
		w.LeagueNodes = roster
		w.CoordPub = pub
		return w
	}

	// Unsigned, but claiming to be the Coordinator.
	m := newMember()
	m.ApplyPacket(Packet{FromBoard: "AlphaBBS", Seq: 1, LeagueConfig: &LeagueConfig{GameLength: 99}})
	if m.Config.GameLength != 7 {
		t.Errorf("an unsigned order was obeyed: game length %d", m.Config.GameLength)
	}

	// Signed with a key that is not the league's.
	other, _ := testCoordKeys(t)
	forged := Packet{FromBoard: "AlphaBBS", Seq: 2, LeagueConfig: &LeagueConfig{GameLength: 98}}
	forgedWorld := newMember()
	forgedWorld.CoordKey = other
	_ = forgedWorld.SignAsCoordinator(&forged)
	m = newMember()
	m.ApplyPacket(forged)
	if m.Config.GameLength != 7 {
		t.Errorf("an order signed with the wrong key was obeyed: game length %d", m.Config.GameLength)
	}

	// Genuine: signed by the Coordinator's key.
	genuine := Packet{FromBoard: "AlphaBBS", Seq: 3, LeagueConfig: &LeagueConfig{GameLength: 42, TurnsPerDay: 15}}
	signer := newMember()
	signer.CoordKey = priv
	if err := signer.SignAsCoordinator(&genuine); err != nil {
		t.Fatalf("signing: %v", err)
	}
	m = newMember()
	m.ApplyPacket(genuine)
	if m.Config.GameLength != 42 {
		t.Fatalf("a genuine order was refused: game length %d", m.Config.GameLength)
	}

	// The same packet a second time changes nothing.
	m.Config.GameLength = 7
	m.ApplyPacket(genuine)
	if m.Config.GameLength != 7 {
		t.Errorf("a replayed order was obeyed a second time: game length %d", m.Config.GameLength)
	}

	// An older sequence number from the same board is a replay too.
	m2 := newMember()
	m2.ApplyPacket(genuine)
	stale := Packet{FromBoard: "AlphaBBS", Seq: 2, LeagueConfig: &LeagueConfig{GameLength: 5}}
	signer.Outbox = nil
	_ = signer.SignAsCoordinator(&stale)
	m2.ApplyPacket(stale)
	if m2.Config.GameLength != 42 {
		t.Errorf("a stale order was obeyed: game length %d", m2.Config.GameLength)
	}
}

// The Coordinator can start a new season across the league, and a board carries
// it out once (#65).
func TestLeagueWideReset(t *testing.T) {
	roster := []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}
	priv, pub := testCoordKeys(t)

	cfgA := DefaultConfig()
	cfgA.BoardID, cfgA.IBBS = "AlphaBBS", true
	lc := NewWorldSeed(cfgA, 1)
	lc.LeagueNodes = roster
	lc.CoordKey, lc.CoordPub = priv, pub
	lc.AddHuman("alice", "Alethia")

	if err := lc.DeclareLeagueReset("2026-09-01", "Season two begins."); err != nil {
		t.Fatalf("DeclareLeagueReset: %v", err)
	}
	if len(lc.Empires) != 0 {
		t.Errorf("the Coordinator's own board kept %d realms through the reset", len(lc.Empires))
	}
	if len(lc.LeagueNodes) != 2 || len(lc.CoordKey) == 0 {
		t.Error("the reset threw away the league identity it must keep")
	}
	var order Packet
	for _, p := range lc.Outbox {
		if p.Reset != nil {
			order = p
		}
	}
	if order.Reset == nil {
		t.Fatal("no reset order was queued for the other boards")
	}

	cfgB := DefaultConfig()
	cfgB.BoardID, cfgB.IBBS = "BravoBBS", true
	m := NewWorldSeed(cfgB, 1)
	m.LeagueNodes = roster
	m.CoordPub = pub
	m.AddHuman("bob", "Bobland")
	m.ApplyPacket(order)
	if len(m.Empires) != 0 {
		t.Errorf("member board kept %d realms through the league reset", len(m.Empires))
	}
	if m.Season != order.Reset.Season {
		t.Errorf("member is on season %d, the league is on %d", m.Season, order.Reset.Season)
	}

	// A member cannot start a season itself.
	if err := m.DeclareLeagueReset("2026-10-01", ""); err != ErrNotCoordinator {
		t.Errorf("a member board declared a league reset: %v", err)
	}
}

// The Coordinator rebroadcasts the roster and ruleset on every run, so a board
// that already holds them must not post a news line each time — six identical
// "updated the league roster" lines in one day is what this prevents.
func TestUnchangedLeagueBroadcastIsQuiet(t *testing.T) {
	roster := []LeagueNode{{Number: 1, Name: "Alpha BBS"}, {Number: 2, Name: "Bravo BBS"}}
	priv, pub := testCoordKeys(t)

	lc := NewWorldSeed(DefaultConfig(), 1)
	lc.Config.BoardID = "Alpha BBS"
	lc.LeagueNodes = roster
	lc.CoordKey, lc.CoordPub = priv, pub

	member := NewWorldSeed(DefaultConfig(), 1)
	member.Config.BoardID = "Bravo BBS"
	member.LeagueNodes = roster
	member.CoordPub = pub

	broadcast := func() {
		lc.Outbox = nil
		lc.ExportLeagueConfig()
		lc.ExportNodeList()
		lc.StampOutbox()
		for _, pkt := range lc.Outbox {
			member.ApplyPacket(pkt)
		}
	}
	broadcast() // first one adopts and reports
	member.NewsToday = nil
	broadcast() // second carries nothing new

	for _, line := range member.NewsToday {
		t.Errorf("an unchanged broadcast posted news: %q", line)
	}

	// A roster that really changed is still reported.
	member.LeagueNodes = roster[:1]
	broadcast()
	if len(member.NewsToday) == 0 {
		t.Error("a roster that actually changed should be reported")
	}
}

// TestCoordinatorRenameStillRecognizedByNode is #105's core claim: the
// roster's node number is the identity, so a Coordinator that has renamed
// itself is still obeyed by a member whose own roster copy has not caught up
// with the new name yet.
func TestCoordinatorRenameStillRecognizedByNode(t *testing.T) {
	coordPub, coordSec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	lc := NewWorldSeed(Config{BoardID: "NewCoordName"}, 1)
	lc.CoordKey = coordSec
	lc.LeagueNodes = []LeagueNode{{Number: 1, Name: "NewCoordName"}, {Number: 2, Name: "Member"}}
	lc.Outbox = []Packet{{FromBoard: "NewCoordName", ToBoard: "Member",
		LeagueNodes: []LeagueNode{{Number: 1, Name: "NewCoordName"}, {Number: 2, Name: "Member"}, {Number: 3, Name: "NewMember"}}}}
	lc.StampOutbox()
	order := lc.Outbox[0]
	if order.FromNode != 1 {
		t.Fatalf("StampOutbox did not stamp FromNode: %+v", order)
	}

	// The member's own roster is stale: it still has the Coordinator's OLD name.
	m := NewWorldSeed(Config{BoardID: "Member"}, 1)
	m.CoordPub = coordPub
	m.LeagueNodes = []LeagueNode{{Number: 1, Name: "OldCoordName"}, {Number: 2, Name: "Member"}}

	m.ApplyPacket(order)

	found := false
	for _, n := range m.LeagueNodes {
		if n.Name == "NewMember" {
			found = true
		}
	}
	if !found {
		t.Errorf("coordinator orders were refused after a rename, even though the node number still matches; news:\n%s", newsText(m))
	}
}

// perBoardConfigFields are the Config fields a league does NOT broadcast. The
// rule they are the exception to: anything that changes how the local game plays
// must be the same on every planet, or the season is not a fair one. So a field
// earns a place here only by being identity, a file path, or session policy —
// never by being a rule.
var perBoardConfigFields = map[string]string{
	"DataDir":         "where this board keeps its files",
	"BoardID":         "this board's name in the league",
	"InboundDir":      "packet path, this machine's business",
	"OutboundDir":     "packet path, this machine's business",
	"OutboundDirs":    "per-neighbour packet paths on this machine",
	"LeagueNumber":    "which league this board belongs to",
	"IBBS":            "whether this board plays in a league at all",
	"IdleTimeoutSecs": "when to boot a silent caller and free the world lock",
	"MaxIdleWarnings": "how many warnings before that boot",
	"AICount":         "a league board never gets AI barons, so the value is inert there",
	// Not a rule and not a setting: the -dupe-check switch, in force for one
	// run and never saved. Broadcasting it would push one tester's override
	// onto every board in the league.
	"DupeCheckOverride": "the -dupe-check testing switch, this run only",
}

// TestEveryGameRuleIsBroadcast holds the line the rule above draws. A new Config
// field is either in LeagueConfig or listed as per-board with a reason; adding
// one and forgetting both fails here rather than shipping a league whose planets
// quietly play different games. Three rules were found on the wrong side this
// way — idle removal, the money cap and unlimited food.
func TestEveryGameRuleIsBroadcast(t *testing.T) {
	broadcast := map[string]bool{}
	lc := reflect.TypeOf(LeagueConfig{})
	for i := 0; i < lc.NumField(); i++ {
		broadcast[lc.Field(i).Name] = true
	}
	cfg := reflect.TypeOf(Config{})
	for i := 0; i < cfg.NumField(); i++ {
		name := cfg.Field(i).Name
		_, local := perBoardConfigFields[name]
		if !broadcast[name] && !local {
			t.Errorf("Config.%s is neither broadcast to the league nor listed as per-board: "+
				"add it to LeagueConfig (and leagueRuleset/applyLeagueRuleset), or to "+
				"perBoardConfigFields with the reason it is not a game rule", name)
		}
		if broadcast[name] && local {
			t.Errorf("Config.%s is both broadcast and listed as per-board", name)
		}
	}
	// The list must not outlive the fields it describes.
	for name := range perBoardConfigFields {
		if _, ok := cfg.FieldByName(name); !ok {
			t.Errorf("perBoardConfigFields names %s, which Config no longer has", name)
		}
	}
}

// The league's name is the Coordinator's to set, and it must reach the member
// boards: a name only the Coordinator's own board shows is worse than none,
// because each sysop then sees a different one on the same league's Game Setup.
func TestLeagueNameTravelsWithTheRuleset(t *testing.T) {
	co := DefaultConfig()
	co.LeagueName = "Southern Cross"
	member := DefaultConfig()
	member.LeagueName = "whatever this sysop typed"
	member.applyLeagueRuleset(co.leagueRuleset())
	if member.LeagueName != "Southern Cross" {
		t.Errorf("member board calls the league %q, want %q", member.LeagueName, "Southern Cross")
	}
}
