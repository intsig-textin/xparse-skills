package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.intsig.net/xparse/xparse-client/internal/exitcode"
)

func TestPreflightSourcesReturnsLocalErrorBeforeAPI(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.pdf")
	_, err := preflightSources([]string{missing})
	if err == nil || err.ExitCode() != exitcode.GeneralError ||
		!strings.Contains(err.Error(), "FILE_NOT_FOUND") {
		t.Fatalf("error = %#v", err)
	}
}

func TestPreflightSourcesReturnsSpecsForValidInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "image.data")
	if err := os.WriteFile(
		path,
		[]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00},
		0o600,
	); err != nil {
		t.Fatal(err)
	}

	specs, err := preflightSources([]string{
		path,
		"https://documents.example.com/report.pdf",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(specs) != 2 || specs[0].DetectedType != "png" ||
		!specs[1].ServerPreflightRequired {
		t.Fatalf("specs = %#v", specs)
	}
}
