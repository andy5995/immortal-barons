package ftn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	counterFile = "ftn-counter.json"
	counterSpan = uint64(36 * 36 * 36 * 36)
)

type counterState struct {
	Next uint64 `json:"next"`
}

// nextAlias reserves the counter before returning. The caller still uses
// exclusive publication because an old cycle's file may remain in a mailer
// queue. skipped values after a crash are harmless.
func nextAlias(dataDir, destinationDir string, league, transmitter int) (string, bool, error) {
	if league < 0 || league > 999 || transmitter < 1 || transmitter > 999 {
		return "", false, fmt.Errorf("8.3 FTN aliases require a league number in 0..999 and a transmitting node number in 1..999")
	}
	path := filepath.Join(dataDir, counterFile)
	var state counterState
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &state); err != nil {
			return "", false, fmt.Errorf("read %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return "", false, err
	}
	for attempts := uint64(0); attempts < counterSpan; attempts++ {
		value := state.Next
		state.Next++
		if err := writeCounterAtomic(path, state); err != nil {
			return "", false, err
		}
		namespace := league*1000 + transmitter
		name := paddedBase36(uint64(namespace), 4) + paddedBase36(value%counterSpan, 4) + ".BRP"
		if _, err := os.Lstat(filepath.Join(destinationDir, name)); os.IsNotExist(err) {
			return name, value != 0 && value%counterSpan == 0, nil
		} else if err != nil {
			return "", false, err
		}
	}
	return "", false, fmt.Errorf("all %d FTN attachment aliases are still present in %s", counterSpan, destinationDir)
}

func paddedBase36(value uint64, width int) string {
	s := strings.ToUpper(strconv.FormatUint(value, 36))
	if len(s) < width {
		s = strings.Repeat("0", width-len(s)) + s
	}
	return s
}

func writeCounterAtomic(path string, state counterState) error {
	data, err := json.Marshal(state)
	if err != nil {
		return err
	}
	return replaceFileAtomic(path, data, 0o644)
}
