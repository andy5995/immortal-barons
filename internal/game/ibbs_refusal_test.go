package game

import (
	"strings"
	"testing"
)

// A refused order-bearing packet must say WHICH gate it failed. Six situations
// produce the one refusal, they need six different fixes, and three of them are
// on the sending board — so a notice that names none of them sends the reader
// after the wrong one. Each case here asserts a phrase only its own branch
// produces, not merely that some text came back.
func TestCoordRefusalReasonNamesTheGateThatFailed(t *testing.T) {
	roster := []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}
	_, pub := testCoordKeys(t)

	// member is Bravo: not the Coordinator, roster loaded, key recorded.
	member := func() *World {
		cfg := DefaultConfig()
		cfg.BoardID = "BravoBBS"
		w := NewWorldSeed(cfg, 1)
		w.LeagueNodes = roster
		w.CoordPub = pub
		return w
	}

	orders := Packet{FromBoard: "AlphaBBS", FromNode: 1, LeagueNodes: roster}

	t.Run("this board is the Coordinator", func(t *testing.T) {
		cfg := DefaultConfig()
		cfg.BoardID = "AlphaBBS"
		w := NewWorldSeed(cfg, 1)
		w.LeagueNodes = roster
		w.CoordPub = pub
		if got := w.CoordRefusalReason(orders); !strings.Contains(got, "takes orders from no one") {
			t.Errorf("got %q, want the self-as-Coordinator reason", got)
		}
	})

	t.Run("sender is not node 1", func(t *testing.T) {
		p := orders
		p.FromNode = 4
		if got := member().CoordRefusalReason(p); !strings.Contains(got, "node 4") {
			t.Errorf("got %q, want the node number named", got)
		}
	})

	t.Run("no roster", func(t *testing.T) {
		w := member()
		w.LeagueNodes = nil
		p := orders
		p.FromNode = 0
		if got := w.CoordRefusalReason(p); !strings.Contains(got, "names no node 1") {
			t.Errorf("got %q, want the empty-roster reason", got)
		}
	})

	t.Run("name does not match node 1", func(t *testing.T) {
		p := orders
		p.FromNode, p.FromBoard = 0, "ImpostorBBS"
		if got := member().CoordRefusalReason(p); !strings.Contains(got, "AlphaBBS") {
			t.Errorf("got %q, want this board's actual Coordinator named", got)
		}
	})

	// The two that must stay distinct: holding no key is a setup step nobody has
	// done, a signature that will not verify is two boards disagreeing.
	t.Run("no key recorded here", func(t *testing.T) {
		w := member()
		w.CoordPub = nil
		if got := w.CoordRefusalReason(orders); !strings.Contains(got, "no Coordinator key is recorded") {
			t.Errorf("got %q, want the missing-key reason", got)
		}
	})

	t.Run("sender did not sign", func(t *testing.T) {
		if got := member().CoordRefusalReason(orders); !strings.Contains(got, "did not sign") {
			t.Errorf("got %q, want the unsigned reason", got)
		}
	})

	t.Run("signature does not verify", func(t *testing.T) {
		p := orders
		p.Signature = []byte("not a real signature")
		got := member().CoordRefusalReason(p)
		if !strings.Contains(got, "did not match") {
			t.Errorf("got %q, want the mismatch reason", got)
		}
		if strings.Contains(got, "no Coordinator key is recorded") {
			t.Error("a mismatch must not read as a missing key")
		}
	})
}

// The reason has to reach the sysop, not just exist -- and it must NOT reach
// the planet's news, where no player can act on it and the 20-line cap would
// let it delete the day's real events.
func TestRefusalNoticeCarriesTheReason(t *testing.T) {
	cfg := DefaultConfig()
	cfg.BoardID = "BravoBBS"
	w := NewWorldSeed(cfg, 1)
	w.LeagueNodes = []LeagueNode{{Number: 1, Name: "AlphaBBS"}, {Number: 2, Name: "BravoBBS"}}
	w.ApplyPacket(Packet{FromBoard: "AlphaBBS", FromNode: 1, LeagueNodes: w.LeagueNodes})

	var line string
	for _, n := range w.SysopNotices {
		if strings.Contains(n, "was refused") {
			line = n
		}
	}
	if line == "" {
		t.Fatal("no refusal recorded for the sysop")
	}
	if !strings.Contains(line, "no Coordinator key is recorded") {
		t.Errorf("the notice %q does not say why it was refused", line)
	}
	for _, n := range w.NewsToday {
		if strings.Contains(n, "was refused") {
			t.Errorf("a transport fault reached the planet's news: %q", n)
		}
	}
}
