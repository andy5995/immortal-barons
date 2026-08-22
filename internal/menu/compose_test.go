package menu

import (
	"strings"
	"testing"
)

// breRuler is BRE's own message-editor ruler, read out of the overlay's string
// table and seen on screen in a live session (docs/dev/bre-screens.md). It is
// written out in full rather than built from msgLineWidth so that a change to
// the width has to come with new evidence, and so the ruler can never quietly
// stop matching the column the editor wraps at.
const breRuler = "[---+----|----+----|----+----|----+----|----+----|----+----|----+----]"

func TestMessageRulerMatchesBRE(t *testing.T) {
	if got := msgRuler(msgLineWidth); got != breRuler {
		t.Errorf("ruler =\n%q\nwant\n%q", got, breRuler)
	}
	if n := len(breRuler) - 2; n != 68 {
		t.Errorf("ruler spans %d columns, want 68", n)
	}
}

// The message editor advertises /S=save /A=abort /C=clear in its header, so a
// line that is exactly one of those commands — either case — must act on it
// (the reported bug was that only a bare "/" did anything, so "/S" got added
// as text). One table covers the command set; the upper/lowercase rows are the
// case-folding check.
func TestComposeMessageCommands(t *testing.T) {
	cases := []struct {
		name     string
		keys     string
		wantSend bool
		wantText string // checked only when wantSend
	}{
		{"save /S", "hello\rworld\r/S", true, "hello\nworld"},
		{"save /s lowercase", "Hello\r/s\r", true, "Hello"},
		{"abort /A", "secret\r/A", false, ""},
		{"abort /a lowercase", "Hi\r/a\r", false, ""},
		{"clear /C then save", "oops\r/Ckeep\r/S", true, "keep"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeSession{keys: []rune(c.keys)}
			text, send := composeMessage(f)
			if send != c.wantSend {
				t.Fatalf("send = %v, want %v (text %q)", send, c.wantSend, text)
			}
			if c.wantSend && text != c.wantText {
				t.Errorf("text = %q, want %q", text, c.wantText)
			}
		})
	}
}

func TestComposeMessageTextEndingInSlashIsNotACommand(t *testing.T) {
	// "hello/s" is ordinary text, not the save command.
	f := &fakeSession{keys: []rune("hello/s\r/S\r")}
	msg, ok := composeMessage(f)
	if !ok {
		t.Fatal("expected save on the second line")
	}
	if msg != "hello/s" {
		t.Fatalf("msg = %q, want %q", msg, "hello/s")
	}
}

// The wrap cases below assert the composed message's real line breaks, and each
// first checks the ruler reached the screen — when a scripted key sequence runs
// dry the session ends cleanly, so without that marker a flow change upstream
// would leave these green while never entering the editor.
//
// The expected splits are BRE's, observed live on 2026-08-14: a line takes 68
// characters and the 69th wraps.
func TestComposeMessageWrapsAtASpace(t *testing.T) {
	// "kkkkk" ends at column 65 and "lllll" starts at 67, so the wrap fires on
	// the third 'l' and carries the whole word down.
	typed := "aaaaa bbbbb ccccc ddddd eeeee fffff ggggg hhhhh iiiii jjjjj kkkkk lllll"
	f := &fakeSession{keys: []rune(typed + "\r/S")}
	text, send := composeMessage(f)
	if !send {
		t.Fatal("editor did not save")
	}
	if !strings.Contains(f.out.String(), breRuler) {
		t.Fatal("never reached the editor: the ruler was not drawn")
	}
	want := "aaaaa bbbbb ccccc ddddd eeeee fffff ggggg hhhhh iiiii jjjjj kkkkk\nlllll"
	if text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
	if first := strings.SplitN(text, "\n", 2)[0]; len(first) != 65 {
		t.Errorf("first line is %d columns, want 65 (the break drops the space)", len(first))
	}
	// The carried word and the space before it were echoed on the line being
	// left, so three characters must be erased from it before the newline.
	if !strings.Contains(f.out.String(), strings.Repeat("\b \b", 3)) {
		t.Error("the carried word was not erased from the line it came from")
	}
}

func TestComposeMessageSplitsAnOverlongWord(t *testing.T) {
	// A word with no space in reach of the break search has nowhere to break, so
	// it is split at the margin: 68 characters stay, the rest starts a new line.
	typed := strings.Repeat("0123456789", 7) + "012345" // 76 characters
	f := &fakeSession{keys: []rune(typed + "\r/S")}
	text, send := composeMessage(f)
	if !send {
		t.Fatal("editor did not save")
	}
	if !strings.Contains(f.out.String(), breRuler) {
		t.Fatal("never reached the editor: the ruler was not drawn")
	}
	want := strings.Repeat("0123456789", 6) + "01234567" + "\n" + "89012345"
	if text != want {
		t.Errorf("text = %q, want %q", text, want)
	}
	if first := strings.SplitN(text, "\n", 2)[0]; len(first) != 68 {
		t.Errorf("first line is %d columns, want 68", len(first))
	}
	// Nothing is carried, so nothing may be erased. BRE erases one cell here
	// anyway and its screen ends up a character short of what it saves; that is
	// a display fault, not behaviour to copy.
	if strings.Contains(f.out.String(), "\b \b") {
		t.Error("a margin split erased characters that were not carried")
	}
}

