package store

import (
	"errors"
	"testing"
)

func TestLockIsExclusive(t *testing.T) {
	cfg := cfgIn(t.TempDir())
	l1, err := Lock(cfg, false)
	if err != nil {
		t.Fatalf("first lock should succeed: %v", err)
	}
	if _, err := Lock(cfg, false); !errors.Is(err, ErrBusy) {
		t.Errorf("second non-blocking lock should be ErrBusy, got %v", err)
	}
	if err := l1.Release(); err != nil {
		t.Fatalf("release: %v", err)
	}
	l2, err := Lock(cfg, false)
	if err != nil {
		t.Fatalf("lock after release should succeed: %v", err)
	}
	l2.Release()
}
