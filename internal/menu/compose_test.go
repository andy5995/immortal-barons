package menu

import "testing"

// The message editor advertises /S=save /A=abort /C=clear in its header, so a
// line that is exactly one of those commands must act on it (the reported bug
// was that only a bare "/" did anything, so "/S" got added as text).

func TestComposeMessageSlashSSaves(t *testing.T) {
	f := &fakeSession{keys: []rune("Hello\r/S\r")}
	msg, ok := composeMessage(f)
	if !ok {
		t.Fatal("/S should save the message")
	}
	if msg != "Hello" {
		t.Fatalf("msg = %q, want %q", msg, "Hello")
	}
}

func TestComposeMessageSlashALowercaseAborts(t *testing.T) {
	f := &fakeSession{keys: []rune("Hi\r/a\r")}
	msg, ok := composeMessage(f)
	if ok {
		t.Fatalf("/a should abort, got ok=true msg=%q", msg)
	}
}

func TestComposeMessageSlashCClears(t *testing.T) {
	// Type a line, clear it, type another, then save: only the second survives.
	f := &fakeSession{keys: []rune("first\r/C\rsecond\r/S\r")}
	msg, ok := composeMessage(f)
	if !ok {
		t.Fatal("/S after /C should save")
	}
	if msg != "second" {
		t.Fatalf("msg = %q, want %q (/C should have cleared 'first')", msg, "second")
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
