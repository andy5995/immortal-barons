// Command barons-ftn moves outbound inter-BBS packets into the FTN handoff
// directory and creates Synchronet-compatible Type-2 file-attach netmail.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/andy5995/immortal-barons/internal/ftn"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/textwrap"
)

func main() {
	dataDir := flag.String("data", "./data", "folder that holds the game data and ftn.cfg")
	version := flag.Bool("version", false, "print the version, then exit")
	flag.Parse()
	if flag.NArg() != 0 {
		fmt.Fprintf(os.Stderr, "barons-ftn: unknown argument %q\n", flag.Arg(0))
		os.Exit(2)
	}
	if *version {
		fmt.Printf("barons-ftn %s\n", game.VersionString())
		fmt.Printf("go: %s\n", runtime.Version())
		return
	}
	result, err := ftn.Run(*dataDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "barons-ftn:", err)
		os.Exit(1)
	}
	for _, warning := range result.Warnings {
		const prefix = "barons-ftn: warning: "
		fmt.Fprint(os.Stderr, prefix,
			textwrap.Wrap(warning, textwrap.Console, strings.Repeat(" ", len(prefix))), "\n")
	}
	for _, queued := range result.Queued {
		fmt.Printf("Queued %s for %s (%s) as %s\n",
			queued.PacketPath, queued.NextHop, queued.Address, queued.Message)
	}
	if len(result.Queued) == 0 {
		fmt.Println("No outbound packets.")
	}
}
