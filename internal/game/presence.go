package game

// presence.go — who is on the board right now.
//
// There is no session table and no connection state in the world file: the door
// is file-per-action, so the only thing every node shares is world.json. A
// baron's presence is therefore a timestamp they refresh as they play, and
// "online" means that stamp is recent.
//
// The per-action stamp rides an existing write: every menu action already runs
// one world transaction (menu.postActionCheck), which reloads, mutates and
// saves, so refreshing it there costs no extra I/O and no extra lock. The one
// deliberate extra transaction is at session start, which is what puts a caller
// who never acts on the roster at all.
//
// This is deliberately approximate. A baron reading a long screen or composing
// a message runs no transaction and drops off the roster until their next
// action. That is the harmless direction — see OnlineWindowSecs for why the
// window is short rather than generous.

// MarkActive records that this baron just acted. Call it under the world lock,
// on a freshly resolved empire.
func (e *Empire) MarkActive() { e.LastActive = timeNow().Unix() }

// MarkOffline clears the stamp, so a baron who logs off (or whose disconnect
// the door catches) leaves the roster at once instead of lingering for the
// window. A stamp that survives means the session died without unwinding.
func (e *Empire) MarkOffline() { e.LastActive = 0 }

// Online reports whether this baron acted within OnlineWindowSecs. An empire
// that has never played — every AI realm — carries a zero stamp and is never
// online.
func (e *Empire) Online() bool {
	if e.LastActive == 0 {
		return false
	}
	return timeNow().Unix()-e.LastActive < OnlineWindowSecs
}
