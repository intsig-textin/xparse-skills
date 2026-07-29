// Package client provides a shared HTTP client for the Textin xParser API.
package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
)

// API endpoints.
const (
	PaidAPIBaseURL   = "https://api.textin.com"
	PaidParseAPIPath = "/api/v1/xparse/parse/sync"

	FreeAPIBaseURL   = "https://api.textin.com"
	FreeParseAPIPath = "/api/v1/agent/parse/sync"

	clientFromEnv       = "XPARSE_CLIENT_FROM"
	clientFromCLI       = "cli"
	clientFromWorkBuddy = "workbuddy"
)

// APIMode represents free vs paid API selection.
type APIMode string

const (
	APIModeAuto APIMode = "auto" // compatibility alias for free
	APIModeFree APIMode = "free"
	APIModePaid APIMode = "paid"
)

// Client wraps HTTP calls to the Textin xParser API.
type Client struct {
	AppID       string
	SecretCode  string
	BearerToken string
	BaseURL     string
	ParsePath   string
	IsFreeAPI   bool
	HTTPClient  *http.Client
}

// ParseOptions holds parameters for an xparse API call.
type ParseOptions struct {
	PageRange             string // e.g. "1-5" or "1-2,5-10"
	Password              string
	IncludeHierarchy      bool
	IncludeInlineObjects  bool
	IncludeCharDetails    bool
	IncludeImageData      bool
	IncludeTableStructure bool
	IncludePages          bool
	IncludeTitleTree      bool
	TableView             string // "html" | "markdown"
}

// ── Request config structures ──

type parseRequestConfig struct {
	Document     *documentConfig `json:"document,omitempty"`
	Capabilities *capabilities   `json:"capabilities"`
	Scope        *scope          `json:"scope,omitempty"`
}

type documentConfig struct {
	Password string `json:"password,omitempty"`
}

type capabilities struct {
	IncludeHierarchy      bool   `json:"include_hierarchy"`
	IncludeInlineObjects  bool   `json:"include_inline_objects"`
	IncludeCharDetails    bool   `json:"include_char_details"`
	IncludeImageData      bool   `json:"include_image_data"`
	IncludeTableStructure bool   `json:"include_table_structure"`
	Pages                 bool   `json:"pages"`
	TitleTree             bool   `json:"title_tree"`
	TableView             string `json:"table_view"`
}

type scope struct {
	PageRange string `json:"page_range,omitempty"`
}

// BuildConfig constructs the JSON config string for the multipart "config" field.
func (o *ParseOptions) BuildConfig() string {
	cfg := parseRequestConfig{
		Capabilities: &capabilities{
			IncludeHierarchy:      o.IncludeHierarchy,
			IncludeInlineObjects:  o.IncludeInlineObjects,
			IncludeCharDetails:    o.IncludeCharDetails,
			IncludeImageData:      o.IncludeImageData,
			IncludeTableStructure: o.IncludeTableStructure,
			Pages:                 o.IncludePages,
			TitleTree:             o.IncludeTitleTree,
			TableView:             o.TableView,
		},
	}

	if o.Password != "" {
		cfg.Document = &documentConfig{Password: o.Password}
	}
	if o.PageRange != "" {
		cfg.Scope = &scope{PageRange: o.PageRange}
	}

	data, _ := json.Marshal(cfg)
	return string(data)
}

// ── Response structures ──

// ParseResponse is the top-level JSON response from the xParser API.
type ParseResponse struct {
	Code       int        `json:"code"`
	Message    string     `json:"message"`
	XRequestID string     `json:"x_request_id,omitempty"`
	Data       *ParseData `json:"data,omitempty"`
}

// UnmarshalJSON 兼容 "data" 和 "result" 两种字段名。
func (r *ParseResponse) UnmarshalJSON(b []byte) error {
	type alias struct {
		Code       int        `json:"code"`
		Message    string     `json:"message"`
		XRequestID string     `json:"x_request_id,omitempty"`
		Data       *ParseData `json:"data,omitempty"`
		Result     *ParseData `json:"result,omitempty"`
	}
	var a alias
	if err := json.Unmarshal(b, &a); err != nil {
		return err
	}
	r.Code = a.Code
	r.Message = a.Message
	r.XRequestID = a.XRequestID
	if a.Data != nil {
		r.Data = a.Data
	} else {
		r.Data = a.Result
	}
	return nil
}

