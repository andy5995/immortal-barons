// Command barons-web serves Immortal Barons in a web browser. It's a thin
// front-end: the browser runs xterm.js, receives the game's ANSI output over a
// Server-Sent Events stream, and POSTs keystrokes back. Each browser gets its
// own game session (keyed by a browser-generated id); the game itself runs
// through the same play.Run path as the local and door front-ends, so no game
// logic lives here.
//
// Stdlib only — SSE + POST instead of a websocket library, to avoid a
// dependency. That is clunkier than a socket but fine for a turn-based game.
package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

// hub tracks active browser sessions by id.
type hub struct {
	mu       sync.Mutex
	sessions map[string]*session.WebSession
	cfg      game.Config
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dataDir := flag.String("data", "./data", "game data directory")
	flag.Parse()

	cfg, err := store.LoadConfig(*dataDir)
	if err != nil {
		log.Fatalln("config:", err)
	}
	h := &hub{sessions: map[string]*session.WebSession{}, cfg: cfg}

	http.HandleFunc("/", h.index)
	http.HandleFunc("/stream", h.stream)
	http.HandleFunc("/key", h.key)

	log.Printf("Immortal Barons web front-end on %s", *addr)
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func (h *hub) index(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, indexHTML)
}

// stream opens the Server-Sent Events channel for a session, creating the
// session (and starting its game) on first connect.
func (h *hub) stream(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing id", http.StatusBadRequest)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	h.mu.Lock()
	ws, existed := h.sessions[id]
	if !existed {
		ws = session.NewWebSession()
		h.sessions[id] = ws
		go h.runGame(id, ws, r.URL.Query().Get("name"))
	}
	h.mu.Unlock()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher.Flush()

	for {
		select {
		case b := <-ws.Out():
			// Base64 so ANSI bytes (newlines, escapes, UTF-8) survive the
			// line-oriented SSE framing; the browser decodes to a byte array.
			fmt.Fprintf(w, "data: %s\n\n", base64.StdEncoding.EncodeToString(b))
			flusher.Flush()
		case <-ws.Done():
			return
		case <-r.Context().Done():
			return
		}
	}
}

// key feeds one or more keystrokes (the POST body) to a session.
func (h *hub) key(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	h.mu.Lock()
	ws := h.sessions[id]
	h.mu.Unlock()
	if ws == nil {
		http.Error(w, "no such session", http.StatusNotFound)
		return
	}
	body, _ := io.ReadAll(io.LimitReader(r.Body, 64))
	for _, ru := range string(body) {
		ws.Feed(ru)
	}
	w.WriteHeader(http.StatusNoContent)
}

// runGame plays one session to completion, then cleans it up.
func (h *hub) runGame(id string, ws *session.WebSession, name string) {
	defer func() {
		ws.Close()
		h.mu.Lock()
		delete(h.sessions, id)
		h.mu.Unlock()
	}()
	handle := name
	if handle == "" {
		handle = "web-" + id
	}
	today := time.Now().Format("2006-01-02")
	if err := play.Run(ws, play.Identity{Handle: handle}, h.cfg, today); err != nil {
		log.Printf("session %s: %v", id, err)
	}
	fmt.Fprint(ws, "\r\n\x1b[97mUntil next turn, Baron. Refresh to play again.\x1b[0m\r\n")
}

const indexHTML = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>Immortal Barons</title>
<!-- xterm.js pinned to an exact version. TODO before any real deployment:
     vendor these assets locally (or add Subresource Integrity hashes) so a
     CDN compromise cannot inject script. Loaded from a CDN for now while the
     web front-end is a work in progress. -->
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/xterm@5.3.0/css/xterm.min.css">
<script src="https://cdn.jsdelivr.net/npm/xterm@5.3.0/lib/xterm.min.js"></script>
<style>html,body{margin:0;height:100%;background:#000}#term{padding:6px}</style>
</head>
<body>
<div id="term"></div>
<script>
  const id = (window.crypto && crypto.randomUUID) ? crypto.randomUUID()
                                                   : String(Date.now()) + Math.random();
  const term = new Terminal({cols: 80, rows: 25, convertEol: true,
                             fontFamily: 'monospace', cursorBlink: true});
  term.open(document.getElementById('term'));
  term.focus();

  const es = new EventSource('/stream?id=' + id);
  es.onmessage = (e) => {
    const bin = atob(e.data);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    term.write(bytes);       // xterm decodes the UTF-8 bytes (block glyphs etc.)
  };

  term.onData((data) => {
    fetch('/key?id=' + id, {method: 'POST', body: data});
  });
</script>
</body>
</html>
`