func TestComposeMessageDoesNotWrapPastTheLastLine(t *testing.T) {
	// Nineteen empty lines put the cursor on line 20, where there is nowhere to
	// wrap to. The line must stop taking keys at the margin rather than open a
	// twenty-first. (BRE opens one, then drops it when it saves.)
	typed := strings.Repeat("\r", msgMaxLines-1) + strings.Repeat("0123456789", 7) + "012345\r"
	f := &fakeSession{keys: []rune(typed)}
	text, send := composeMessage(f)
	if !send {
		t.Fatal("editor did not save when the lines ran out")
	}
	if !strings.Contains(f.out.String(), breRuler) {
		t.Fatal("never reached the editor: the ruler was not drawn")
	}
	got := strings.Split(text, "\n")
	if len(got) != 20 {
		t.Fatalf("message has %d lines, want 20", len(got))
	}
	if want := strings.Repeat("0123456789", 6) + "01234567"; got[19] != want {
		t.Errorf("last line = %q, want %q", got[19], want)
	}
	if strings.Contains(f.out.String(), "21>") {
		t.Error("a 21st line prompt was drawn")
	}
}

func TestComposeMessageLeavesQuotedLinesAlone(t *testing.T) {
	// A reply's quoted lines are placed in the editor, not typed into it, so an
	// over-long one is echoed as it stands — the wrap only ever fires on a key.
	quoted := "> " + strings.Repeat("x", 90)
	f := &fakeSession{keys: []rune("/S")}
	text, send := composeMessageFrom(f, []string{quoted})
	if !send {
		t.Fatal("editor did not save")
	}
	if !strings.Contains(f.out.String(), breRuler) {
		t.Fatal("never reached the editor: the ruler was not drawn")
	}
	if text != quoted {
		t.Errorf("text = %q, want the quoted line unchanged (%q)", text, quoted)
	}
}

func TestComposeMessageBackspaceReopensThePreviousLine(t *testing.T) {
	// Backspace at column 1 takes the line above back out of the message and
	// puts the cursor at its end, which is how an unwanted wrap is undone.
	f := &fakeSession{keys: []rune("aaa\rbbb\b\b\b\bX\r/S")}
	text, send := composeMessage(f)
	if !send {
		t.Fatal("editor did not save")
	}
	if !strings.Contains(f.out.String(), breRuler) {
		t.Fatal("never reached the editor: the ruler was not drawn")
	}
	if text != "aaaX" {
		t.Errorf("text = %q, want %q", text, "aaaX")
	}
}

// Backspacing a line empty puts the cursor back at column 1, where '/' is the
// command key again. BRE tests the column (compose_message 0x3A4), not whether
// the line has been typed into; IB peeked only a line's first key, so "/A" after
// a backspace landed in the message as text.
func TestComposeMessageSlashWorksAfterBackspacingToColumnOne(t *testing.T) {
	for _, c := range []struct {
		name     string
		keys     string
		wantSend bool
		wantText string
	}{
		{"abort", "keep\rab\b\b/A", false, ""},
		{"save", "keep\rab\b\b/S", true, "keep"},
	} {
		t.Run(c.name, func(t *testing.T) {
			f := &fakeSession{keys: []rune(c.keys)}
			text, send := composeMessage(f)
			if !strings.Contains(f.out.String(), breRuler) {
				t.Fatal("never reached the editor: the ruler was not drawn")
			}
			if send != c.wantSend {
				t.Fatalf("send = %v, want %v (text %q)", send, c.wantSend, text)
			}
			if send && text != c.wantText {
				t.Errorf("text = %q, want %q", text, c.wantText)
			}
			if strings.Contains(text, "/") {
				t.Errorf("the command key was taken as text: %q", text)
			}
		})
	}
}

// A backspace PAST column 1 still reopens the line above, and the reopened line
// is not at column 1, so a '/' typed there is ordinary text.
func TestComposeMessageSlashOnAReopenedLineIsText(t *testing.T) {
	f := &fakeSession{keys: []rune("hi\r\b/s\r/S")}
	text, send := composeMessage(f)
	if !send {
		t.Fatal("editor did not save")
	}
	if text != "hi/s" {
		t.Errorf("text = %q, want %q", text, "hi/s")
	}
}
