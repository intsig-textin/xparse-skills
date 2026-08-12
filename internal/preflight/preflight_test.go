package preflight

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestInspectLocalFileDetectsPNGWithoutExtension(t *testing.T) {
	path := filepath.Join(t.TempDir(), "document.bin")
	data := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0x00}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	spec, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if spec.SourceType != SourceLocal || spec.DetectedType != "png" ||
		spec.SizeBytes != int64(len(data)) || spec.PageCount != 1 {
		t.Fatalf("unexpected spec: %#v", spec)
	}
}

func TestInspectLocalFileRejectsDeterministicFailures(t *testing.T) {
	tests := []struct {
		name string
		make func(t *testing.T) string
		code string
	}{
		{
			name: "missing",
			make: func(t *testing.T) string {
				return filepath.Join(t.TempDir(), "missing.pdf")
			},
			code: CodeFileNotFound,
		},
		{
			name: "directory",
			make: func(t *testing.T) string { return t.TempDir() },
			code: CodeUnsupportedFileType,
		},
		{
			name: "empty",
			make: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "empty.pdf")
				if err := os.WriteFile(path, nil, 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
			code: CodeEmptyFile,
		},
		{
			name: "unsupported binary",
			make: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "payload.bin")
				if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
			code: CodeUnsupportedFileType,
		},
		{
			name: "invalid pdf",
			make: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "broken.pdf")
				if err := os.WriteFile(path, []byte("%PDF-not-valid"), 0o600); err != nil {
					t.Fatal(err)
				}
				return path
			},
			code: CodeInvalidPDF,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Inspect(tt.make(t))
			var preflightErr *Error
			if !errors.As(err, &preflightErr) || preflightErr.Code != tt.code {
				t.Fatalf("error = %#v, want code %s", err, tt.code)
			}
		})
	}
}

func TestInspectURLSecurityBoundary(t *testing.T) {
	for _, input := range []string{
		"http://example.com/report.pdf",
		"https://localhost/report.pdf",
		"https://127.0.0.1/report.pdf",
		"https://192.168.1.10/report.pdf",
		"https://user:secret@example.com/report.pdf",
	} {
		t.Run(input, func(t *testing.T) {
			_, err := Inspect(input)
			var preflightErr *Error
			if !errors.As(err, &preflightErr) || preflightErr.Code != CodeInvalidURL {
				t.Fatalf("error = %#v, want %s", err, CodeInvalidURL)
			}
		})
	}

	spec, err := Inspect("https://documents.example.com/report.pdf?token=private")
	if err != nil {
		t.Fatal(err)
	}
	if spec.SourceType != SourceURL || !spec.ServerPreflightRequired {
		t.Fatalf("unexpected URL spec: %#v", spec)
	}
}
