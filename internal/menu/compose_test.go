package menu

import "testing"

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
