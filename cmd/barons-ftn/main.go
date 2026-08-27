// Command barons-ftn wraps and unwraps inter-BBS packets at an FTN boundary.
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
	inbound := flag.Bool("in", false, "receive, unwrap, and route inbound FTN transport bundles")
	outbound := flag.Bool("out", false, "bundle and hand off outbound game packets (the default)")
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
	if *inbound && *outbound {
		fmt.Fprintln(os.Stderr, "barons-ftn: use only one of --in and --out")
		os.Exit(2)
	}
	var result ftn.Result
	var err error
	if *inbound {
		result, err = ftn.RunIn(*dataDir)
	} else {
		result, err = ftn.RunOut(*dataDir)
	}
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
	if *inbound {
		fmt.Printf("Delivered %d packet(s) to the game.\n", result.Delivered)
	} else if len(result.Queued) == 0 {
		fmt.Println("No outbound packets.")
	}
}
