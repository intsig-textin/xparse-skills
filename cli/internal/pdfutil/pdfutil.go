// Package pdfutil provides local PDF utilities without requiring external APIs.
package pdfutil

import (
	"fmt"
	"strings"

	"github.com/ledongthuc/pdf"
)

// PageCount returns the number of pages in a PDF file.
func PageCount(filepath string) (int, error) {
	f, r, err := pdf.Open(filepath)
	if err != nil {
		return 0, fmt.Errorf("failed to open PDF: %w", err)
	}
	defer f.Close()

	return r.NumPage(), nil
}

// FirstPageText extracts text from the first page of a PDF.
// Returns empty string on failure (best-effort, not all PDFs support text extraction).
func FirstPageText(filepath string) string {
	f, r, err := pdf.Open(filepath)
	if err != nil {
		return ""
	}
	defer f.Close()

	if r.NumPage() < 1 {
		return ""
	}

	page := r.Page(1)
	text, err := page.GetPlainText(nil)
	if err != nil {
		return ""
	}

	// Limit to first 500 characters for doc-type heuristics
	if len(text) > 500 {
		text = text[:500]
	}

	return strings.TrimSpace(text)
}
