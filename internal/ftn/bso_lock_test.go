package ftn

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAcquireBSYWritesOwnerAndHoldsFileLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "00e50064.bsy")
	held, err := acquireBSY(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if held != nil {
			_ = releaseBSY(path, held)
		}
	}()
	if _, err := held.file.Seek(0, 0); err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(held.file)
	if err != nil {
		t.Fatal(err)
	}
	if line := strings.TrimSpace(string(data)); line != fmt.Sprintf("%s%d", bsyOwnerPrefix, os.Getpid()) || len(line) >= 70 {
		t.Fatalf("BSY owner line = %q", line)
	}
	old := time.Now().Add(-bsyStaleAge - time.Minute)
	if err := os.Chtimes(path, old, old); err != nil {
		t.Fatal(err)
	}
	if recovered, err := recoverOwnBSY(path); err != nil || recovered {
		t.Fatalf("active old BSY recovery = %t, %v", recovered, err)
	}
	if _, err := acquireBSY(path); !errors.Is(err, errPeerBusy) {
		t.Fatalf("second acquisition = %v, want peer busy", err)
	}
	if err := releaseBSY(path, held); err != nil {
		t.Fatal(err)
	}
	held = nil
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("released BSY remains: %v", err)
	}
}

func TestAcquireBSYRecoversOnlyOldUnlockedOwnerFile(t *testing.T) {
	for _, tc := range []struct {
		name    string
		body    string
		age     time.Duration
		recover bool
	}{
		{"our old semaphore", bsyOwnerPrefix + "1234\n", bsyStaleAge + time.Minute, true},
		{"our young semaphore", bsyOwnerPrefix + "1234\n", time.Minute, false},
		{"foreign old semaphore", "binkd pid=1234\n", bsyStaleAge + time.Minute, false},
		{"legacy empty semaphore", "", bsyStaleAge + time.Minute, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "00e50064.bsy")
			if err := os.WriteFile(path, []byte(tc.body), 0o644); err != nil {
				t.Fatal(err)
			}
			stamp := time.Now().Add(-tc.age)
			if err := os.Chtimes(path, stamp, stamp); err != nil {
				t.Fatal(err)
			}
			held, err := acquireBSY(path)
			if tc.recover {
				if err != nil {
					t.Fatalf("stale owner file was not recovered: %v", err)
				}
				if err := releaseBSY(path, held); err != nil {
					t.Fatal(err)
				}
				return
			}
			if !errors.Is(err, errPeerBusy) {
				t.Fatalf("acquire = %v, want peer busy", err)
			}
			if got, err := os.ReadFile(path); err != nil || string(got) != tc.body {
				t.Fatalf("busy file changed to %q: %v", got, err)
			}
		})
	}
}
