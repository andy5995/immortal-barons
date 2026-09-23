package ftn

import (
	"os"
	"path/filepath"
)

// handoffFaultFile holds the last handoff failure the sysop was alarmed about,
// under the transport's own spool rather than in the world: the handoff runs
// after the world is saved, and recording its outcome there would take the
// world lock back on every run.
const handoffFaultFile = "handoff-fault"

// NoteHandoffFault records the outcome of a handoff and reports whether fault
// is one the sysop has not yet been alarmed about. fault is the failure's text,
// or "" for a handoff that succeeded or had nothing configured, which clears
// the record so the same failure alarms again if it recurs.
//
// It is how a handoff that keeps failing raises the alarm once rather than on
// every scheduled run, as the planetary step's faults do (#187): compared by
// text, so a failure that changes is a new one.
func NoteHandoffFault(dataDir, fault string) (bool, error) {
	path := filepath.Join(dataDir, spoolDir, handoffFaultFile)
	if fault == "" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false, err
		}
		return false, nil
	}
	if last, err := os.ReadFile(path); err == nil && string(last) == fault {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return true, err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(fault), 0o644); err != nil {
		return true, err
	}
	return true, os.Rename(tmp, path)
}
