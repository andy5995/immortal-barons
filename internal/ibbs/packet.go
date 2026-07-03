// Package ibbs defines the inter-BBS packet format for sharing scores
// between boards. A sysop's mailer transports these files; this package
// only reads and writes them.
package ibbs

import (
	"encoding/json"
	"os"
)

type Score struct {
	Empire   string
	NetWorth int
	Land     int
}

type Packet struct {
	BoardID string
	Date    string
	Scores  []Score
}

// Write saves a packet as JSON to path.
func Write(path string, p Packet) error {
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// Read loads a packet from path.
func Read(path string) (Packet, error) {
	var p Packet
	data, err := os.ReadFile(path)
	if err != nil {
		return p, err
	}
	err = json.Unmarshal(data, &p)
	return p, err
}
