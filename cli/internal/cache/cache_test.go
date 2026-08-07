package cache

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCleanAllPreservesTelemetryAndCredentials(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	cacheFile := filepath.Join(CacheDir(), "document.json")
	docInfoFile := filepath.Join(docInfoDir(), "document.json")
	telemetryFile := filepath.Join(BaseDir(), "profiles", "workbuddy", "telemetry", "outbox.json")
	credentialFile := filepath.Join(BaseDir(), "profiles", "workbuddy", "oauth-token.json")
	for _, path := range []string{cacheFile, docInfoFile, telemetryFile, credentialFile} {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("test"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	if err := CleanAll(); err != nil {
		t.Fatal(err)
	}
	for _, removed := range []string{cacheFile, docInfoFile} {
		if _, err := os.Stat(removed); !os.IsNotExist(err) {
			t.Fatalf("cached file still exists: %s", removed)
		}
	}
	for _, preserved := range []string{telemetryFile, credentialFile} {
		if _, err := os.Stat(preserved); err != nil {
			t.Fatalf("private state was deleted: %s: %v", preserved, err)
		}
	}
}
