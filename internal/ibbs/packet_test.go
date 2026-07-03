package ibbs

import (
	"path/filepath"
	"testing"
)

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "packet.json")
	want := Packet{
		BoardID: "wildside",
		Date:    "2026-07-03",
		Scores: []Score{
			{Empire: "Iron Dominion", NetWorth: 50000, Land: 120},
			{Empire: "Ashfall Clan", NetWorth: 30000, Land: 80},
		},
	}

	if err := Write(path, want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read(path)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if got.BoardID != want.BoardID || got.Date != want.Date {
		t.Fatalf("got %+v, want %+v", got, want)
	}
	if len(got.Scores) != len(want.Scores) {
		t.Fatalf("got %d scores, want %d", len(got.Scores), len(want.Scores))
	}
	for i := range want.Scores {
		if got.Scores[i] != want.Scores[i] {
			t.Errorf("score %d: got %+v, want %+v", i, got.Scores[i], want.Scores[i])
		}
	}
}

func TestReadMissingFile(t *testing.T) {
	_, err := Read(filepath.Join(t.TempDir(), "nope.json"))
	if err == nil {
		t.Fatal("expected error reading missing file, got nil")
	}
}
