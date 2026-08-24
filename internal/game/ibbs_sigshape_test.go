package game

import (
	"encoding/json"
	"testing"
)

// signingBytes renders the payload by hand rather than marshalling a struct,
// which puts every escaping rule in our own code. A difference in any of them
// invalidates every signature in the league silently, so this compares the
// renderer against encoding/json itself on the cases where hand-rolling goes
// wrong: HTML escaping, non-ASCII, nil versus empty slices, nested bytes.
//
// The shadow struct is deliberately a separate definition. Sharing one with the
// production code would move both sides of the comparison together and prove
// nothing.
type shadowPayload struct {
	FromBoard    string
	Seq          uint64
	LeagueConfig *LeagueConfig
	LeagueNodes  []LeagueNode
	Reset        *LeagueReset
	Bulletins    *BulletinSet
}

func TestSigningBytesMatchesEncodingJSON(t *testing.T) {
	cases := []Packet{
		{FromBoard: "Alpha", Seq: 1},
		{FromBoard: "", Seq: 0},
		{FromBoard: `A "quoted" & <html> board`, Seq: 99},
		{FromBoard: "Ünïcøde ✈ 日本", Seq: 1 << 62},
		{FromBoard: "Alpha", LeagueNodes: []LeagueNode{}},
		{FromBoard: "Alpha", LeagueNodes: []LeagueNode{{Number: 1, Name: "A<B>"}}},
		{FromBoard: "Alpha", LeagueConfig: &LeagueConfig{}},
		{FromBoard: "Alpha", Reset: &LeagueReset{Season: 3}},
		{FromBoard: "Alpha", Bulletins: &BulletinSet{}},
		{FromBoard: "Alpha", Bulletins: &BulletinSet{Files: []BulletinFile{{Name: "n", Data: []byte{0, 1, 255}}}}},
	}
	for i, p := range cases {
		want, err := json.Marshal(shadowPayload{p.FromBoard, p.Seq, p.LeagueConfig, p.LeagueNodes, p.Reset, p.Bulletins})
		if err != nil {
			t.Fatal(err)
		}
		got, err := signingBytes(p, 6)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != string(want) {
			t.Errorf("case %d differs:\n got  %s\n want %s", i, got, want)
		}
	}
}
