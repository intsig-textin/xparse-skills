package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
)

// API endpoints.
const (
	paidAPIBaseURL   = "https://api.textin.com"
	paidParseAPIPath = "/api/v1/xparse/parse/sync"

	freeAPIBaseURL   = "https://api.textin.com"
	freeParseAPIPath = "/api/v1/agent/parse/sync"
)

// APIMode represents free vs paid API selection.
type APIMode string

const (
	APIModeAuto APIMode = "" // auto: paid if key exists, else free
	APIModeFree APIMode = "free"
	APIModePaid APIMode = "paid"
)

// XParserClient wraps HTTP calls to the Textin xParser API.
type XParserClient struct {
	AppID      string
	SecretCode string
	BaseURL    string
	ParsePath  string
	IsFreeAPI  bool
	HTTPClient *http.Client
}

// ── Request config structures (multipart/form-data "config" field) ──

// ParseRequestConfig is the JSON config sent as the "config" form field.
type ParseRequestConfig struct {
	Document     *DocumentConfig `json:"document,omitempty"`
	Capabilities *Capabilities   `json:"capabilities"`
	Scope        *Scope          `json:"scope,omitempty"`
}

// DocumentConfig holds document-level settings.
type DocumentConfig struct {
	Password string `json:"password,omitempty"`
}

// Capabilities controls what the API returns.
type Capabilities struct {
	IncludeHierarchy      bool   `json:"include_hierarchy"`
	IncludeInlineObjects  bool   `json:"include_inline_objects"`
	IncludeCharDetails    bool   `json:"include_char_details"`
	IncludeImageData      bool   `json:"include_image_data"`
	IncludeTableStructure bool   `json:"include_table_structure"`
	Pages                 bool   `json:"pages"`
	TitleTree             bool   `json:"title_tree"`
	TableView             string `json:"table_view"`
}

// Scope controls the processing range.
type Scope struct {
	PageRange string `json:"page_range,omitempty"`
}

// ── Response structures ──

// ParseResponse is the top-level JSON response from the xParser API.
type ParseResponse struct {
	Code       int        `json:"code"`
	Message    string     `json:"message"`
	XRequestID string     `json:"x_request_id,omitempty"`
	Data       *ParseData `json:"data,omitempty"`
}

// GetMarkdown returns the markdown content from the response.
func (r *ParseResponse) GetMarkdown() string {
	if r.Data != nil {
		return r.Data.Markdown
	}
	return ""
}

// HasResult returns true if Data is present.
func (r *ParseResponse) HasResult() bool {
	return r.Data != nil
}

// GetSuccessCount returns the number of successfully parsed pages.
func (r *ParseResponse) GetSuccessCount() int {
	if r.Data != nil {
		return r.Data.SuccessCount
	}
	return 0
}

// GetPageCount returns the total number of pages.
func (r *ParseResponse) GetPageCount() int {
	if r.Data != nil && r.Data.Metadata != nil {
		return r.Data.Metadata.PageCount
	}
	return 0
}

// GetDurationMs returns the engine duration in milliseconds.
func (r *ParseResponse) GetDurationMs() float64 {
	if r.Data != nil && r.Data.Summary != nil {
		return r.Data.Summary.DurationMs
	}
	return 0
}

// ParseData holds the parsed output from the xParser API.
type ParseData struct {
	SchemaVersion string          `json:"schema_version"`
	FileID        string          `json:"file_id"`
	JobID         string          `json:"job_id"`
	SuccessCount  int             `json:"success_count"`
	Metadata      *ParseMetadata  `json:"metadata,omitempty"`
	Markdown      string          `json:"markdown"`
	Elements      json.RawMessage `json:"elements,omitempty"`
	TitleTree     json.RawMessage `json:"title_tree,omitempty"`
	Pages         json.RawMessage `json:"pages,omitempty"`
	Summary       *Summary        `json:"summary,omitempty"`
}

// ParseMetadata holds document metadata from the API.
type ParseMetadata struct {
	Filename  string `json:"filename"`
	Filetype  string `json:"filetype"`
	PageCount int    `json:"page_count"`
}

// Summary holds processing statistics.
type Summary struct {
	DurationMs float64 `json:"duration_ms"`
}

