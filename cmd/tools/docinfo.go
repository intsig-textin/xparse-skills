package tools

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/cache"
	"gitlab.intsig.net/xparse/xparse-client/internal/exitcode"
	"gitlab.intsig.net/xparse/xparse-client/internal/models"
	"gitlab.intsig.net/xparse/xparse-client/internal/pdfutil"
)

var getDocInfoCmd = &cobra.Command{
	Use:   "get_doc_info <filepath>",
	Short: "Get document basic information (local, zero API calls)",
	Long: `Extract document metadata using local PDF library only.
Returns doc_id, page_count, doc_type without any API calls.
This is always the first step before using other primitives.

Examples:
  xparse-cli get_doc_info report.pdf
  xparse-cli get_doc_info /path/to/contract.pdf`,
	Args: cobra.ExactArgs(1),
	RunE: runGetDocInfo,
}

func runGetDocInfo(cmd *cobra.Command, args []string) error {
	inputPath := args[0]

	// Resolve to absolute path
	absPath, err := filepath.Abs(inputPath)
	if err != nil {
		return generalErr("failed to resolve path: "+inputPath,
			"[fix] provide a valid file path")
	}

	// Check file exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return generalErr(exitcode.ErrFileNotFound+": "+absPath,
			"[ask human] verify the file exists and is accessible")
	}

	// Compute doc_id
	docID := cache.DocID(absPath)

	// Check disk cache first
	if info, err := cache.GetDocInfo(docID); err == nil {
		return outputJSON(info)
	}

	// Get page count from local PDF library
	pageCount, err := pdfutil.PageCount(absPath)
	if err != nil {
		return generalErr("failed to read PDF page count: "+err.Error(),
			"[retry] ensure the file is a valid PDF")
	}

	// Infer doc_type
	filename := filepath.Base(absPath)
	docType := inferDocType(filename, absPath)

	info := &models.DocInfo{
		DocID:     docID,
		Filepath:  absPath,
		Filename:  filename,
		PageCount: pageCount,
		DocType:   docType,
	}

	// Persist to disk
	if err := cache.SetDocInfo(docID, info); err != nil {
		return generalErr("failed to save doc info: "+err.Error(), "")
	}

	return outputJSON(info)
}

// inferDocType determines document type from filename and first page content.
func inferDocType(filename, fpath string) string {
	lower := strings.ToLower(filename)

	patterns := []struct {
		re      *regexp.Regexp
		docType string
	}{
		{regexp.MustCompile(`合同|contract|协议|agreement`), "contract"},
		{regexp.MustCompile(`年报|报告|report|annual`), "report"},
		{regexp.MustCompile(`手册|manual|规范|spec|指南|guide`), "manual"},
		{regexp.MustCompile(`发票|invoice|收据|receipt`), "invoice"},
		{regexp.MustCompile(`演示|presentation|ppt`), "presentation"},
	}

	for _, p := range patterns {
		if p.re.MatchString(lower) {
			return p.docType
		}
	}

	// Try first page text
	firstPageText := strings.ToLower(pdfutil.FirstPageText(fpath))
	if firstPageText != "" {
		for _, p := range patterns {
			if p.re.MatchString(firstPageText) {
				return p.docType
			}
		}
	}

	return "other"
}
