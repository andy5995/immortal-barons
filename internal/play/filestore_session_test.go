package play

import (
	"sync"
	"testing"

	"github.com/andy5995/immortal-barons/internal/store"
)

// TestConcurrentDoorSessionsFileStore runs two door sessions against ONE data
// dir at the same time. Each goes through the real Run wiring (load → FileStore
// → onboard → quit). Before Task 4, Run held a session-long non-blocking flock,
// so the second caller would have been turned away with ErrBusy ("busy"); now
// every action is its own file-locked transaction, so both onboard and both
// realms land in world.json.
func TestConcurrentDoorSessionsFileStore(t *testing.T) {
	cfg := cfgIn(t.TempDir())

	var wg sync.WaitGroup
	run := func(handle, realm string) {
		defer wg.Done()
		f := &fakeSession{keys: []rune(" \r" + realm + "\r0")} // splash, English, realm, quit
		reason, err := Run(f, Identity{Handle: handle}, cfg, "2026-07-05")
		if err != nil {
			t.Errorf("%s: %v", handle, err)
		}
		if reason == "busy" {
			t.Errorf("%s: session was bounced as busy — the session-long lock should be gone", handle)
		}
	}

	wg.Add(2)
	go run("alice", "Alicia")
	go run("bob", "Bobsland")
	wg.Wait()

	w, err := store.Load(cfg)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if e := w.FindByOwner("alice"); e == nil || e.Name != "Alicia" {
		t.Errorf("alice not persisted: %v", e)
	}
	if e := w.FindByOwner("bob"); e == nil || e.Name != "Bobsland" {
		t.Errorf("bob not persisted: %v", e)
	}
}
