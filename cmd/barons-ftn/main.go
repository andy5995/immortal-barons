// Command barons-ftn wraps and unwraps inter-BBS packets at an FTN boundary.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/andy5995/immortal-barons/internal/ftn"
	"github.com/andy5995/immortal-barons/internal/game"
	"github.com/andy5995/immortal-barons/internal/textwrap"
)

func main() {
	dataDir := flag.String("data", "./data", "folder that holds the game data and ftn.cfg")
	inbound := flag.Bool("in", false, "receive, unwrap, and route inbound FTN transport bundles")
	outbound := flag.Bool("out", false, "bundle and hand off outbound game packets (the default)")
	status := flag.Bool("status", false, "report what each spool is holding and why, changing nothing")
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
	if *status {
		if err := reportStatus(*dataDir); err != nil {
			fmt.Fprintln(os.Stderr, "barons-ftn:", err)
			os.Exit(1)
		}
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
	} else if result.Snapshots > 0 {
		// An empty system and a stalled one both queue nothing, and a scheduled
		// run's log is the only place a sysop would ever see the difference
		// (#228).
		fmt.Printf("%d queued; %d snapshot(s) still waiting on %d peer(s): %s (oldest %s without progress)\n",
			len(result.Queued), result.Snapshots, len(result.Waiting),
			strings.Join(result.Waiting, ", "), result.OldestWait.Round(time.Minute))
		for _, stalled := range result.Stalled {
			fmt.Printf("  %s\n", stalled)
		}
	} else if len(result.Queued) == 0 {
		fmt.Println("No outbound packets.")
	}
}

// reportStatus answers the question a file count cannot: not how many files
// are in the spool, but what is actually unfinished, for whom, for how long,
// and why. It reads and prints; a sysop reaching for it is trying to find out
// what is happening, which is the worst moment to move anything (#228).
func reportStatus(dataDir string) error {
	status, err := ftn.Status(dataDir)
	if err != nil {
		return err
	}
	if len(status.Peers) == 0 {
		fmt.Println("Outbound: nothing waiting.")
	} else {
		fmt.Println("Outbound, longest wait first:")
		for _, peer := range status.Peers {
			fmt.Printf("  %-24s %d snapshot(s), %s without progress\n",
				peer.Name, peer.Snapshots, peer.Oldest.Round(time.Minute))
			if peer.LastError != "" {
				fmt.Printf("  %-24s last failure: %s\n", "", peer.LastError)
			}
		}
		fmt.Println("  A snapshot is kept whole until every target in it publishes, so it")
		fmt.Println("  also holds bundles for peers that already went out.")
	}
	if len(status.Inbound) == 0 {
		fmt.Println("Inbound: nothing pending.")
	} else {
		fmt.Println("Inbound, longest wait first:")
		for _, receipt := range status.Inbound {
			fmt.Printf("  %-24s %s: %s\n", receipt.ID, receipt.Age.Round(time.Minute), receipt.Reason)
		}
	}
	for _, dir := range status.Unreadable {
		fmt.Printf("Unreadable journal in %s: neither retry state nor quarantine, and nothing will retry it.\n", dir)
	}
	if status.SetAside > 0 {
		fmt.Printf("Set aside: %d packet(s) nothing retries; read and clear them once the cause is fixed.\n", status.SetAside)
	}
	return nil
}
