// Command barons plays Immortal Barons locally in your terminal against the
// shared persistent world.
package main

import (
	"flag"
	"fmt"
	"os"
	"os/user"
	"time"

	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/play"
	"github.com/andy5995/immortal-barons/internal/session"
)

const version = "0.0.1"

func main() {
	name := flag.String("name", defaultName(), "your player handle")
	dataDir := flag.String("data", "./data", "game data directory")
	flag.Parse()

	cfg := game.DefaultConfig()
	cfg.DataDir = *dataDir
	today := time.Now().Format("2006-01-02")

	c := session.NewConsole()
	defer c.Close()

	fmt.Fprintf(c, "\n      IMMORTAL BARONS  v%s\n\n", version)
	if err := play.Run(c, play.Identity{Handle: *name}, cfg, today); err != nil {
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
