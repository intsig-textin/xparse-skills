package cmd

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/exitcode"
)

// ── V1 parse flags ──

var (
	parseView                     string // json | markdown
	parseAPI                      string // free | paid
	parsePageRangeFlag            string // "1-5" or "1-2,5-10"
	parsePassword                 string
	parseIncludeHierarchy         string
	parseIncludeInlineObjects     string
	parseIncludeCharDetail        string
	parseIncludeImageData         string
	parseIncludeTableStructure    string
	parseIncludePages             string
	parseIncludeTitleTree         string
	parseTableView                string // html | markdown
	parseListFile                 string
	parseOutput                   string
)

var parseCmd = &cobra.Command{
	Use:   "parse <file-or-url>",
	Short: "Parse a document to Markdown or JSON",
	Long: `Parse documents using the Textin xParser API.

Supports: PDF, Images (png, jpg, bmp, tiff, webp), Doc(x), Ppt(x), Xls(x),
          HTML, TXT, OFD, RTF

Examples:
  # Basic usage — markdown to stdout (zero config, uses free API)
  xparse-cli parse report.pdf

  # View as JSON
  xparse-cli parse report.pdf --view json

  # Save to file
  xparse-cli parse report.pdf --output ./result/

  # Parse specific pages
  xparse-cli parse report.pdf --page-range "1-5"

  # Encrypted PDF
  xparse-cli parse secret.pdf --password mypassword

  # Batch from file list
  xparse-cli parse --list files.txt --output ./result/

  # Use paid API explicitly
  xparse-cli parse report.pdf --api paid`,
	RunE: runParse,
}

func init() {
	rootCmd.AddCommand(parseCmd)

	parseCmd.Flags().StringVar(&parseView, "view", "markdown", "Output view: markdown, json")
	parseCmd.Flags().StringVar(&parseAPI, "api", "", "API mode: free, paid (default: paid if key exists, else free)")
	parseCmd.Flags().StringVar(&parsePageRangeFlag, "page-range", "", `Page range, e.g. "1-5" or "1-2,5-10"`)
	parseCmd.Flags().StringVar(&parsePassword, "password", "", "Password for encrypted documents")
	parseCmd.Flags().StringVar(&parseIncludeHierarchy, "include-hierarchy", "", "Include element hierarchy and parent-child associations: true, false (default true)")
	parseCmd.Flags().Lookup("include-hierarchy").NoOptDefVal = "true"
	parseCmd.Flags().StringVar(&parseIncludeInlineObjects, "include-inline-objects", "", "Include inline objects (formulas, handwriting, checkboxes, embedded images): true, false (default true)")
	parseCmd.Flags().Lookup("include-inline-objects").NoOptDefVal = "true"
	parseCmd.Flags().StringVar(&parseIncludeCharDetail, "include-char-details", "", "Include character-level details: true, false (default false)")
	parseCmd.Flags().Lookup("include-char-details").NoOptDefVal = "true"
	parseCmd.Flags().StringVar(&parseIncludeImageData, "include-image-data", "", "Include image URLs and Base64 data: true, false (default true)")
	parseCmd.Flags().Lookup("include-image-data").NoOptDefVal = "true"
	parseCmd.Flags().StringVar(&parseIncludeTableStructure, "include-table-structure", "", "Include structured table cell data with coordinates: true, false (default true)")
	parseCmd.Flags().Lookup("include-table-structure").NoOptDefVal = "true"
	parseCmd.Flags().StringVar(&parseIncludePages, "include-pages", "", "Include per-page metadata and preview images: true, false (default true)")
	parseCmd.Flags().Lookup("include-pages").NoOptDefVal = "true"
	parseCmd.Flags().StringVar(&parseIncludeTitleTree, "include-title-tree", "", "Include hierarchical document outline: true, false (default true)")
	parseCmd.Flags().Lookup("include-title-tree").NoOptDefVal = "true"
	parseCmd.Flags().StringVar(&parseTableView, "table-view", "html", "Table output format: html, markdown (default html)")
	parseCmd.Flags().StringVar(&parseListFile, "list", "", "Read input list from file (one path per line); requires --output")
	parseCmd.Flags().StringVar(&parseOutput, "output", "", "Output file path or directory; omit for stdout")
}

