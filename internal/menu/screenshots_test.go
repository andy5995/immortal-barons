package menu

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/andy5995/immortal-barons/internal/session"
)

// The website's Screenshots page is rendered from these captures on every docs
// build (scripts/render-screenshots.sh), so the pictures follow the screens
// rather than going stale in the tree. The test writes nothing unless
// IB_SCREENSHOT_DIR is set; without it, it still proves each screen is reached.

// errScreenDone stops a capture at the read after its last scripted key.
var errScreenDone = errors.New("screen captured")

// screenCapture plays a key script and keeps what was written after the last
// key: the screen those keys led to, up to where it waits for the player.
type screenCapture struct {
	keys []rune
	pos  int
	out  bytes.Buffer
}

func (c *screenCapture) Write(p []byte) (int, error) { return c.out.Write(p) }

func (c *screenCapture) ReadKey() (rune, error) {
	if c.pos >= len(c.keys) {
		panic(errScreenDone)
	}
	c.out.Reset()
	r := c.keys[c.pos]
	c.pos++
	return r, nil
}

// captureScreen runs play against a CP437 session, as a door caller sees it,
// pressing keys, and returns the screen the last key opened.
func captureScreen(t *testing.T, keys string, play func(s session.Session)) []byte {
	t.Helper()
	raw := &screenCapture{keys: []rune(keys)}
	func() {
		defer func() {
			if r := recover(); r != nil && r != errScreenDone {
				panic(r)
			}
		}()
		play(session.NewCP437Writer(raw))
	}()
	out := raw.out.Bytes()
	// The first line is the key's echo on the previous screen's prompt, whose
	// "> " was written before the capture began.
	if keys != "" {
		if i := bytes.IndexByte(out, '\n'); i >= 0 {
			out = out[i+1:]
		}
	}
	return out
}

var sgrRE = regexp.MustCompile(`\x1b\[([0-9;]*)m`)

// xterm256 is the RGB of a 256-color palette index.
func xterm256(n int) (r, g, b int) {
	switch {
	case n < 16:
		base := [16][3]int{{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0}, {0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
			{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0}, {0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255}}
		return base[n][0], base[n][1], base[n][2]
	case n < 232:
		n -= 16
		lv := [6]int{0, 95, 135, 175, 215, 255}
		return lv[n/36], lv[n/6%6], lv[n%6]
	default:
		v := 8 + 10*(n-232)
		return v, v, v
	}
}

// ansiloveCompat rewrites a capture into what ansilove renders faithfully.
// ansilove reads the 1990s ANSI dialect, so three things change: 256-color SGR
// becomes PabloDraw's 24-bit ESC[1;R;G;Bt (foreground) and ESC[0;R;G;Bt
// (background); aixterm bright colors (90-97) become bold + 30-37; and the line
// break after a full 80-column row is removed, since ansilove wraps there by
// itself and ignores the ESC[?7l that stops a terminal from doing the same.
func ansiloveCompat(b []byte) []byte {
	// ansilove has no 22 (bold off), so a normal color after a bright one resets
	// and restores what the reset cleared.
	var bold, reverse bool
	var bg string
	b = sgrRE.ReplaceAllFunc(b, func(m []byte) []byte {
		params := strings.Split(string(sgrRE.FindSubmatch(m)[1]), ";")
		var sgr []string
		var rgb strings.Builder
		for i := 0; i < len(params); i++ {
			p := params[i]
			switch {
			case p == "0" || p == "":
				bold, reverse, bg = false, false, ""
				sgr = append(sgr, "0")
			case p == "7":
				reverse = true
				sgr = append(sgr, p)
			case len(p) == 2 && p[0] == '4' && p[1] >= '0' && p[1] <= '7':
				bg = p
				sgr = append(sgr, p)
			case len(p) == 2 && p[0] == '3' && p[1] >= '0' && p[1] <= '7' && bold:
				bold = false
				sgr = append(sgr, "0")
				if bg != "" {
					sgr = append(sgr, bg)
				}
				if reverse {
					sgr = append(sgr, "7")
				}
				sgr = append(sgr, p)
			case (p == "38" || p == "48") && i+2 < len(params) && params[i+1] == "5":
				n, _ := strconv.Atoi(params[i+2])
				r, g, bl := xterm256(n)
				layer := 0
				if p == "38" {
					layer = 1
				}
				// ansilove reads a packed RGB below 16 as a palette index, so
				// black would paint as index 0's gray.
				if r == 0 && g == 0 && bl < 16 {
					r = 1
				}
				fmt.Fprintf(&rgb, "\x1b[%d;%d;%d;%dt", layer, r, g, bl)
				i += 2
			case len(p) == 2 && p[0] == '9' && p[1] >= '0' && p[1] <= '7':
				bold = true
				sgr = append(sgr, "1", "3"+p[1:])
			default:
				sgr = append(sgr, p)
				// A reset clears ansilove's 24-bit colors, but 49 leaves the
				// background painted.
				if p == "49" {
					bg = ""
					rgb.WriteString("\x1b[0;0;0;0t")
				}
			}
		}
		out := rgb.String()
		if len(sgr) > 0 {
			out = "\x1b[" + strings.Join(sgr, ";") + "m" + out
		}
		return []byte(out)
	})
	b = bytes.ReplaceAll(b, []byte("\x1b[?7l"), nil)
	b = bytes.ReplaceAll(b, []byte("\x1b[?7h"), nil)

	var out bytes.Buffer
	for _, line := range bytes.SplitAfter(b, []byte("\n")) {
		body := bytes.TrimRight(line, "\r\n")
		if len(body) < len(line) && visibleCols(body) == 80 {
			line = body
		}
		out.Write(line)
	}
	return out.Bytes()
}

var escRE = regexp.MustCompile(`\x1b\[[0-9;?]*[A-Za-z]`)

// visibleCols counts the printed cells of a CP437 line: one per byte, escapes excluded.
func visibleCols(line []byte) int { return len(escRE.ReplaceAll(line, nil)) }

// Each screen is reached from the opening menu with the keys a player presses,
// so a changed route fails here instead of photographing the wrong screen.
func TestScreenshots(t *testing.T) {
	opening := func(s session.Session) {
		w := newWorld()
		GameLoop(s, w.World, w.handle, Term{})
	}
	shots := []struct {
		name, keys, marker string
		play               func(s session.Session)
	}{
		{"splash", "", "a BBS door game of conquest and empire", func(s session.Session) { Splash(s) }},
		{"entry", "", "[Entry]", opening},
		// Play, dismiss the income and upkeep reports, leave the bank.
		{"spending", "\r  \r", "[Spending]", opening},
		// The System menu opens from inside a turn, off Spending.
		{"system", "\r  \r*", "[System]", opening},
		{"preferences", "P", "[Preferences]", opening},
		// Help, jump to About.
		{"about", "?A\r", "An independent tribute", opening},
	}
	dir := os.Getenv("IB_SCREENSHOT_DIR")
	for _, sh := range shots {
		got := captureScreen(t, sh.keys, sh.play)
		if !strings.Contains(stripANSI(string(got)), sh.marker) {
			t.Errorf("%s: keys %q did not reach the screen with %q:\n%s", sh.name, sh.keys, sh.marker, got)
			continue
		}
		if dir == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, sh.name+".ans"), ansiloveCompat(got), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
