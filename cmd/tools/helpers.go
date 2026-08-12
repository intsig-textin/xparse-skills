package tools

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/authsession"
	"gitlab.intsig.net/xparse/xparse-client/internal/client"
	"gitlab.intsig.net/xparse/xparse-client/internal/config"
	"gitlab.intsig.net/xparse/xparse-client/internal/exitcode"
	"gitlab.intsig.net/xparse/xparse-client/internal/telemetry"
)

// resolveCredentials resolves credentials for primitive commands.
func resolveCredentials(cmd *cobra.Command) (*config.CredentialSource, error) {
	credSrc, err := config.ResolveCredentials(cmd)
	if err != nil {
		return nil, generalErr(exitcode.ErrCredentialsConfig,
			"[ask human] run xparse-cli auth or set XPARSE_APP_ID and XPARSE_SECRET_CODE env vars")
	}
	return credSrc, nil
}

// newClient creates a default-free xparse API client. Primitive cache-reading
// commands must not become paid merely because credentials are stored.
func newClient(cmd *cobra.Command, credSrc *config.CredentialSource) *client.Client {
	if config.Profile() != config.ProfileWorkBuddy {
		return client.NewAutoClient(cmd, credSrc, nil)
	}
	cfg, _ := config.Load()
	bearerToken, _ := authsession.AccessToken(cmd.Context(), cmd, cfg, &http.Client{Timeout: 30 * time.Second}, time.Now)
	telemetry.SetBearerToken(bearerToken)
	return client.NewClientWithBearer(cmd, credSrc, bearerToken, true, nil)
}

// outputJSON marshals and prints a value as JSON to stdout.
func outputJSON(v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return generalErr("failed to marshal output: "+err.Error(), "")
	}
	fmt.Println(string(data))
	return nil
}

// exitError carries a process exit code alongside the error message.
type exitError struct {
	code int
	msg  string
}

func (e *exitError) Error() string { return e.msg }
func (e *exitError) ExitCode() int { return e.code }

// usageErr outputs plain text to stderr and returns exit code 2.
func usageErr(message string, suggestion string) *exitError {
	fmt.Fprintln(os.Stderr, message)
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "> %s\n", suggestion)
	}
	return &exitError{code: exitcode.UsageError, msg: message}
}

// generalErr outputs plain text to stderr and returns exit code 1.
func generalErr(message string, suggestion string) *exitError {
	fmt.Fprintln(os.Stderr, message)
	if suggestion != "" {
		fmt.Fprintf(os.Stderr, "> %s\n", suggestion)
	}
	return &exitError{code: exitcode.GeneralError, msg: message}
}

// apiErr outputs API error to stderr and returns exit code 3.
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