// ParseOptions holds V1 parse parameters.
type ParseOptions struct {
	PageRange          string // e.g. "1-5" or "1-2,5-10"
	Password           string
	IncludeCharDetails bool
}

// buildConfig constructs the JSON config string for the multipart "config" field.
// CLI defaults override API defaults to provide the richest output without extra flags.
func (o *ParseOptions) buildConfig() string {
	cfg := ParseRequestConfig{
		Capabilities: &Capabilities{
			IncludeHierarchy:      true,                 // CLI default: true  (API default: true)
			IncludeInlineObjects:  true,                 // CLI default: true  (API default: false)
			IncludeCharDetails:    o.IncludeCharDetails, // CLI default: false (API default: false)
			IncludeImageData:      true,                 // CLI default: true  (API default: false)
			IncludeTableStructure: true,                 // CLI default: true  (API default: false)
			Pages:                 true,                 // CLI default: true  (API default: false)
			TitleTree:             true,                 // CLI default: true  (API default: false)
			TableView:             "html",               // CLI default: html  (API default: html)
		},
	}

	if o.Password != "" {
		cfg.Document = &DocumentConfig{Password: o.Password}
	}

	if o.PageRange != "" {
		cfg.Scope = &Scope{PageRange: o.PageRange}
	}

	data, _ := json.Marshal(cfg)
	return string(data)
}

// resolveAPIMode determines whether to use free or paid API.
// Logic:
//   - --api free  → free
//   - --api paid  → paid (requires credentials)
//   - default     → paid if credentials exist, else free
func resolveAPIMode(mode APIMode, cred *config.CredentialSource) (isFree bool) {
	switch mode {
	case APIModeFree:
		return true
	case APIModePaid:
		return false
	default:
		// Auto: use paid if credentials are available
		return cred.AppID == "" || cred.SecretCode == ""
	}
}

// newXParserClient creates a client configured for free or paid API.
func newXParserClient(cmd *cobra.Command, cred *config.CredentialSource, isFree bool) *XParserClient {
	cfg, _ := config.Load()

	var baseURL, parsePath string
	if isFree {
		baseURL = freeAPIBaseURL
		parsePath = freeParseAPIPath
	} else {
		baseURL = config.GetBaseURL(cmd, cfg)
		parsePath = paidParseAPIPath
	}

	httpClient := &http.Client{}
	if verboseFlag {
		httpClient = newVerboseHTTPClient()
	}

	return &XParserClient{
		AppID:      cred.AppID,
		SecretCode: cred.SecretCode,
		BaseURL:    baseURL,
		ParsePath:  parsePath,
		IsFreeAPI:  isFree,
		HTTPClient: httpClient,
	}
}

// ParseFile uploads a local file to the xParser API and returns the response.
func (c *XParserClient) ParseFile(filePath string, opts *ParseOptions) (*ParseResponse, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// file field
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	// config field (JSON string)
	if err := writer.WriteField("config", opts.buildConfig()); err != nil {
		return nil, fmt.Errorf("failed to write config field: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", c.BaseURL+c.ParsePath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.setAuthHeaders(req)

	return c.doRequest(req)
}

// ParseURL sends a URL to the xParser API for remote file parsing.
func (c *XParserClient) ParseURL(fileURL string, opts *ParseOptions) (*ParseResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// file_url field
	if err := writer.WriteField("file_url", fileURL); err != nil {
		return nil, fmt.Errorf("failed to write file_url field: %w", err)
	}

	// config field (JSON string)
	if err := writer.WriteField("config", opts.buildConfig()); err != nil {
		return nil, fmt.Errorf("failed to write config field: %w", err)
	}
	writer.Close()

	req, err := http.NewRequest("POST", c.BaseURL+c.ParsePath, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	c.setAuthHeaders(req)

	return c.doRequest(req)
}

func (c *XParserClient) setAuthHeaders(req *http.Request) {
	req.Header.Set("X-From", "cli")
	if !c.IsFreeAPI && c.AppID != "" && c.SecretCode != "" {
		req.Header.Set("x-ti-app-id", c.AppID)
		req.Header.Set("x-ti-secret-code", c.SecretCode)
	}
}

func (c *XParserClient) doRequest(req *http.Request) (*ParseResponse, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var result ParseResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
		}
		return nil, fmt.Errorf("failed to parse response JSON: %w", err)
	}

	return &result, nil
}