// ── entry point ──

func runParse(cmd *cobra.Command, args []string) error {
	// Validate --view
	if parseView != "markdown" && parseView != "json" {
		return usageErr(exitcode.ErrInvalidView,
			"[fix] use --view markdown or --view json")
	}

	// Validate --table-view
	switch parseTableView {
	case "html", "markdown":
	default:
		return usageErr("invalid --table-view value, must be 'html' or 'markdown'",
			"[fix] use --table-view html or --table-view markdown")
	}

	// Validate --api
	var apiMode APIMode
	switch parseAPI {
	case "", "free", "paid":
		apiMode = APIMode(parseAPI)
	default:
		return usageErr(exitcode.ErrInvalidAPI,
			"[fix] use --api free or --api paid")
	}

	// Validate capability flags
	includeHierarchy, err := parseBoolFlag(parseIncludeHierarchy, "include-hierarchy", true)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --include-hierarchy, --include-hierarchy=true, or --include-hierarchy=false")
	}
	includeInlineObjects, err := parseBoolFlag(parseIncludeInlineObjects, "include-inline-objects", true)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --include-inline-objects, --include-inline-objects=true, or --include-inline-objects=false")
	}
	includeCharDetails, err := parseBoolFlag(parseIncludeCharDetail, "include-char-details", false)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --include-char-details, --include-char-details=true, or --include-char-details=false")
	}
	includeImageData, err := parseBoolFlag(parseIncludeImageData, "include-image-data", true)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --include-image-data, --include-image-data=true, or --include-image-data=false")
	}
	includeTableStructure, err := parseBoolFlag(parseIncludeTableStructure, "include-table-structure", true)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --include-table-structure, --include-table-structure=true, or --include-table-structure=false")
	}
	includePages, err := parseBoolFlag(parseIncludePages, "include-pages", true)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --include-pages, --include-pages=true, or --include-pages=false")
	}
	includeTitleTree, err := parseBoolFlag(parseIncludeTitleTree, "include-title-tree", true)
	if err != nil {
		return usageErr(err.Error(), "[fix] use --include-title-tree, --include-title-tree=true, or --include-title-tree=false")
	}

	// Collect input sources
	sources, err := collectSources(args, parseListFile)
	if err != nil {
		return usageErr(exitcode.ErrOpenListFile,
			"[ask human] check that the --list file "+parseListFile+" exists and is readable")
	}

	if len(sources) == 0 {
		return usageErr(exitcode.ErrNoInput,
			"[fix] provide a file path, URL, or use --list <file>")
	}

	// Validate file existence early
	for _, src := range sources {
		if !isURL(src) {
			if _, err := os.Stat(src); os.IsNotExist(err) {
				return generalErr(exitcode.ErrFileNotFound+": "+src,
					"[ask human] verify "+src+" exists and is accessible")
			}
		}
	}

	// --list requires --output
	if parseListFile != "" && parseOutput == "" {
		return usageErr(exitcode.ErrListRequiresOut,
			"[fix] add --output <directory> when using --list")
	}

	// Batch mode requires --output to be a directory
	if len(sources) > 1 && parseOutput == "" {
		return usageErr(exitcode.ErrMultiRequiresOut,
			"[fix] add --output <directory> for batch processing")
	}

	// Validate output directory exists and is writable before calling the API
	if parseOutput != "" {
		if err := validateOutputDir(parseOutput); err != nil {
			return err
		}
	}

	// Resolve credentials
	credSrc, err := config.ResolveCredentials(cmd)
	if err != nil {
		return generalErr(exitcode.ErrCredentialsConfig,
			"[ask human] run xparse-cli auth or set XPARSE_APP_ID and XPARSE_SECRET_CODE env vars")
	}

	// Determine API mode
	isFree := resolveAPIMode(apiMode, credSrc)

	// --api paid requires credentials
	if !isFree && (credSrc.AppID == "" || credSrc.SecretCode == "") {
		return usageErr(exitcode.ErrPaidNoCreds,
			"[ask human] run xparse-cli auth; or [fix] re-run with --api free")
	}

	client := newXParserClient(cmd, credSrc, isFree)

	opts := &ParseOptions{
		PageRange:             parsePageRangeFlag,
		Password:              parsePassword,
		IncludeHierarchy:      includeHierarchy,
		IncludeInlineObjects:  includeInlineObjects,
		IncludeCharDetails:    includeCharDetails,
		IncludeImageData:      includeImageData,
		IncludeTableStructure: includeTableStructure,
		IncludePages:          includePages,
		IncludeTitleTree:      includeTitleTree,
		TableView:             parseTableView,
	}

	if len(sources) == 1 {
		return runSingleParse(client, sources[0], opts)
	}
	return runBatchParse(client, sources, opts)
}

