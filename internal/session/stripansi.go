package session

import "bytes"

// stripEscapes removes ANSI escape sequences from p. A console that cannot
// interpret them prints them verbatim, so where VT output is unavailable — the
// legacy Windows console, which no per-process call can talk round (issue #98)
// — the choice is plain text or a screen peppered with "[94m".
//
// It is stateless: a sequence split across two Write calls would survive. The
// ansi helpers always emit a whole sequence inside one string, so that does not
// arise.
func stripEscapes(p []byte) []byte {
	if !bytes.ContainsRune(p, 0x1b) {
		return p
	}
	out := make([]byte, 0, len(p))
	for i := 0; i < len(p); i++ {
		if p[i] != 0x1b {
			out = append(out, p[i])
			continue
		}
		// Skip to the sequence's final byte; the loop's own i++ steps over it.
		// CSI (ESC [) takes parameter and intermediate bytes up to 0x3f, a plain
		// two- or three-byte escape only intermediates up to 0x2f.
		i++
		last := byte(0x2f)
		if i < len(p) && p[i] == '[' {
			i++
			last = 0x3f
		}
		for i < len(p) && p[i] >= 0x20 && p[i] <= last {
			i++
		}
	}
	return out
}
