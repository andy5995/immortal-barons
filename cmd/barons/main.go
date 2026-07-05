// Command barons plays Immortal Barons locally in your terminal against the
// shared persistent world.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
	"github.com/andy5995/immortal-barons/internal/store"
)

func main() {
	name := flag.String("name", defaultName(), "your player handle")
	dataDir := flag.String("data", "./data", "game data directory")
	flag.Parse()

	cfg, err := store.LoadConfig(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "config:", err)
		os.Exit(1)
	}
	today := time.Now().Format("2006-01-02")

	c := session.NewConsole()
	defer c.Close()

	if _, err := play.Run(c, play.Identity{Handle: *name}, cfg, today); err != nil {
		fmt.Fprintln(os.Stderr, "barons:", err)
	}
	fmt.Fprint(c, "\nUntil next turn, Baron.\n")
}

func defaultName() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return "sysop"
}
