// Command immortal-barons-web serves Immortal Barons in a web browser. It's a thin
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
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
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
	world    *game.World
}

// saveWorld persists the shared world under the world lock.
func (h *hub) saveWorld() error {
	var err error
	h.world.With(func() { err = store.Save(h.world, h.cfg) })
	return err
}

func main() {
	addr := flag.String("addr", ":8080", "listen address")
	dataDir := flag.String("data", "./data", "game data directory")
	flag.Parse()

	cfg, err := store.LoadConfig(*dataDir)
	if err != nil {
		log.Fatalln("config:", err)
	}

	lock, err := store.Lock(cfg, false)
	if err != nil {
		log.Fatalf("cannot start: another process holds this world (a web-hosted world can't be shared with a door): %v", err)
	}
	defer lock.Release()
	world, err := store.Load(cfg)
	if err != nil {
		log.Fatalf("load world: %v", err)
	}
	today := time.Now().Format("2006-01-02")
	world.Today = today
	world.DailyMaintenance(today)
	h := &hub{sessions: map[string]*session.WebSession{}, cfg: cfg, world: world}

	// Daily maintenance on date rollover (guarded by the world lock).
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			d := time.Now().Format("2006-01-02")
			h.world.With(func() {
				if h.world.Today != d {
					h.world.Today = d
					h.world.DailyMaintenance(d)
				}
			})
		}
	}()

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
	// The session id is a server-generated secret carried in an HttpOnly
	// cookie — never a client-supplied query param — so it can't be guessed,
	// forged, or used to impersonate another caller's handle.
	if _, err := r.Cookie("bsid"); err != nil {
		b := make([]byte, 16)
		if _, err := rand.Read(b); err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     "bsid",
			Value:    hex.EncodeToString(b),
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	io.WriteString(w, indexHTML)
}

// sessionID reads the secret session id from the bsid cookie.
func sessionID(r *http.Request) (string, bool) {
	c, err := r.Cookie("bsid")
	if err != nil || c.Value == "" {
		return "", false
	}
	return c.Value, true
}

// stream opens the Server-Sent Events channel for a session, creating the
// session (and starting its game) on first connect.
func (h *hub) stream(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(r)
	if !ok {
		http.Error(w, "no session; load / first", http.StatusBadRequest)
		return
	}
	flusher, canFlush := w.(http.Flusher)
	if !canFlush {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	h.mu.Lock()
	ws, existed := h.sessions[id]
	if !existed {
		if h.cfg.MaxConcurrentSessions > 0 && len(h.sessions) >= h.cfg.MaxConcurrentSessions {
			h.mu.Unlock()
			http.Error(w, "The realm is full — try again shortly.", http.StatusServiceUnavailable)
			return
		}
		ws = session.NewWebSession()
		h.sessions[id] = ws
		log.Printf("session connected from %s", r.RemoteAddr)
		go h.runGame(id, ws, r.RemoteAddr)
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
			// Game over: flush any output still buffered, then send a terminal
			// "end" event so the browser's EventSource stops instead of
			// auto-reconnecting and starting a brand-new game.
			for draining := true; draining; {
				select {
				case b := <-ws.Out():
					fmt.Fprintf(w, "data: %s\n\n", base64.StdEncoding.EncodeToString(b))
				default:
					draining = false
				}
			}
			fmt.Fprint(w, "event: end\ndata: \n\n")
			flusher.Flush()
			return
		case <-r.Context().Done():
			// The tab/stream closed. The game keeps running until the idle
			// timeout boots it (the browser's EventSource may just be
			// reconnecting), so don't end the session here.
			log.Printf("session from %s stream disconnected", r.RemoteAddr)
			return
		}
	}
}

// key feeds one or more keystrokes (the POST body) to a session.
func (h *hub) key(w http.ResponseWriter, r *http.Request) {
	id, ok := sessionID(r)
	if !ok {
		http.Error(w, "no session", http.StatusBadRequest)
		return
	}
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

// runGame plays one session to completion, then cleans it up. addr is the
// caller's remote address, logged for the operator (the secret session id is
// never logged).
func (h *hub) runGame(id string, ws *session.WebSession, addr string) {
	defer func() {
		ws.Close()
		h.mu.Lock()
		delete(h.sessions, id)
		h.mu.Unlock()
	}()
	// Handle is derived from the secret session id, so a web caller cannot
	// claim another caller's BBS handle.
	reason, err := play.Session(ws, play.Identity{Handle: "web-" + id}, h.world, h.cfg, h.saveWorld)
	if err != nil {
		log.Printf("session from %s: %v", addr, err)
	}
	log.Printf("session from %s ended (%s)", addr, reason)
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
  // The session id lives in an HttpOnly cookie set by GET /, sent
  // automatically with same-origin requests — the page never handles it.
  const term = new Terminal({cols: 80, rows: 25, convertEol: true,
                             fontFamily: 'monospace', cursorBlink: true});
  term.open(document.getElementById('term'));
  term.focus();

  const es = new EventSource('/stream');
  // The server sends an "end" event when the game is over. Close the stream so
  // the browser does not auto-reconnect and start a fresh game on its own.
  es.addEventListener('end', () => es.close());
  es.onmessage = (e) => {
    const bin = atob(e.data);
    const bytes = new Uint8Array(bin.length);
    for (let i = 0; i < bin.length; i++) bytes[i] = bin.charCodeAt(i);
    term.write(bytes);       // xterm decodes the UTF-8 bytes (block glyphs etc.)
  };

  term.onData((data) => {
    fetch('/key', {method: 'POST', body: data});
  });
</script>
</body>
</html>
`
