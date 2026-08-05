package telemetry

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

func withFileLock(path string, action func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	lock := flock.New(path)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	locked, err := lock.TryLockContext(ctx, 5*time.Millisecond)
	if err != nil {
		return err
	}
	if !locked {
		return context.DeadlineExceeded
	}
	_ = os.Chmod(path, 0o600)
	defer func() { _ = lock.Unlock() }()
	return action()
}