// ── single file/url ──

func runSingleParse(client *XParserClient, source string, opts *ParseOptions) error {
	var resp *ParseResponse
	var err error

	if isURL(source) {
		resp, err = client.ParseURL(source, opts)
	} else {
		resp, err = client.ParseFile(source, opts)
	}
	if err != nil {
		return generalErr(exitcode.ErrNetworkRequest,
			"[retry] with --verbose for "+source+"; max 2 retries, 2s backoff")
	}

	if resp.Code != 200 {
		return apiErr(resp.Code, resp.Message, resp.XRequestID)
	}

	if !resp.HasResult() {
		return generalErr(exitcode.ErrNoResultData,
			"[retry] once for "+source+"; if persists, try a different file or format")
	}

	return outputResult(resp, source)
}

// ── batch ──

func runBatchParse(client *XParserClient, sources []string, opts *ParseOptions) error {
	info, err := os.Stat(parseOutput)
	if err != nil || !info.IsDir() {
		return generalErr(exitcode.ErrOutputDirNotExist+": "+parseOutput,
			"[ask human] create the directory with mkdir -p "+parseOutput)
	}

	type fileError struct {
		source     string
		reason     string
		suggestion string
	}
	var failures []fileError

	// Build base command for retry suggestions
	retryBase := fmt.Sprintf("xparse-cli parse %%s --output %s", parseOutput)
	if parseView != "markdown" {
		retryBase += " --view " + parseView
	}
	if parseAPI != "" {
		retryBase += " --api " + parseAPI
	}
	if opts.PageRange != "" {
		retryBase += " --page-range " + opts.PageRange
	}
	if opts.Password != "" {
		retryBase += " --password " + opts.Password
	}
	if !opts.IncludeHierarchy {
		retryBase += " --include-hierarchy=false"
	}
	if !opts.IncludeInlineObjects {
		retryBase += " --include-inline-objects=false"
	}
	if opts.IncludeCharDetails {
		retryBase += " --include-char-details"
	}
	if !opts.IncludeImageData {
		retryBase += " --include-image-data=false"
	}
	if !opts.IncludeTableStructure {
		retryBase += " --include-table-structure=false"
	}
	if !opts.IncludePages {
		retryBase += " --include-pages=false"
	}
	if !opts.IncludeTitleTree {
		retryBase += " --include-title-tree=false"
	}
	if opts.TableView != "html" {
		retryBase += " --table-view " + opts.TableView
	}

	for _, source := range sources {
		var resp *ParseResponse
		var err error

		retryCmd := fmt.Sprintf(retryBase, `"`+source+`"`)

		if isURL(source) {
			resp, err = client.ParseURL(source, opts)
		} else {
			resp, err = client.ParseFile(source, opts)
		}

		if err != nil {
			failures = append(failures, fileError{source, exitcode.ErrNetworkRequest,
				fmt.Sprintf("[retry] %s --verbose; max 2 retries, 2s backoff", retryCmd)})
			continue
		}

		if resp.Code != 200 {
			info := exitcode.FromAPICode(resp.Code, resp.Message, resp.XRequestID)
			failures = append(failures, fileError{source,
				fmt.Sprintf("%d：%s", info.APICode, info.Message),
				batchSuggestion(info, source, retryCmd)})
			continue
		}

		if !resp.HasResult() {
			failures = append(failures, fileError{source, exitcode.ErrNoResultData,
				fmt.Sprintf("[retry] %s", retryCmd)})
			continue
		}

		if _, err := saveResult(resp, source, parseOutput); err != nil {
			failures = append(failures, fileError{source, exitcode.ErrSaveResult,
				fmt.Sprintf("[ask human] check disk space and write permissions for %s", parseOutput)})
			continue
		}
	}

	if len(failures) > 0 {
		fmt.Fprintf(os.Stderr, "%s (%d/%d failed)\n", exitcode.ErrBatchPartial, len(failures), len(sources))
		for i, f := range failures {
			fmt.Fprintf(os.Stderr, "> [%d] %s: %s\n", i+1, f.source, f.reason)
			fmt.Fprintf(os.Stderr, ">     %s\n", f.suggestion)
		}
		return &exitError{code: exitcode.GeneralError, msg: exitcode.ErrBatchPartial}
	}
	return nil
}