// HasResult returns true if Data is present.
func (r *ParseResponse) HasResult() bool {
	return r.Data != nil
}

// GetMarkdown returns the markdown content from the response.
func (r *ParseResponse) GetMarkdown() string {
	if r.Data != nil {
		return r.Data.Markdown
	}
	return ""
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

// ── Client construction ──

// ResolveAPIMode determines whether to use free or paid API. Paid access must
// always be explicitly requested; stored credentials never change the default.
func ResolveAPIMode(mode APIMode, cred *config.CredentialSource) (isFree bool) {
	_ = cred // Kept in the signature for compatibility with existing callers.
	if mode == APIModePaid {
		return false
	}
	return true
}

// NewClient creates a client configured for free or paid API.
// Pass a custom httpClient (e.g. verbose) or nil for default.
func NewClient(cmd *cobra.Command, cred *config.CredentialSource, isFree bool, httpClient *http.Client) *Client {
	return NewClientWithBearer(cmd, cred, "", isFree, httpClient)
}

// NewClientWithBearer creates a client that prefers OAuth Bearer
// authentication over AppKey for paid API calls.
func NewClientWithBearer(cmd *cobra.Command, cred *config.CredentialSource, bearerToken string, isFree bool, httpClient *http.Client) *Client {
	cfg, _ := config.Load()

	var baseURL, parsePath string
	if isFree {
		baseURL = FreeAPIBaseURL
		parsePath = FreeParseAPIPath
		if (cmd != nil && cmd.Flags().Changed("base-url")) ||
			strings.TrimSpace(os.Getenv("XPARSE_BASE_URL")) != "" ||
			(config.Profile() == config.ProfileWorkBuddy &&
				cfg != nil && strings.TrimSpace(cfg.BaseURL) != "") {
			baseURL = config.GetBaseURL(cmd, cfg)
		}
	} else {
		baseURL = config.GetBaseURL(cmd, cfg)
		parsePath = PaidParseAPIPath
	}

	if httpClient == nil {
		httpClient = &http.Client{}
	}

	return &Client{
		AppID:       cred.AppID,
		SecretCode:  cred.SecretCode,
		BearerToken: bearerToken,
		BaseURL:     baseURL,
		ParsePath:   parsePath,
		IsFreeAPI:   isFree,
		HTTPClient:  httpClient,
	}
}

// NewAutoClient creates a default-free client. The name is retained for source
// compatibility; callers must use NewClient explicitly for paid requests.
func NewAutoClient(cmd *cobra.Command, cred *config.CredentialSource, httpClient *http.Client) *Client {
	isFree := ResolveAPIMode(APIModeAuto, cred)
	return NewClient(cmd, cred, isFree, httpClient)
}

// ── API methods ──

// ParseFile uploads a local file to the xParser API and returns the response.
func (c *Client) ParseFile(filePath string, opts *ParseOptions) (*ParseResponse, error) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("failed to create form file: %w", err)
	}
	if _, err := part.Write(fileData); err != nil {
		return nil, fmt.Errorf("failed to write file data: %w", err)
	}

	if err := writer.WriteField("config", opts.BuildConfig()); err != nil {
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
func (c *Client) ParseURL(fileURL string, opts *ParseOptions) (*ParseResponse, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	if err := writer.WriteField("file_url", fileURL); err != nil {
		return nil, fmt.Errorf("failed to write file_url field: %w", err)
	}

	if err := writer.WriteField("config", opts.BuildConfig()); err != nil {
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

func (c *Client) setAuthHeaders(req *http.Request) {
	req.Header.Set("X-From", resolveClientFrom(os.Getenv(clientFromEnv)))
	if c.IsFreeAPI {
		return
	}
	if c.BearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.BearerToken)
		return
	}
	if c.AppID != "" && c.SecretCode != "" {
		req.Header.Set("x-ti-app-id", c.AppID)
		req.Header.Set("x-ti-secret-code", c.SecretCode)
	}
}

func resolveClientFrom(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case clientFromWorkBuddy:
		return clientFromWorkBuddy
	default:
		if config.Profile() == config.ProfileWorkBuddy {
			return clientFromWorkBuddy
		}
		return clientFromCLI
	}
}

func (c *Client) doRequest(req *http.Request) (*ParseResponse, error) {
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
