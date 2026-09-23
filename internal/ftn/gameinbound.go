package ftn

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"time"

	"github.com/andy5995/immortal-barons/internal/store"
)

// gameInboundGrace is how long a bundle in the game's inbound that cannot be
// read is left where it is before it is set aside. A ZIP's index is written
// last, so a mailer still writing one leaves a file that fails to open, and
// setting that aside would lose a bundle that reads cleanly a minute later. The
// same window the game gives an unreadable packet.
const gameInboundGrace = 5 * time.Minute

// UnwrapGameInbound unwraps the transport bundles found in the game's own
// inbound directory into plain packets beside them, for the planetary step to
// apply (#230). A board with no FTN settings at all meets bundles there when its
// mailer delivers straight into the game's inbound, and so does one whose
// transport is set up but whose peer publishes into that directory; either way
// the bundle is read as a bundle rather than set aside as a corrupt packet.
//
// Nothing is forwarded over FTN from here. A packet in transit is delivered
// into the game's inbound as though it had arrived unbundled, and the
// planetary step passes it on exactly as it does such a packet. Plain packets
// are left alone. It waits for another run holding the transport lock.
func UnwrapGameInbound(dataDir string) (Result, error) { return unwrapGameInbound(dataDir, true) }

// TryUnwrapGameInbound is UnwrapGameInbound for a caller that must not wait;
// see TryRunOut.
func TryUnwrapGameInbound(dataDir string) (Result, error) { return unwrapGameInbound(dataDir, false) }

func unwrapGameInbound(dataDir string, wait bool) (Result, error) {
	board, err := store.LoadConfig(dataDir)
	if err != nil {
		return Result{}, err
	}
	inbound := cleanAbsolute(board.Inbound())
	// A directory that is also a mailer inbound is RunIn's to read: it matches
	// an attached bundle to its envelope, and it waits for that envelope.
	if transport, err := LoadConfig(dataDir); err == nil &&
		slices.ContainsFunc(transport.IncomingFileDirs, func(dir string) bool { return cleanAbsolute(dir) == inbound }) {
		return Result{}, nil
	}
	// Looked for before anything is locked or read, so a board with none --
	// every board whose peers send plain packets -- pays for a directory read
	// and nothing else, and needs no roster to do it.
	bundles, err := gameInboundBundles(inbound)
	if err != nil || len(bundles) == 0 {
		return Result{}, err
	}
	board, nodes, world, lock, err := leagueContext(dataDir, wait)
	if err != nil {
		return Result{}, err
	}
	defer lock.Release()
	root := filepath.Join(dataDir, spoolDir, inSpoolDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return Result{}, err
	}
	var result Result
	fromHere := func(receipt inboundReceipt) bool { return cleanAbsolute(filepath.Dir(receipt.Source)) == inbound }
	if err := resumeInboundReceipts(root, board, dataDir, Config{}, world, nodes, Address{}, fromHere, &result); err != nil {
		return result, err
	}
	for _, path := range bundles {
		name := filepath.Base(path)
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			continue // a resumed receipt above has just finished it
		}
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s: %v", name, err))
			continue
		}
		if _, _, err := readTransport(data, name); err != nil {
			setAsideGameBundle(dataDir, path, err, &result)
			continue
		}
		// A transmitter missing from the roster is left in place rather than set
		// aside: the roster that names a new board may arrive in this very run,
		// and the next one then reads the bundle.
		if err := ingestTransportFile(root, board, dataDir, Config{}, world, nodes, Address{}, path, "", "", Address{}, false, &result); err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s in %s: %v; it stays there for the next run", name, inbound, err))
		}
	}
	return result, nil
}

// gameInboundBundles lists the packet files in dir that are bundles.
func gameInboundBundles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var bundles []string
	for _, entry := range entries {
		path := filepath.Join(dir, entry.Name())
		if !entry.IsDir() && store.IsPacketFile(entry.Name()) && store.IsBundleFile(path) {
			bundles = append(bundles, path)
		}
	}
	return bundles, nil
}

// setAsideGameBundle moves a bundle that cannot be read to where the transport
// sets aside its own, once it is old enough not to be a write in progress. Left
// in place, it would be read and refused on every run for as long as the board
// runs.
func setAsideGameBundle(dataDir, path string, cause error, result *Result) {
	name := filepath.Base(path)
	if info, err := os.Stat(path); err == nil && time.Since(info.ModTime()) < gameInboundGrace {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s is not a readable transport bundle yet (%v); it may still be arriving, so it is left for the next run", name, cause))
		return
	}
	if err := quarantineTransport(dataDir, path); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%s is not a readable transport bundle (%v) and could not be set aside: %v", name, cause, err))
		return
	}
	result.Warnings = append(result.Warnings, fmt.Sprintf(
		"%s is not a readable transport bundle (%v); it has been set aside in %s",
		name, cause, filepath.Join(dataDir, spoolDir, badSpoolDir)))
}
