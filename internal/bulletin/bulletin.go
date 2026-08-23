// Package bulletin reads the sysop's bulletin files off disk and turns them
// into something a session can display.
//
// Bulletins live under the data directory in bull/, split by who owns them:
//
//	bull/league — the League Coordinator's, distributed to every board
//	bull/local  — this board's own, whatever its sysop puts there
//
// A bulletin is a plain .txt or an ANSI .ans file, and its TITLE is the first
// line of the file (escapes stripped), so a sysop adds one by copying a file
// in — there is no index to edit. The original kept every bulletin in a single
// bulletin.lst with ^NAME markers; separate files are IB's own arrangement,
// because a league's bulletins have to travel as files anyway.
//
// The package deliberately knows nothing about the game world or the league
// transport: internal/store does the syncing, internal/menu the drawing.
package bulletin

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/andy5995/immortal-barons/internal/screen"
)

// Scope is which of the two bulletin directories a bulletin came from.
type Scope string

const (
	// League bulletins are the Coordinator's, shown under the "Galactic"
	// heading and replaced wholesale by whatever the league sends.
	League Scope = "league"
	// Local bulletins are the board's own, listed after the league's.
	Local Scope = "local"
)

// Root is the bulletin directory under a data directory.
const Root = "bull"

// MaxSize caps one bulletin file. A league bulletin is copied into a packet and
// carried to every board on every planetary run, so an oversized file is refused
// at the source rather than pushed through the transport.
const MaxSize = 64 * 1024

// Dir is the on-disk directory for one scope.
func Dir(dataDir string, scope Scope) string {
	return filepath.Join(dataDir, Root, string(scope))
}

// Bulletin is one file, ready to list.
type Bulletin struct {
	Scope Scope
	Name  string // file name within the scope's directory
	Path  string
	Title string // first line of the file, or the file name if that line is blank
}

// List returns a scope's bulletins in file-name order. A missing directory is
// not an error: a board with no bulletins has simply never made one.
func List(dataDir string, scope Scope) ([]Bulletin, error) {
	dir := Dir(dataDir, scope)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Bulletin
	for _, e := range entries {
		if e.IsDir() || !SafeName(e.Name()) {
			continue
		}
		path := filepath.Join(dir, e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		out = append(out, Bulletin{
			Scope: scope,
			Name:  e.Name(),
			Path:  path,
			Title: Title(data, e.Name()),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// Text decodes a bulletin for display. ANSI artwork is authored in CP437, so
// the bytes are decoded to the engine's internal UTF-8 and the session's own
// encoder puts them back on the wire in whatever the caller's terminal speaks.
// A file that is already valid UTF-8 with a character above ASCII is taken at
// its word instead — a sysop writing plain text on a modern editor gets what
// they typed.
func Text(data []byte) string {
	if isUTF8Text(data) {
		return string(data)
	}
	return screen.FromCP437(data)
}

// Title reads a bulletin's title off its first line, with any ANSI escapes and
// CP437 art bytes removed. A first line that carries no letters or digits (a
// row of block characters, an empty line) leaves the file name to name it,
// since a menu entry has to say something.
func Title(data []byte, fallback string) string {
	line, _, _ := strings.Cut(Text(data), "\n")
	title := strings.TrimSpace(stripANSI(line))
	if !hasWord(title) {
		return strings.TrimSuffix(fallback, filepath.Ext(fallback))
	}
	return title
}

// SafeName reports whether a file name may be listed or written as a bulletin.
// The extension check keeps a sysop's stray notes and editor backups out of the
// menu; the rest of it matters because a league bulletin's name arrives from
// another board, and a name is used to build a path (#118's lesson: a packet
// field is a claim, not a fact).
func SafeName(name string) bool {
	if name == "" || name != filepath.Base(name) || strings.HasPrefix(name, ".") {
		return false
	}
	if strings.ContainsAny(name, `/\`) || name == "." || name == ".." {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".txt", ".ans":
	default:
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// Digest fingerprints a bulletin's contents, so a board can tell an unchanged
// file from an edited one without keeping a copy of it.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// Write saves a bulletin into a scope's directory, creating the directory when
// it does not exist yet. The write is atomic: a reader either sees the previous
// version or the new one, never a half-copied file.
func Write(dataDir string, scope Scope, name string, data []byte) error {
	if !SafeName(name) {
		return os.ErrInvalid
	}
	dir := Dir(dataDir, scope)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+name+"-*.tmp")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	target := filepath.Join(dir, name)
	// Windows will not rename onto an existing file.
	os.Remove(target)
	return os.Rename(tmpPath, target)
}

// Remove deletes one bulletin from a scope's directory.
func Remove(dataDir string, scope Scope, name string) error {
	if !SafeName(name) {
		return os.ErrInvalid
	}
	err := os.Remove(filepath.Join(dataDir, Root, string(scope), name))
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// isUTF8Text reports whether data is valid UTF-8 carrying at least one
// character above ASCII. Pure ASCII is left to the CP437 path, where it decodes
// to itself, so the two agree on everything a plain text file contains.
func isUTF8Text(data []byte) bool {
	if !utf8.Valid(data) {
		return false
	}
	for _, b := range data {
		if b >= 0x80 {
			return true
		}
	}
	return false
}

// stripANSI removes CSI escape sequences from a line, so a title authored in
// colour lists as its text.
func stripANSI(line string) string {
	var sb strings.Builder
	runes := []rune(line)
	for i := 0; i < len(runes); i++ {
		if runes[i] != 0x1b {
			if runes[i] != '\r' {
				sb.WriteRune(runes[i])
			}
			continue
		}
		i++
		if i < len(runes) && runes[i] == '[' {
			// A CSI sequence runs to its first byte in 0x40-0x7e.
			for i++; i < len(runes) && (runes[i] < 0x40 || runes[i] > 0x7e); i++ {
			}
		}
	}
	return sb.String()
}

// hasWord reports whether a candidate title carries anything readable.
func hasWord(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return true
		}
	}
	return false
}
