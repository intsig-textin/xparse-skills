package telemetry

import (
	"crypto/sha256"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"gitlab.intsig.net/xparse/xparse-client/internal/cache"
)

func SummarizeSource(source string) InputSummary {
	if parsed, err := url.Parse(source); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		hash := sha256.Sum256([]byte(source))
		return InputSummary{
			Kind: "remote_document", DocumentRef: fmt.Sprintf("%x", hash[:6]),
			Ext: safeExtension(parsed.Path),
		}
	}
	absPath, err := filepath.Abs(source)
	if err != nil {
		return InputSummary{Kind: "local_document"}
	}
	summary := InputSummary{
		Kind: "local_document", DocumentRef: cache.DocID(absPath), Ext: safeExtension(absPath),
	}
	if info, err := os.Stat(absPath); err == nil {
		summary.SizeBytes = info.Size()
	}
	return summary
}

func SummarizeDocument(docID string) InputSummary {
	summary := InputSummary{Kind: "cached_document", DocumentRef: docID}
	info, err := cache.GetDocInfo(docID)
	if err != nil {
		return summary
	}
	summary.Ext = safeExtension(info.Filename)
	if fileInfo, err := os.Stat(info.Filepath); err == nil {
		summary.SizeBytes = fileInfo.Size()
	}
	return summary
}

func safeExtension(path string) string {
	extension := strings.ToLower(filepath.Ext(path))
	if len(extension) > 16 {
		return ""
	}
	return extension
}
