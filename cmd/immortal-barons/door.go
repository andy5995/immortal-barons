package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

// doorLog appends one timestamped diagnostic line to <dataDir>/ib-door.log. It
// is best-effort — a logging failure must never stop the door, so errors are
// ignored — and writes to a file (not stderr) so a remote sysop can capture the
// silent no-splash bounce (issue #37) without changing the door command. Short
// O_APPEND lines are atomic on a local filesystem, so concurrent nodes don't
// interleave.
func doorLog(dataDir, format string, args ...any) {
	f, err := os.OpenFile(filepath.Join(dataDir, "ib-door.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, time.Now().Format("2006-01-02 15:04:05")+" "+format+"\n", args...)
}

// ioModeName renders a dropfile I/O mode for the launch diagnostic.
func ioModeName(m door.IOMode) string {
	switch m {
	case door.IOSerial:
		return "serial"
	case door.IOSocket:
		return "socket"
	case door.IOStdio:
		return "stdio"
	default:
		return "local"
	}
}

// useSocket is the one place that decides between the socket and stdio, so the
// logged backend cannot drift from the one actually opened. See openSession for
// why the terminal test is Unix-only.
func useSocket(caller *door.Caller) bool {
	return caller.IO == door.IOSocket && (runtime.GOOS == "windows" || !session.StdinIsTerminal())
}

// backendName is the I/O path openSession takes for this caller — the thing a
// sysop actually needs in the log, as against what the drop file claimed.
func backendName(caller *door.Caller) string {
	switch {
	case useSocket(caller):
		return "socket"
	case caller.IO == door.IOSerial:
		return "unsupported-serial"
	default:
		return "stdio"
	}
}

// openSession attaches to the caller by the path useSocket picks, and returns a
// func that releases the connection.
func openSession(caller *door.Caller) (session.Session, func(), error) {
	// Which of the two I/O paths to take is NOT decided by the platform, and not
	// by the drop file alone. Two live Linux boards settle it:
	//
	//	Mystic       io=socket socket=8   stdin-tty=TRUE    works on stdio
	//	Synchronet   io=socket socket=58  stdin-tty=FALSE   dropped instantly
	//
	// Both name a socket, so the drop file cannot tell them apart. What differs
	// is whether the BBS also gave this process the connection on standard input:
	// Mystic hands over a terminal and expects the door to use it, while a native
	// Synchronet door with I/O interception off is given the socket as an
	// inherited descriptor and a stdin attached to nothing — so its first read
	// hit EOF and the caller was dropped the moment the door started.
	//
	// So: on Unix a live stdin wins, because a BBS that went to the trouble of
	// handing us one means it to be used. Only when there is no terminal AND the
	// drop file names a socket do we take the socket.
	//
	// That test is Unix-only, and deliberately so. A native Windows door is a
	// child of the BBS and inherits ITS console, so standard input is a terminal
	// there whether or not it carries the caller — the test cannot tell a handed-
	// over connection from the server's own console, and answering "terminal"
	// sends the door to a console the caller cannot see. Windows therefore keeps
	// the older rule it has always used: a drop file naming a socket means the
	// socket. Widening the Unix heuristic to Windows hung the door for a caller
	// (reported against the win32 snapshot).
	if useSocket(caller) {
		sock, err := session.NewSocket(caller.Socket)
		if err != nil {
			return nil, nil, fmt.Errorf("the drop file names socket %d and standard input is not a terminal, "+
				"but attaching to the socket failed: %w"+
				"\n(if your BBS pipes the connection through standard I/O instead, turn the door's"+
				"\nI/O interception on so it writes a Local drop file)",
				caller.Socket, err)
		}
		return sock, sock.Close, nil
	}
	if caller.IO == door.IOSerial {
		return nil, nil, fmt.Errorf("serial (FOSSIL) doors are not supported; configure your BBS for a socket or stdio door")
	}
	st := session.NewStdio()
	return st, st.Close, nil // Close restores a pty stdin's mode (no-op for a pipe)
}

// runLocal plays Immortal Barons locally in the caller's terminal against the
// shared persistent world, for someone testing or playing outside a BBS.
func runLocal(cfg game.Config, name, today string, cs charset, noANSI bool) {
	// -name "" would build a realm with no owner handle, which is the marker for a
	// computer baron — the same fallback the door path applies to a dropfile with
	// no alias.
	if strings.TrimSpace(name) == "" {
		name = defaultName()
	}
	c := session.NewConsole()
	defer c.Close()

	// -no-ansi forces the console's own plain mode, which a legacy Windows console
	// selects for itself (issue #98) — so the same one path is exercised.
	if noANSI {
		c.SetPlain()
	}

	// cs was resolved from the flags and the locale by wantCharset.
	s := encodeFor(session.Session(c), cs)
	if _, err := play.Run(s, play.Identity{Handle: name}, cfg, today); err != nil {
		fmt.Fprintln(os.Stderr, "immortal-barons -local:", err)
		return // no sign-off after a startup failure (e.g. no game — run -reset)
	}
	fmt.Fprint(s, "\nUntil next turn, Baron.\n")
}

// runDoor is the default front-end: the BBS launched us with a drop file.
// It resolves and parses that file, opens the caller's session over stdio or
// a socket, and plays a turn. Every other mode has returned by now.
func runDoor(cfg game.Config, o *opts, today string, cs charset) {
	// Running as a door: the sysop must have declared which drop file the BBS
	// writes (run -set-dropfile once, stored in door.json). Hard-error rather than
	// guess — a wrong guess silently misreads the caller. -help and every other
	// mode returned above, so this gates only real door launches.
	doorCfg, err := store.LoadDoorConfig(cfg.DataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "immortal-barons: door.json:", err)
		os.Exit(1)
	}
	if doorCfg.DropfileFormat == "" {
		fmt.Fprintln(os.Stderr, "immortal-barons:", dropfileUnsetMsg)
		os.Exit(2)
	}

	path := *o.dropPath
	if path == "" && flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	if path == "" {
		path = findDropfile(doorCfg.DropfileFormat)
	}
	if path == "" {
		// Log to ib-door.log too: with a live caller this is a silent bounce
		// (issue #37) — the BBS didn't write the drop file where we looked.
		want := doorCfg.DropfileFormat
		if f, ok := door.FormatByID(want); ok {
			want = f.File
		}
		doorLog(cfg.DataDir, "no drop file found: format=%q searched cwd for %s (pass -dropfile PATH)", doorCfg.DropfileFormat, want)
		fmt.Fprintln(os.Stderr, "immortal-barons: no dropfile found.")
		fmt.Fprintln(os.Stderr, "Run it as a BBS door with -dropfile PATH, or play in your terminal with -local.")
		fmt.Fprintln(os.Stderr)
		flag.Usage()
		os.Exit(2)
	}
	caller, err := door.ParseDropfileAs(path, doorCfg.DropfileFormat)
	if err != nil {
		// Log to ib-door.log too: a parse failure with a live caller is the
		// silent no-splash bounce issue #37 is about, and stderr isn't the
		// caller's connection under a BBS, so without this it leaves no trace.
		doorLog(cfg.DataDir, "drop file parse failed: path=%q format=%q err=%v", path, doorCfg.DropfileFormat, err)
		fmt.Fprintln(os.Stderr, "immortal-barons:", err)
		os.Exit(1)
	}

	// Diagnostic (data/ib-door.log): a silent no-splash bounce (issue #37) leaves
	// no other trace, so record what the dropfile gave us at launch — the I/O mode,
	// time-left, and socket handle name the environment. A file (not stderr) so a
	// remote tester needs no door-config change to capture it.
	doorLog(cfg.DataDir, "launch handle=%q node=%d io=%s seconds-left=%d socket=%d os=%s stdin-tty=%v ansi=%v",
		caller.Handle, caller.Node, ioModeName(caller.IO), caller.SecondsLeft, caller.Socket, runtime.GOOS, session.StdinIsTerminal(), caller.ANSI)

	s, closeSession, err := openSession(caller)
	// ...and separately, which backend actually opened. The line above reports
	// what the DROP FILE said; a sysop chasing a dead session needs to know what
	// the door then did with it, and the two used to differ silently.
	doorLog(cfg.DataDir, "session i/o backend=%s", backendName(caller))
	if err != nil {
		// Log the attach failure to the file too: a winsock socket attach that
		// fails here (issue #37, Windows socket doors) exits before the "session
		// ended" line, so without this it would leave only the launch line.
		doorLog(cfg.DataDir, "openSession failed: %v", err)
		fmt.Fprintln(os.Stderr, "immortal-barons:", err)
		os.Exit(1)
	}
	defer closeSession()

	// Traditional BBS terminals expect CP437, so transcode UTF-8 -> CP437 unless
	// the sysop forces another charset.
	s = encodeFor(s, wantCharset(*o.utf8, *o.cp437, *o.asciiOut, false))
	// A caller at the far end of a socket cannot be probed for ANSI the way a
	// local console can, so the dropfile flag their BBS profile filled in is the
	// only signal (issue #101). Without ANSI they would otherwise read every
	// escape as literal text.
	if !caller.ANSI || *o.noANSI {
		s = session.NewPlain(s)
	}

	handle := caller.Handle
	if handle == "" {
		handle = fmt.Sprintf("node%d", caller.Node)
	}
	// The caller's remaining BBS time is a hard session cap: play.Run's deadline
	// boots at it, saving the world and releasing the lock cleanly (unlike the
	// old os.Exit, which lost the turn's progress).
	id := play.Identity{Handle: handle, TimeLeft: time.Duration(caller.SecondsLeft) * time.Second}
	reason, err := play.Run(s, id, cfg, today)
	// Diagnostic (data/ib-door.log): record how the session ended. A silent
	// no-splash bounce (issue #37) shows up here as reason="disconnect" right
	// after launch — an I/O read that failed immediately, which points at a dead
	// handle rather than the time-left deadline (that prints before it boots).
	doorLog(cfg.DataDir, "session ended handle=%q reason=%q err=%v", handle, reason, err)
	if err != nil {
		// Fail loudly to the CALLER, not just the BBS log. A bootstrap failure
		// (world load, lock, I/O) otherwise drops the caller straight back to the
		// BBS menu with no splash and no reason — looking like the door is broken.
		// Write the reason to their screen and hold it briefly so they can read it
		// before the BBS reclaims the screen.
		fmt.Fprintf(s, "\r\nImmortal Barons could not start:\r\n  %v\r\n\r\nPlease tell the sysop. Returning to the BBS...\r\n", err)
		fmt.Fprintln(os.Stderr, "immortal-barons:", err)
		time.Sleep(4 * time.Second)
		os.Exit(1)
	}
}
