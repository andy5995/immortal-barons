package screen

import "testing"

func TestFromCP437MapsHighBytesAndKeepsEscapes(t *testing.T) {
	// 0xDB is the full block and 0xB0 the light shade; the ESC colour sequence,
	// being all ASCII, has to survive byte for byte.
	out := FromCP437([]byte{0x1b, '[', '3', '6', 'm', 0xDB, 0xB0})
	if out != "\x1b[36m█░" {
		t.Errorf("got %q", out)
	}
}
