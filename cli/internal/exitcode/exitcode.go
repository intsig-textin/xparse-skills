// Package exitcode defines CLI exit codes and fixed error messages.
package exitcode

// Exit codes per V1 spec.
const (
	Success      = 0 // Success
	GeneralError = 1 // General error — plain text on stderr
	UsageError   = 2 // Usage / parameter error — plain text on stderr
	APIError     = 3 // API error — "api_code：message" on stderr
)

// ── Exit 1: General error messages ──

const (
	ErrCredentialsConfig = "credentials configuration error"
	ErrNetworkRequest    = "network or request failed"
	ErrNoResultData      = "API returned success but no result data"
	ErrCreateOutputDir   = "failed to create output directory"
	ErrBatchPartial      = "batch completed with errors"
	ErrSaveResult        = "failed to save result"
	ErrFileNotFound      = "file not found"
	ErrOutputDirNotExist = "output directory does not exist"
)

// ── Exit 2: Usage error messages ──

const (
	ErrInvalidView        = "invalid --view value, must be 'markdown' or 'json'"
	ErrInvalidAPI         = "invalid --api value, must be 'free' or 'paid'"
	ErrInvalidCharDetails = "invalid --include-char-details value, must be 'true' or 'false'"
	ErrOpenListFile       = "failed to open or read --list file"
	ErrNoInput            = "no input files specified"
	ErrListRequiresOut    = "--list requires --output"
	ErrMultiRequiresOut   = "multiple inputs require --output"
	ErrPaidNoCreds        = "paid API requires credentials"
	ErrInvalidFlag        = "invalid parameter"
)

// ── Exit 3: API error mapping ──

// APIErrorInfo holds the API error details for exit code 3.
// Message comes from the actual API response, not a predefined translation.
type APIErrorInfo struct {
	APICode        int
	Message        string // from API response
	XRequestID     string // from API response
	Retryable      bool
	Suggestion     string
	ContactSupport bool   // true = user/agent cannot resolve; show request_id for Textin support
}

// suggestionMap maps Textin API error codes to agent-actionable suggestions.
var suggestionMap = map[int]struct {
	retryable      bool
	suggestion     string
	contactSupport bool
}{
	400:   {false, "[ask human] provide a valid file — current file is empty (0 bytes)", false},
	500:   {true, "[retry] max 2 retries, 2s backoff", true},
	30203: {true, "[retry] max 2 retries, 2s backoff", true},
	40003: {false, "[fallback] re-run with --api free; or [ask human] top up at textin.com", false},
	40004: {false, "[fix] check all flags and values; run xparse-cli help parse", false},
	40007: {false, "[ask human] check service configuration at textin.com", true},
	40008: {false, "[ask human] activate service at textin.com", true},
	40101: {false, "[fallback] re-run with --api free; or [ask human] run xparse-cli auth", false},
	40102: {false, "[fallback] re-run with --api free; or [ask human] run xparse-cli auth", false},
	40103: {false, "[ask human] check IP whitelist settings at textin.com", false},
	40301: {false, "[fix] convert to a supported format (JPG, PNG, BMP, TIFF, WebP)", true},
	40302: {false, "[ask human] file exceeds size limit (free API: 10MB, paid API: 500MB); compress/split the file, or switch to paid API with --api paid", false},
	40303: {false, "[fix] use a supported file format; if using free API, only PDF and images are supported — [ask human] run xparse-cli auth", true},
	40304: {false, "[fix] resize image to 20–20000px per side", true},
	40305: {false, "[fix] check the file argument", false},
	40306: {true, "[retry] wait 3s then retry, max 2 retries", false},
	40307: {false, "[fallback] re-run with --api paid; or [ask human] run xparse-cli auth", false},
	40400: {false, "[fix] check and correct the URL", true},
	40422: {false, "[ask human] re-download the file or provide a different copy", true},
	40423: {false, "[ask human] provide the correct password; re-run with --password <correct>", false},
	40424: {false, "[fix] adjust --page-range to fit within the document's actual page count", false},
	40425: {false, "[fix] use a supported file format; see xparse-cli help parse", true},
	40427: {false, "[fix] set DPI to 72, 144, or 216", false},
	40428: {true, "[retry] or [fix] convert to PDF first, then parse the PDF", true},
	40429: {false, "[ask human] contact Textin support", true},
	50207: {true, "[retry] partial results may be available in stdout, retry for full results", true},
}

// FromAPICode builds an APIErrorInfo using the message and request ID from the API response.
// Returns nil for success (200).
func FromAPICode(apiCode int, message string, xRequestID string) *APIErrorInfo {
	if apiCode == 200 {
		return nil
	}
	info := &APIErrorInfo{
		APICode:    apiCode,
		Message:    message,
		XRequestID: xRequestID,
	}
	if s, ok := suggestionMap[apiCode]; ok {
		info.Retryable = s.retryable
		info.Suggestion = s.suggestion
		info.ContactSupport = s.contactSupport
	} else {
		// Unknown error code — may need support
		info.Retryable = true
		info.Suggestion = "[retry] once; if still fails, contact Textin support"
		info.ContactSupport = true
	}
	return info
}
