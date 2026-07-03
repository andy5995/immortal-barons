package game

type Config struct {
	TurnsPerDay     int
	ProtectionTurns int
	AICount         int
	DataDir         string
	GameLength      int // days before the league ends and resets; 0 = endless
}

func DefaultConfig() Config {
	return Config{
		TurnsPerDay:     10,
		ProtectionTurns: 20,
		AICount:         0,
		DataDir:         "./data",
		GameLength:      0,
	}
}
