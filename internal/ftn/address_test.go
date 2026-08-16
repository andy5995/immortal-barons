package ftn

import "testing"

func TestParseAddress(t *testing.T) {
	tests := []struct {
		text string
		want Address
	}{
		{"1:229/100", Address{Zone: 1, Net: 229, Node: 100}},
		{"2:500/10.4", Address{Zone: 2, Net: 500, Node: 10, Point: 4}},
	}
	for _, tt := range tests {
		got, err := ParseAddress(tt.text)
		if err != nil {
			t.Fatalf("ParseAddress(%q): %v", tt.text, err)
		}
		if got != tt.want {
			t.Errorf("ParseAddress(%q) = %+v, want %+v", tt.text, got, tt.want)
		}
	}
}

func TestParseAddressRequiresZone(t *testing.T) {
	if _, err := ParseAddress("229/100"); err == nil {
		t.Fatal("ParseAddress accepted an address with no zone")
	}
}
