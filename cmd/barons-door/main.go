// Command barons-door runs Immortal Barons as a native BBS door. The BBS
// writes a dropfile (DOOR32.SYS or DOOR.SYS) and launches this program with
// the caller's connection on stdin/stdout. It reads the dropfile to learn
// who the caller is and how much time they have, then runs the game.
//
// Configure your BBS to run it with the dropfile path, e.g.:
//
//	barons-door -dropfile /path/to/node/DOOR32.SYS
//
// With no -dropfile, it looks for DOOR32.SYS or DOOR.SYS in the working
// directory.
package main

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/andy5995/immortal-barons/internal/ansi"
	"github.com/andy5995/immortal-barons/internal/door"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/menu"
	"github.com/andy5995/immortal-barons/internal/session"
)

func main() {
	dropPath := flag.String("dropfile", "", "path to the BBS dropfile (DOOR32.SYS or DOOR.SYS)")
	flag.Parse()

	path := *dropPath
	if path == "" && flag.NArg() > 0 {
		path = flag.Arg(0)
	}
	if path == "" {
		path = findDropfile()
	}
	if path == "" {
		fmt.Fprintln(os.Stderr, "barons-door: no dropfile found; pass -dropfile PATH")
		os.Exit(2)
	}

	caller, err := door.ParseDropfile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "barons-door:", err)
		os.Exit(1)
	}

	s := session.NewStdio()

	// Honor the caller's remaining BBS time with a hard cutoff.
	if caller.SecondsLeft > 0 {
		go func() {
			time.Sleep(time.Duration(caller.SecondsLeft) * time.Second)
			fmt.Fprintf(s, "\n\n%sYour BBS time is up. Farewell, Baron!%s\n", ansi.FgBrightYellow, ansi.Reset)
			os.Exit(0)
		}()
	}

	name := caller.Handle
	if name == "" {
		name = "Baron"
	}
	fmt.Fprintf(s, "%s%sIMMORTAL BARONS%s\n", ansi.Clear, ansi.FgBrightYellow, ansi.Reset)
	fmt.Fprintf(s, "Welcome, %s (node %d).\n\n", name, caller.Node)

	// Until persistence lands, each call is a fresh game named for the caller.
	g := game.New()
	if caller.Handle != "" {
		g.Player().Name = caller.Handle
	}
	menu.Run(s, g, menu.Build())

	fmt.Fprintf(s, "%s\nUntil next turn, Baron.\n", ansi.Reset)
}

func findDropfile() string {
	for _, n := range []string{"door32.sys", "DOOR32.SYS", "door.sys", "DOOR.SYS"} {
		if _, err := os.Stat(n); err == nil {
			return n
		}
	}
	return ""
}