// ── output ──

func outputResult(resp *ParseResponse, source string) error {
	if parseOutput == "" {
		// stdout mode
		switch parseView {
		case "json":
			data, _ := json.MarshalIndent(resp, "", "  ")
			fmt.Println(string(data))
		default: // "markdown"
			fmt.Print(resp.GetMarkdown())
		}
		return nil
	}

	// file mode
	if _, err := saveResult(resp, source, parseOutput); err != nil {
		return generalErr(exitcode.ErrSaveResult,
			"[ask human] check disk space and write permissions for "+parseOutput)
	}
	return nil
}

func saveResult(resp *ParseResponse, source string, outputPath string) (string, error) {
	dir, base := resolveOutputTarget(outputPath, source)

	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", fmt.Errorf("%s: %s", exitcode.ErrOutputDirNotExist, dir)
	}

	ext := ".md"
	if parseView == "json" {
		ext = ".json"
	}

	outPath := filepath.Join(dir, base+ext)

	switch parseView {
	case "json":
		data, err := json.MarshalIndent(resp, "", "  ")
		if err != nil {
			return "", fmt.Errorf("failed to marshal json: %w", err)
		}
		if err := os.WriteFile(outPath, data, 0o644); err != nil {
			return "", fmt.Errorf("failed to save json: %w", err)
		}
	default: // "markdown"
		if err := os.WriteFile(outPath, []byte(resp.GetMarkdown()), 0o644); err != nil {
			return "", fmt.Errorf("failed to save markdown: %w", err)
		}
	}

	return outPath, nil
}


// ── error helpers ──

// apiErr outputs "api_code：message" to stderr and returns exit code 3.
// Second line: AI-actionable suggestion.
// Third line (if applicable): contact support with request_id.
func apiErr(apiCode int, message string, xRequestID string) *exitError {
	info := exitcode.FromAPICode(apiCode, message, xRequestID)
	fmt.Fprintf(os.Stderr, "%d：%s\n", info.APICode, info.Message)
	if info.Suggestion != "" {
		fmt.Fprintf(os.Stderr, "> %s\n", info.Suggestion)
	}
	if info.ContactSupport && xRequestID != "" {
		fmt.Fprintf(os.Stderr, "  (request_id: %s, contact Textin support if unresolved)\n", xRequestID)
	}
	return &exitError{code: exitcode.APIError, msg: info.Message}
}

// batchSuggestion generates a context-specific suggestion for a per-file API error in batch mode.
func batchSuggestion(info *exitcode.APIErrorInfo, source string, retryCmd string) string {
	switch {
	case info.Retryable:
		return fmt.Sprintf("[retry] %s", retryCmd)
	case info.APICode == 40423:
		return fmt.Sprintf("[ask human] provide the correct password for %s; [retry] %s --password <password>", filepath.Base(source), retryCmd)
	case info.APICode == 40424:
		return fmt.Sprintf("[fix] adjust --page-range for %s; [retry] %s", filepath.Base(source), retryCmd)
	case info.APICode == 40003 || info.APICode == 40101 || info.APICode == 40102:
		return fmt.Sprintf("[fallback] %s --api free", retryCmd)
	case info.APICode == 40307:
		return fmt.Sprintf("[fallback] %s --api paid", retryCmd)
	default:
		return info.Suggestion
	}
}

