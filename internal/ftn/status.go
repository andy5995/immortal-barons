package ftn

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// PeerWait is one peer's share of the outbound backlog. Snapshots counts the
// claimed snapshots that still hold a target for it — not files, and not
// packets: a snapshot is kept whole until every target in it is published, so
// it also holds the bundles of peers that already went out (#228).
type PeerWait struct {
	Name      string
	Snapshots int
	Oldest    time.Duration
	LastError string
}

// ReceiptWait is one inbound bundle that did not finish. Reason says which of
// the three ways it can stall applies, read from the receipt rather than
// guessed from the directory.
type ReceiptWait struct {
	ID     string
	Age    time.Duration
	Reason string
}

// SpoolStatus is what a sysop needs to tell a working transport from a stalled
// one, in the terms the journals actually record. Nothing here is a file count:
// counting files is what makes a healthy spool look like a backlog.
type SpoolStatus struct {
	Peers      []PeerWait
	Inbound    []ReceiptWait
	Unreadable []string // spool directories whose journal will not parse
	SetAside   int      // packets in the bad folder, which nothing retries
}

// Status reads both spools without changing either. It is deliberately
// read-only: a sysop reaching for it is trying to find out what is happening,
// which is the worst moment to move anything.
func Status(dataDir string) (SpoolStatus, error) {
	var status SpoolStatus
	now := time.Now()

	byPeer := map[string]*PeerWait{}
	outRoot := filepath.Join(dataDir, spoolDir, outSpoolDir)
	if err := eachSpoolDir(outRoot, func(name, dir string) {
		planPath := filepath.Join(dir, batchPlanFile)
		plan, err := loadBatchPlan(planPath)
		if err != nil {
			if !os.IsNotExist(err) {
				status.Unreadable = append(status.Unreadable, filepath.Join(outSpoolDir, name))
			}
			return
		}
		age := now.Sub(lastAdvanced(plan, planPath))
		for _, target := range plan.Targets {
			if target.Done {
				continue
			}
			peer := byPeer[target.Name]
			if peer == nil {
				peer = &PeerWait{Name: target.Name}
				byPeer[target.Name] = peer
			}
			peer.Snapshots++
			if age > peer.Oldest {
				peer.Oldest = age
			}
			if target.LastError != "" {
				peer.LastError = target.LastError
			}
		}
	}); err != nil {
		return status, err
	}
	for _, peer := range byPeer {
		status.Peers = append(status.Peers, *peer)
	}
	sort.Slice(status.Peers, func(a, b int) bool {
		if status.Peers[a].Oldest != status.Peers[b].Oldest {
			return status.Peers[a].Oldest > status.Peers[b].Oldest // the longest wait first
		}
		return status.Peers[a].Name < status.Peers[b].Name
	})

	inRoot := filepath.Join(dataDir, spoolDir, inSpoolDir)
	if err := eachSpoolDir(inRoot, func(name, dir string) {
		planPath := filepath.Join(dir, inboundReceiptFile)
		receipt, err := loadInboundReceipt(planPath)
		if err != nil {
			if !os.IsNotExist(err) {
				status.Unreadable = append(status.Unreadable, filepath.Join(inSpoolDir, name))
			}
			return
		}
		status.Inbound = append(status.Inbound, ReceiptWait{
			ID:     receipt.ID,
			Age:    now.Sub(receiptAdvanced(receipt, planPath)),
			Reason: receiptReason(receipt),
		})
	}); err != nil {
		return status, err
	}
	sort.Slice(status.Inbound, func(a, b int) bool { return status.Inbound[a].Age > status.Inbound[b].Age })

	bad, err := os.ReadDir(filepath.Join(dataDir, spoolDir, badSpoolDir))
	if err == nil {
		for _, e := range bad {
			if !e.IsDir() {
				status.SetAside++
			}
		}
	}
	return status, nil
}

func eachSpoolDir(root string, fn func(name, dir string)) error {
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			fn(entry.Name(), filepath.Join(root, entry.Name()))
		}
	}
	return nil
}

func receiptAdvanced(receipt inboundReceipt, planPath string) time.Time {
	if !receipt.Progress.IsZero() {
		return receipt.Progress
	}
	if !receipt.Created.IsZero() {
		return receipt.Created
	}
	if info, err := os.Stat(planPath); err == nil {
		return info.ModTime()
	}
	return time.Now()
}

// receiptReason names which of the three stalls this receipt is in. They want
// different answers from a sysop — a collision is a decision, a busy peer is a
// wait — so a single "incomplete" would send them looking in the wrong place.
func receiptReason(receipt inboundReceipt) string {
	for _, local := range receipt.Local {
		if !local.Done {
			return fmt.Sprintf("packet %s not yet delivered to the game; a canonical-name collision needs your decision", local.Name)
		}
	}
	for _, target := range receipt.Targets {
		if !target.Done {
			reason := "waiting on that peer"
			if target.LastError != "" {
				reason = target.LastError
			}
			return fmt.Sprintf("transit for %s: %s", target.Name, reason)
		}
	}
	return "delivered, waiting to clear its source or envelope"
}
