package store

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/andy5995/immortal-barons/internal/game"
)

// League key files (#53). The Coordinator's board holds coord.key, the private
// half; every board in the league holds coord.pub, the public half. Only the
// Coordinator can sign an order, and holding the public half lets nobody forge
// one — which is the difference between this and a shared secret every member
// could sign with.
//
// The names mirror BRE's COORD.KEY, which was a binary secret a sysop copied to
// hand coordinatorship on. Handing it on here means copying coord.key.
const (
	CoordKeyFile = "coord.key"
	CoordPubFile = "coord.pub"
)

// GenerateCoordKey writes a new Coordinator key pair into dir and returns the
// public half as hex, for the sysop to send to the other boards. Refuses to
// overwrite an existing private key: doing so would orphan every board that
// already holds the matching public one.
func GenerateCoordKey(dir string) (string, error) {
	priv := filepath.Join(dir, CoordKeyFile)
	if _, err := os.Stat(priv); err == nil {
		return "", os.ErrExist
	}
	pub, sec, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(priv, []byte(hex.EncodeToString(sec)+"\n"), 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, CoordPubFile), []byte(hex.EncodeToString(pub)+"\n"), 0o644); err != nil {
		return "", err
	}
	return hex.EncodeToString(pub), nil
}

// InstallCoordPub records the league's Coordinator public key on this board, so
// it can check the orders it is sent.
func InstallCoordPub(dir, hexKey string) error {
	raw, err := hex.DecodeString(strings.TrimSpace(hexKey))
	if err != nil {
		return err
	}
	if len(raw) != ed25519.PublicKeySize {
		return errors.New("that is not a valid Coordinator public key")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, CoordPubFile), []byte(hex.EncodeToString(raw)+"\n"), 0o644)
}

// loadLeagueKeys reads whatever key material this board has. Missing files are
// not an error: a stand-alone board needs none, and a league board without them
// simply cannot verify orders, which VerifyCoordinatorOrders treats as refusal.
func loadLeagueKeys(w *game.World, cfg game.Config) {
	w.CoordKey = readHexKey(filepath.Join(cfg.DataDir, CoordKeyFile), ed25519.PrivateKeySize)
	w.CoordPub = readHexKey(filepath.Join(cfg.DataDir, CoordPubFile), ed25519.PublicKeySize)
}

func readHexKey(path string, want int) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil || len(raw) != want {
		return nil
	}
	return raw
}