// validateOutputDir checks that the output path exists and is writable before
// any API call is made, so the user gets an early error instead of losing work.
func validateOutputDir(outputPath string) *exitError {
	info, err := os.Stat(outputPath)
	if os.IsNotExist(err) {
		return generalErr(exitcode.ErrOutputDirNotExist+": "+outputPath,
			"[ask human] create the directory first: mkdir -p "+outputPath)
	}
	if err != nil {
		return generalErr(exitcode.ErrOutputDirNotExist+": "+outputPath,
			"[ask human] check that "+outputPath+" is accessible")
	}
	if !info.IsDir() {
		return usageErr("output path is not a directory: "+outputPath,
			"[fix] specify a directory path for --output, not a file")
	}

	// Probe write permission with a temp file
	probe, err := os.CreateTemp(outputPath, ".xparse-write-check-*")
	if err != nil {
		return generalErr("no write permission on output directory: "+outputPath,
			"[ask human] fix permissions: chmod u+w "+outputPath)
	}
	probe.Close()
	os.Remove(probe.Name())

	return nil
}

// usageErr outputs plain text to stderr and returns exit code 2.
// Second line: AI-actionable suggestion with context.
func usageErr(message string, suggestion string) *exitError {
	fmt.Fprintln(os.Stderr, message)
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "> %s\n", suggestion)
	}
	return &exitError{code: exitcode.UsageError, msg: message}
}

// generalErr outputs plain text to stderr and returns exit code 1.
// Second line: AI-actionable suggestion with context.
func generalErr(message string, suggestion string) *exitError {
	fmt.Fprintln(os.Stderr, message)
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "> %s\n", suggestion)
	}
	return &exitError{code: exitcode.GeneralError, msg: message}
}

// exitError carries a process exit code alongside the error message.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }

// ExitCode returns the process exit code for this error.
func (e *exitError) ExitCode() int { return e.code }

// parseBoolFlag parses a bool-string flag value ("", "true", "false") into a bool.
// An empty string returns the provided defaultValue.
func parseBoolFlag(value, flagName string, defaultValue bool) (bool, error) {
	switch strings.ToLower(value) {
	case "":
		return defaultValue, nil
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("invalid --%s value, must be 'true' or 'false'", flagName)
	}
}

// ── shared helpers ──

func isURL(s string) bool {
	return strings.HasPrefix(s, "http://") || strings.HasPrefix(s, "https://")
}

func collectSources(args []string, listFile string) ([]string, error) {
	var sources []string
	sources = append(sources, args...)

	if listFile != "" {
		f, err := os.Open(listFile)
		if err != nil {
			return nil, fmt.Errorf("failed to open list file: %w", err)
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				sources = append(sources, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("failed to read list file: %w", err)
		}
	}

	return sources, nil
}

func baseNameNoExt(source string) string {
	base := filepath.Base(source)
	ext := filepath.Ext(base)
	if ext != "" {
		return base[:len(base)-len(ext)]
	}
	return base
}

func resolveOutputTarget(outputPath, source string) (dir, base string) {
	dir = outputPath
	base = baseNameNoExt(source)

	// If outputPath is an existing file or looks like a file path, use it directly
	info, err := os.Stat(outputPath)
	if err == nil && !info.IsDir() {
		return filepath.Dir(outputPath), baseNameNoExt(outputPath)
	}

	if outputPathLooksLikeFile(outputPath) {
		return filepath.Dir(outputPath), baseNameNoExt(outputPath)
	}

	return dir, base
}

func outputPathLooksLikeFile(outputPath string) bool {
	if outputPath == "" || hasTrailingPathSeparator(outputPath) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(outputPath))
	return ext == ".md" || ext == ".json" || ext == ".markdown"
}

func hasTrailingPathSeparator(path string) bool {
	return strings.HasSuffix(path, "/") || strings.HasSuffix(path, "\\")
}
