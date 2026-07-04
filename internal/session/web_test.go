package session

import (
	"io"
	"testing"
)

func TestWebSessionRoundTrip(t *testing.T) {
	ws := NewWebSession()
	ws.Feed('x')
	r, err := ws.ReadKey()
	if err != nil || r != 'x' {
		t.Fatalf("ReadKey: got %q, %v", r, err)
	}
	if _, err := ws.Write([]byte("hi")); err != nil {
		t.Fatal(err)
	}
	if got := <-ws.Out(); string(got) != "hi" {
		t.Errorf("Out: got %q, want \"hi\"", got)
	}
}

func TestWebSessionCloseUnblocksReadKeyAndIsIdempotent(t *testing.T) {
	ws := NewWebSession()
	ws.Close()
	if _, err := ws.ReadKey(); err != io.EOF {
		t.Errorf("ReadKey after Close: want EOF, got %v", err)
	}
	ws.Close() // must not panic
}
