package game

type Config struct {
	TurnsPerDay     int
	ProtectionTurns int
	AICount         int
	DataDir         string
}

func DefaultConfig() Config {
	return Config{
		TurnsPerDay:     10,
		ProtectionTurns: 20,
		AICount:         0,
		DataDir:         "./data",
	}
}
