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
	// BoardKeyFile is this board's own signing key (#118) — a different job from
	// the two above. The Coordinator pair says who may dictate league rules;
	// this one says which board a packet actually came from. A board that is not
	// the Coordinator still needs it, and the Coordinator needs both.
	BoardKeyFile = "board.key"
)

// GenerateCoordKey writes a new Coordinator key pair into dir and returns the
// public half as hex, for the sysop to send to the other boards.
func GenerateCoordKey(dir string) (string, error) {
	pub, err := generateKeyPair(dir, CoordKeyFile)
	if err != nil {
		return "", err
	}
	// The Coordinator alone also keeps its own public half on disk, so its board
	// can verify the orders it broadcasts to itself.
	if err := os.WriteFile(filepath.Join(dir, CoordPubFile), []byte(pub+"\n"), 0o644); err != nil {
		return "", err
	}
	return pub, nil
}

// GenerateBoardKey writes this board's packet-signing key pair into dir and
// returns the public half as hex, for the sysop to send to their League
// Coordinator to put in the roster. Refuses to overwrite: a new key silently
// invalidates every packet this board sends until the roster catches up, which
// looks to the rest of the league like forgery.
func GenerateBoardKey(dir string) (string, error) {
	return generateKeyPair(dir, BoardKeyFile)
}

// generateKeyPair writes a new ed25519 private key into dir/name at 0600 and
// returns the public half as hex. Refuses to overwrite an existing key: for
// either pair, replacing one silently invalidates work already distributed —
// the Coordinator's orphans every board holding the old public half, and a
// board's makes its packets read as forgeries until the roster catches up.
func generateKeyPair(dir, name string) (string, error) {
	priv := filepath.Join(dir, name)
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
	w.BoardKey = readHexKey(filepath.Join(cfg.DataDir, BoardKeyFile), ed25519.PrivateKeySize)
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
