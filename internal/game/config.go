package game

type Config struct {
	TurnsPerDay     int
	ProtectionTurns int
	AICount         int
	DataDir         string
	GameLength      int    // days before the league ends and resets; 0 = endless
	BoardID         string // name of this board in exported inter-BBS packets
	IBBS            bool   // participate in inter-BBS play (gates the interplanetary menus)
}

// InterBBSEnabled reports whether inter-BBS / interplanetary features (group
// attacks, IP scores) should be offered: the game is IBBS-configured, or is a
// timed league.
func (c Config) InterBBSEnabled() bool {
	return c.IBBS || c.GameLength > 0
}

func DefaultConfig() Config {
	return Config{
		TurnsPerDay:     10,
		ProtectionTurns: 20,
		AICount:         0,
		DataDir:         "./data",
		GameLength:      0,
		BoardID:         "local",
	}
}
