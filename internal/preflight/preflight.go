// Package preflight inspects deterministic local input facts before any API call.
package preflight

import (
	"bytes"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"gitlab.intsig.net/xparse/xparse-client/internal/pdfutil"
)

const (
	SourceLocal = "local"
	SourceURL   = "url"

	CodeFileNotFound        = "FILE_NOT_FOUND"
	CodeFileNotReadable     = "FILE_NOT_READABLE"
	CodeEmptyFile           = "EMPTY_FILE"
	CodeUnsupportedFileType = "UNSUPPORTED_FILE_TYPE"
	CodeInvalidPDF          = "INVALID_PDF"
	CodePasswordRequired    = "PASSWORD_REQUIRED"
	CodeInvalidURL          = "INVALID_URL"
)

// Spec contains only facts safe to send to the capability resolver.
type Spec struct {
	SourceType              string
	DetectedType            string
	SizeBytes               int64
	PageCount               int
	ServerPreflightRequired bool
	NormalizedPath          string
}

// Error is a deterministic preflight failure that does not require a service call.
type Error struct {
	Code    string
	Message string
	Cause   error
}

func (e *Error) Error() string {
	if e.Message != "" {
		return e.Code + ": " + e.Message
	}
	return e.Code
}

func (e *Error) Unwrap() error { return e.Cause }

// Inspect validates an HTTPS URL or reads local file metadata and a bounded header.
func Inspect(input string) (*Spec, error) {
	if strings.HasPrefix(input, "http://") || strings.HasPrefix(input, "https://") {
		return inspectURL(input)
	}
	return inspectLocalFile(input)
}

func inspectURL(input string) (*Spec, error) {
	parsed, err := url.ParseRequestURI(input)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" || parsed.User != nil {
		return nil, newError(CodeInvalidURL, "only public HTTPS URLs without embedded credentials are supported", err)
	}
	hostname := strings.ToLower(strings.TrimSuffix(parsed.Hostname(), "."))
	if hostname == "" || hostname == "localhost" || strings.HasSuffix(hostname, ".local") {
		return nil, newError(CodeInvalidURL, "local or empty URL hosts are not allowed", nil)
	}
	if ip := net.ParseIP(hostname); ip != nil &&
		(ip.IsLoopback() || ip.IsPrivate() || ip.IsUnspecified() ||
			ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return nil, newError(CodeInvalidURL, "local or private URL addresses are not allowed", nil)
	}
	return &Spec{
		SourceType:              SourceURL,
		ServerPreflightRequired: true,
	}, nil
}

func inspectLocalFile(input string) (*Spec, error) {
	normalizedPath, err := filepath.Abs(filepath.Clean(input))
	if err != nil {
		return nil, newError(CodeFileNotReadable, "cannot normalize the input path", err)
	}
	info, err := os.Lstat(normalizedPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil, newError(CodeFileNotFound, "input file does not exist", err)
	}
	if err != nil {
		return nil, newError(CodeFileNotReadable, "cannot inspect the input file", err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		info, err = os.Stat(normalizedPath)
		if errors.Is(err, os.ErrNotExist) {
			return nil, newError(CodeFileNotFound, "symbolic link target does not exist", err)
		}
		if err != nil {
			return nil, newError(CodeFileNotReadable, "cannot inspect the symbolic link target", err)
		}
	}
	if !info.Mode().IsRegular() {
		return nil, newError(CodeUnsupportedFileType, "input must be a regular file", nil)
	}
	if info.Size() == 0 {
		return nil, newError(CodeEmptyFile, "input file is empty", nil)
	}

	file, err := os.Open(normalizedPath)
	if err != nil {
		return nil, newError(CodeFileNotReadable, "input file cannot be opened read-only", err)
	}
	defer file.Close()
	header := make([]byte, 4096)
	read, err := io.ReadFull(file, header)
	if err != nil && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, newError(CodeFileNotReadable, "input file header cannot be read", err)
	}
	header = header[:read]
	detectedType := detectType(header, filepath.Ext(normalizedPath))
	if detectedType == "" {
		return nil, newError(CodeUnsupportedFileType, "file content is not a supported document type", nil)
	}

	spec := &Spec{
		SourceType:     SourceLocal,
		DetectedType:   detectedType,
		SizeBytes:      info.Size(),
		NormalizedPath: normalizedPath,
	}
	if detectedType == "pdf" {
		pageCount, pageErr := pdfutil.PageCount(normalizedPath)
		if pageErr != nil {
			message := strings.ToLower(pageErr.Error())
			if strings.Contains(message, "password") || strings.Contains(message, "encrypt") {
				return nil, newError(CodePasswordRequired, "PDF requires a password", pageErr)
			}
			return nil, newError(CodeInvalidPDF, "PDF structure cannot be read", pageErr)
		}
		if pageCount <= 0 {
			return nil, newError(CodeInvalidPDF, "PDF has no readable pages", nil)
		}
		spec.PageCount = pageCount
	} else if isImageType(detectedType) {
		spec.PageCount = 1
	}
	return spec, nil
}

func detectType(header []byte, extension string) string {
	ext := strings.TrimPrefix(strings.ToLower(extension), ".")
	switch {
	case bytes.HasPrefix(header, []byte("%PDF-")):
		return "pdf"
	case len(header) >= 3 && header[0] == 0xff && header[1] == 0xd8 && header[2] == 0xff:
		return "jpeg"
	case bytes.HasPrefix(header, []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}):
		return "png"
	case bytes.HasPrefix(header, []byte{'B', 'M'}):
		return "bmp"
	case bytes.HasPrefix(header, []byte{'I', 'I', 0x2a, 0x00}),
		bytes.HasPrefix(header, []byte{'M', 'M', 0x00, 0x2a}):
		return "tiff"
	case len(header) >= 12 && bytes.Equal(header[:4], []byte("RIFF")) &&
		bytes.Equal(header[8:12], []byte("WEBP")):
		return "webp"
	case bytes.HasPrefix(header, []byte("{\\rtf")):
		return "rtf"
	case looksLikeHTML(header):
		return "html"
	case bytes.HasPrefix(header, []byte{'P', 'K', 0x03, 0x04}):
		if isOneOf(ext, "docx", "pptx", "xlsx", "ofd") {
			return ext
		}
	case bytes.HasPrefix(header, []byte{0xd0, 0xcf, 0x11, 0xe0, 0xa1, 0xb1, 0x1a, 0xe1}):
		if isOneOf(ext, "doc", "ppt", "xls") {
			return ext
		}
	case utf8.Valid(header) && !bytes.ContainsRune(header, '\x00'):
		return "txt"
	}
	return ""
}

func looksLikeHTML(header []byte) bool {
	trimmed := strings.ToLower(strings.TrimSpace(string(header)))
	return strings.HasPrefix(trimmed, "<!doctype html") ||
		strings.HasPrefix(trimmed, "<html")
}

func isImageType(detectedType string) bool {
	return isOneOf(detectedType, "jpeg", "png", "bmp", "tiff", "webp")
}

func isOneOf(value string, candidates ...string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func newError(code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Cause: cause}
}
