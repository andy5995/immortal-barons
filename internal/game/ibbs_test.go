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

	if err := wA.SendTerror(attacker, "boardB", "Victim", 4, TerrorOpDissensions); err != nil {
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
	if !res.Won || res.Kind != "terror" || res.Report == "" {
		t.Errorf("expected a won terror result carrying its own report, got %+v", res)
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

	if err := wA.SendTerror(attacker, "boardB", "Victim", 4, TerrorOpBombIntel); err != nil {
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

	// The target is defenceless, so there is no battle to bleed in and the whole
	// detachment comes home. A flat retreat share used to take 15% from a force
	// nobody fired on (#199).
	if leader.Troopers != 100_000 || leader.Tanks != 1000 {
		t.Errorf("survivors returned wrong: have %d troopers, %d tanks (want 100000, 1000 -- nothing opposed them)", leader.Troopers, leader.Tanks)
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
	if err := w.SendTerror(e, "faraway", "Rome", 5, TerrorOpDemoralize); err != nil {
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

// Intelligence on another planet's realms is a by-product of acting against
// them: a covert operation landing there answers with the target's figures as
// they stand, and that answer fills the sender's Spy Database. BRE does the
// same — resolve_received_covert_operation calls write_spy_report, and the
// sender files it under "Information added to Global Spy Data Bank". IB had an
// errand of its own for this, a per-baron Send Recon that spent an agent and
// scouted without touching anyone; that was IB's invention and is gone.
func TestACovertOpBringsBackIntelligence(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "Wildside"
	asker := NewWorldSeed(cfg, 1)
	scout := asker.AddHuman("alice", "Alethia")
	scout.Agents, scout.Gold, scout.Protection = 5, 10_000_000, 0

	if err := asker.SendTerror(scout, "faraway", "Rome", 2, TerrorOpSpy); err != nil {
		t.Fatalf("SendTerror: %v", err)
	}

	// The far board resolves it against its own state and answers from there.
	farCfg := DefaultConfig()
	farCfg.IBBS = true
	farCfg.BoardID = "faraway"
	far := NewWorldSeed(farCfg, 1)
	rome := far.AddHuman("bob", "Rome")
	rome.Protection = 0
	rome.Land, rome.Gold, rome.Troopers = 4321, 99000, 5000
	reply := far.ApplyPacket(asker.Outbox[0])
	if len(reply.ReconReports) != 1 {
		t.Fatalf("the far board sent %d reports, want 1", len(reply.ReconReports))
	}
	if got := reply.ReconReports[0]; got.Land != 4321 || got.Gold != 99000 || got.Offense == 0 {
		t.Errorf("report does not carry the target's real figures: %+v", got)
	}

	// And the answer files itself on the sending board.
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
	w.GameDay += AnnihilatorBuildDays
	w.LaunchDueAnnihilator()
	if w.Annihilator.Launched {
		t.Error("an unfunded weapon launched")
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

	// Nobody launches it. Construction runs for AnnihilatorBuildDays and then it
	// goes by itself (#114).
	if w.Annihilator.LaunchDay != w.GameDay+AnnihilatorBuildDays {
		t.Errorf("launch day %d, want %d", w.Annihilator.LaunchDay, w.GameDay+AnnihilatorBuildDays)
	}
	w.LaunchDueAnnihilator()
	if w.Annihilator.Launched {
		t.Error("the weapon launched before construction finished")
	}
	w.GameDay += AnnihilatorBuildDays
	w.LaunchDueAnnihilator()
	if !w.Annihilator.Launched {
		t.Fatal("a finished weapon did not launch itself")
	}
	if w.Annihilator.ArrivesDay != w.GameDay+AnnihilatorFlightDays {
		t.Errorf("arrives day %d, want %d", w.Annihilator.ArrivesDay, w.GameDay+AnnihilatorFlightDays)
	}
	// Not even the Coordinator can call back a weapon already in the air.
	if err := w.DismantleAnnihilatorByCoordinator(founder); err == nil {
		t.Error("a weapon in flight was dismantled")
	}

	// Once it has landed on the target, the planet is free to build another.
	w.RetireSpentAnnihilator()
	if w.Annihilator == nil {
		t.Error("the weapon was retired before it arrived")
	}
	w.GameDay = w.Annihilator.ArrivesDay + 1
	w.RetireSpentAnnihilator()
	if w.Annihilator != nil {
		t.Errorf("a spent weapon still occupies the planet: %+v", w.Annihilator)
	}
}

// The target planet is told the weapon exists while it is still being built and
// again when it launches; nothing can touch it until it lands, and then jets and
// only jets can kill it (#63, #16, #112).
func TestAnnihilatorIsVisibleAndCanBeShotDown(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	cfg.BoardID = "faraway"
	target := NewWorldSeed(cfg, 1)
	defender := target.AddHuman("bob", "Rome")
	defender.Regions = RegionMix{Agricultural: 9000}
	defender.syncLand()
	defender.Protection = 0
	defender.Jets = 400_000

	// Under construction: a warning, and nothing to shoot at yet.
	target.ApplyPacket(Packet{FromBoard: "Wildside", Annihilator: &AnnihilatorStatus{FromBoard: "Wildside"}})
	if target.Incoming == nil {
		t.Fatal("target was not told about the weapon being built")
	}
	if target.Incoming.Launched {
		t.Error("a weapon still under construction is marked as flying")
	}

	// Launched: in the air, and out of reach until it lands.
	target.ApplyPacket(Packet{FromBoard: "Wildside", Annihilator: &AnnihilatorStatus{
		FromBoard: "Wildside", Funded: true, Launched: true, ArrivesDay: target.GameDay + 2, Intact: 100,
	}})
	if !target.Incoming.Launched {
		t.Fatal("target does not know the weapon has launched")
	}
	if _, _, err := target.InterceptAnnihilator(defender, 1000); err != ErrAnnihilatorAloft {
		t.Errorf("jets reached a weapon still in flight: %v", err)
	}

	// It lands. Now the jets have something to fight.
	target.GameDay += 2
	target.ArriveAnnihilator()
	if target.Incoming.DaysLeft != AnnihilatorSiegeDays {
		t.Fatalf("siege countdown is %d, want %d", target.Incoming.DaysLeft, AnnihilatorSiegeDays)
	}
	needed := target.AnnihilatorJetsNeeded()
	if want := int64(9000 * (9000/AnnihilatorJetsLandDivisor + AnnihilatorJetsLandBase)); needed != want {
		t.Errorf("%d jets needed, want %d", needed, want)
	}

	// No single sortie can finish it, however many jets it carries.
	knocked, lost, err := target.InterceptAnnihilator(defender, 400_000)
	if err != nil {
		t.Fatalf("InterceptAnnihilator: %v", err)
	}
	if knocked != AnnihilatorMaxSortiePct {
		t.Errorf("one sortie knocked %d%% off, want the %d%% ceiling", knocked, AnnihilatorMaxSortiePct)
	}
	if target.Incoming == nil {
		t.Fatal("one sortie destroyed the weapon; the ceiling must forbid that")
	}
	if lost < 1 || lost >= 400_000 {
		t.Errorf("%d of 400,000 jets lost, want about a third", lost)
	}
	if defender.Jets != 400_000-lost {
		t.Errorf("jets not spent: %d, want %d", defender.Jets, 400_000-lost)
	}

	// A second wave finishes it, and then the siege is over.
	defender.Jets = 400_000
	if _, _, err := target.InterceptAnnihilator(defender, 400_000); err != nil {
		t.Fatalf("InterceptAnnihilator: %v", err)
	}
	if target.Incoming != nil {
		t.Fatalf("weapon survived a second full wave: %+v", target.Incoming)
	}
	before := defender.Land
	target.TickAnnihilator()
	if defender.Land != before {
		t.Errorf("a destroyed weapon still bit: land %d, was %d", defender.Land, before)
	}

	// The builder announcing it once more must not raise a second siege.
	target.ApplyPacket(Packet{FromBoard: "Wildside", Annihilator: &AnnihilatorStatus{
		FromBoard: "Wildside", Funded: true, Launched: true, ArrivesDay: target.GameDay - 1, Intact: 100,
	}})
	if target.Incoming != nil {
		t.Errorf("a spent weapon came back: %+v", target.Incoming)
	}
}

// The weapon is a siege: a tenth of every realm's regions the day it lands and a
// twentieth on each of the four days after, then it burns out. The SDI is no
// defence — only jets can reach it (#111, #112) — and a realm still under
// new-realm protection is passed over.
func TestAnnihilatorBesiegesThePlanetForFiveDays(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IBBS = true
	w := NewWorldSeed(cfg, 1)
	plain := w.AddHuman("bob", "Rome")
	shielded := w.AddHuman("carol", "Carthage")
	newcomer := w.AddHuman("dave", "Dacia")
	for _, e := range []*Empire{plain, shielded, newcomer} {
		e.Regions = RegionMix{Agricultural: 10_000}
		e.syncLand()
		e.Protection = 0
	}
	shielded.SDI = SDIMax
	newcomer.Protection = 5

	w.Incoming = &Annihilator{Creator: "Wildside", Launched: true, ArrivesDay: w.GameDay, Intact: 100}
	w.ArriveAnnihilator()

	// Day one: a tenth, shield or no shield.
	w.TickAnnihilator()
	for _, e := range []*Empire{plain, shielded} {
		if e.Land != 9000 {
			t.Errorf("%s has %d land after the first day, want 9,000", e.Name, e.Land)
		}
	}
	if newcomer.Land != 10_000 {
		t.Errorf("a protected realm lost land: %d, want 10,000", newcomer.Land)
	}

	// Four more days at a twentieth of what is left, then it burns out.
	want := 9000
	for day := 2; day <= AnnihilatorSiegeDays; day++ {
		w.TickAnnihilator()
		want -= want * AnnihilatorLaterDayPct / 100
		if plain.Land != want {
			t.Errorf("day %d: %d land, want %d", day, plain.Land, want)
		}
	}
	if w.Incoming != nil {
		t.Fatalf("the weapon outlasted its %d days: %+v", AnnihilatorSiegeDays, w.Incoming)
	}
	// A battered weapon bites just as deep as a fresh one, so five days of it
	// costs a realm better than a quarter of its land.
	if plain.Land > 7400 {
		t.Errorf("five days of siege left %d land, want under 7,400", plain.Land)
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
	// The one entry here that IS a rule, and it sits here on the original's
	// authority: BRE keeps its lottery switch in the per-install RESOURCE.DAT,
	// not the game data, so two boards in one league may differ. Noted rather
	// than hidden — a league whose boards disagree about the lottery is a league
	// where one planet has a faucet the other does not.
	"Lottery": "each sysop's call in the original, per-install and never broadcast",
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

// A Coordinator-set string reaches the member boards. Tested on the required
// version, which is the kind that matters: a floor only the Coordinator's own
// board honours is worse than none, because each sysop then enforces a
// different one on the same league.
func TestCoordinatorStringsTravelWithTheRuleset(t *testing.T) {
	co := DefaultConfig()
	co.MinBoardVersion = "0.0.7"
	member := DefaultConfig()
	member.MinBoardVersion = "whatever this sysop typed"
	member.applyLeagueRuleset(co.leagueRuleset())
	if member.MinBoardVersion != "0.0.7" {
		t.Errorf("member board requires %q, want %q", member.MinBoardVersion, "0.0.7")
	}
}

// Each of the nine interplanetary terrorist operations does its own thing to
// its own field (#166), which the received-op resolver dispatches on
// (BRE.OVR 0x04a96b). Before this they all destroyed random units.
func TestEachTerrorOpHitsItsOwnField(t *testing.T) {
	stock := func(e *Empire) {
		e.Protection = 0
		e.Agents, e.Troopers, e.Jets, e.People, e.Food = 500, 5_000, 700, 900, 400_000
		e.Morale, e.Support, e.HQ = 100, 100, 90
	}
	cases := []struct {
		op    TerrorOpType
		field func(*Empire) int
	}{
		{TerrorOpBombIntel, func(e *Empire) int { return e.Agents }},
		{TerrorOpDemoralize, func(e *Empire) int { return e.Morale }},
		{TerrorOpDissensions, func(e *Empire) int { return e.Troopers }},
		{TerrorOpBombAirBases, func(e *Empire) int { return e.Jets }},
		{TerrorOpEmigrations, func(e *Empire) int { return e.People }},
		{TerrorOpPropaganda, func(e *Empire) int { return e.Support }},
		{TerrorOpBombFood, func(e *Empire) int { return e.Food }},
		{TerrorOpSabotageHQ, func(e *Empire) int { return e.HQ }},
	}
	for _, c := range cases {
		t.Run(c.op.String(), func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.BoardID = "boardB"
			w := NewWorldSeed(cfg, 3)
			target := w.AddHuman("victim", "Victim")
			stock(target)
			before := c.field(target)

			w.resolveRemoteTerror(RemoteTerror{
				ID: 1, FromBoard: "boardA", TargetEmpire: "Victim",
				Agents: 3, Op: c.op, Strength: 1_000_000,
			})

			if got := c.field(target); got >= before {
				t.Errorf("%s should have cost the target: %d -> %d", c.op, before, got)
			}
			// Nothing else moves: each operation owns one field.
			for _, other := range cases {
				if other.op == c.op {
					continue
				}
				fresh := &Empire{}
				stock(fresh)
				if other.field(target) != other.field(fresh) {
					t.Errorf("%s also changed what %s aims at", c.op, other.op)
				}
			}
		})
	}
}

// Send Spy takes nothing from the target: it brings intelligence home instead.
func TestTerrorSpyTakesNothing(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "boardB"
	w := NewWorldSeed(cfg, 3)
	target := w.AddHuman("victim", "Victim")
	target.Protection = 0
	target.Troopers, target.Agents, target.Morale = 5_000, 500, 100

	res := w.resolveRemoteTerror(RemoteTerror{
		ID: 1, FromBoard: "boardA", TargetEmpire: "Victim",
		Agents: 2, Op: TerrorOpSpy, Strength: 1_000_000,
	})

	if target.Troopers != 5_000 || target.Agents != 500 || target.Morale != 100 {
		t.Errorf("a spy should cost the target nothing, got %+v", target)
	}
	if !res.Won || res.Report == "" {
		t.Errorf("the spy should come home with something to say, got %+v", res)
	}
}

// The odds are BRE's: the root of each side, the defender given half again, and
// the two flat coins on top (calculate_combat_odds, BRE.OVR 0x04a7a9). A far
// stronger sender lands most agents; a far weaker one lands few.
func TestTerrorAgentOddsFavourTheStrongerCovertPool(t *testing.T) {
	w := NewWorldSeed(DefaultConfig(), 7)
	count := func(attack, defense int) int {
		landed := 0
		for i := 0; i < 1000; i++ {
			if w.terrorAgentLands(attack, defense) {
				landed++
			}
		}
		return landed
	}
	strong := count(10_000, 100)
	weak := count(100, 10_000)
	if strong < 800 {
		t.Errorf("a hundredfold pool should land most agents, got %d/1000", strong)
	}
	if weak > 200 {
		t.Errorf("a hundredth of the pool should land few agents, got %d/1000", weak)
	}
	// Even sides: the defender's extra half puts it under a coin flip.
	if even := count(1_000, 1_000); even < 300 || even > 500 {
		t.Errorf("evenly matched pools should land ~40%%, got %d/1000", even)
	}
}

// A terror op that finds no such realm, or one under protection, comes home
// saying so and naming the operation (#165) — not as the "achieved nothing" a
// repelled one gets.
func TestTerrorReturnReportSaysWhyNothingHappened(t *testing.T) {
	sent := InFlightStrike{Kind: "terror", TargetBoard: "boardB", TargetEmpire: "Victim", TerrorOp: TerrorOpBombAirBases}
	cases := []struct {
		outcome AttackOutcome
		want    string
	}{
		{OutcomeNotFound, "no realm called"},
		{OutcomeProtected, "New Realm Protection"},
		{OutcomeRepelled, "turned away"},
	}
	for _, c := range cases {
		got := terrorReturnReport(sent, AttackResult{TargetBoard: "boardB", TargetEmpire: "Victim", Outcome: c.outcome})
		if !strings.Contains(got, c.want) {
			t.Errorf("%s report = %q, want it to mention %q", c.outcome, got, c.want)
		}
		if !strings.Contains(got, TerrorOpBombAirBases.String()) {
			t.Errorf("%s report should name the operation: %q", c.outcome, got)
		}
	}
}

// A group attack aimed at the PLANET fights every living realm at once, not the
// strongest one. BRE marks that target with the letter Z and, on seeing it,
// loops A..Y summing each realm's defence and pooling their jets before a single
// battle (resolve_received_invasion +0x0d3d). IB sent the whole strike against
// the biggest baron and left everyone else untouched.
func TestPlanetWideStrikeFightsEveryRealm(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "here"
	w := NewWorldSeed(cfg, 5)
	big := w.AddHuman("a", "Big")
	small := w.AddHuman("b", "Small")
	shielded := w.AddHuman("c", "Shielded")
	for _, e := range []*Empire{big, small, shielded} {
		e.Protection = 0
		e.Troopers, e.Turrets, e.Tanks, e.Jets = 20_000, 20_000, 5_000, 4_000
		e.Regions = RegionMix{Desert: 4_000}
		e.syncLand()
	}
	big.Troopers *= 4 // make one clearly the strongest
	shielded.Protection = 5

	// Every realm in the pool must be measured, so the defence a planet-wide
	// strike meets is strictly greater than the strongest realm's alone.
	if got, want := len(w.planetDefenders()), 2; got != want {
		t.Fatalf("planetDefenders = %d, want %d (the protected realm is out)", got, want)
	}
	force := AttackForce{Troopers: 400_000, Tanks: 100_000, Bombers: 2_000}
	res := w.resolveRemoteAttack(RemoteAttack{
		ID: 1, FromBoard: "far", TargetEmpire: "", Group: true,
		Offense:      force.offense(),
		Contributors: []Contribution{{Owner: "x", AttackForce: force}},
	})
	if !res.Won {
		t.Fatalf("the strike did not land: %q", res.Outcome)
	}
	// Both unprotected realms bled and lost ground; the protected one did not.
	if small.Land == 4_000 {
		t.Error("the smaller realm was untouched; the strike hit only the strongest")
	}
	if big.Land == 4_000 {
		t.Error("the strongest realm was untouched")
	}
	if shielded.Land != 4_000 || shielded.Turrets != 20_000 {
		t.Errorf("a realm under New Realm Protection was drawn into the battle: %d land, %d turrets",
			shielded.Land, shielded.Turrets)
	}
	if res.LandTaken != (4_000-big.Land)+(4_000-small.Land) {
		t.Errorf("LandTaken %d does not match what the realms actually lost", res.LandTaken)
	}
}
