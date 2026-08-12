package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/exitcode"
)

const freeQuotaURL = "https://api.textin.com/api/v1/agent/parse/quota"

type quotaResponse struct {
	Code       int        `json:"code"`
	Message    string     `json:"message"`
	XRequestID string     `json:"x_request_id"`
	Data       *quotaData `json:"data"`
}

type quotaData struct {
	Enabled             bool   `json:"enabled"`
	DailyPageLimit      int    `json:"daily_page_limit"`
	DailyPagesUsed      int    `json:"daily_pages_used"`
	DailyPagesRemaining int    `json:"daily_pages_remaining"`
	MaxPagesPerRequest  int    `json:"max_pages_per_request"`
	MaxFileSizeMB       int    `json:"max_file_size_mb"`
	ResetAt             string `json:"reset_at"`
}

var quotaCmd = &cobra.Command{
	Use:   "quota",
	Short: "Show free API quota usage",
	RunE:  runQuota,
}

func init() {
	rootCmd.AddCommand(quotaCmd)
}

func runQuota(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		return usageErr("quota does not accept arguments", "[fix] run xparse-cli quota")
	}

	httpClient := &http.Client{}
	if verboseFlag {
		httpClient = newVerboseHTTPClient()
	}

	req, err := http.NewRequest(http.MethodGet, freeQuotaURL, nil)
	if err != nil {
		return generalErr("failed to create quota request", "")
	}
	req.Header.Set("x-from", "cli")

	resp, err := httpClient.Do(req)
	if err != nil {
		return generalErr(exitcode.ErrNetworkRequest, "[retry] with --verbose; max 2 retries, 2s backoff")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return generalErr("failed to read quota response", "[retry] with --verbose")
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		fmt.Fprintf(os.Stderr, "HTTP %d: %s\n", resp.StatusCode, string(body))
		return &exitError{code: exitcode.APIError, msg: resp.Status}
	}

	var quota quotaResponse
	if err := json.Unmarshal(body, &quota); err != nil {
		return generalErr("failed to parse quota response", "[retry] with --verbose")
	}

	if quota.Code != 200 {
		return apiErr(quota.Code, quota.Message, quota.XRequestID)
	}
	if quota.Data == nil {
		return generalErr("quota response has no data", "[retry] with --verbose")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Free quota: %d/%d pages remaining (%d used)\n", quota.Data.DailyPagesRemaining, quota.Data.DailyPageLimit, quota.Data.DailyPagesUsed)
	fmt.Fprintf(cmd.OutOrStdout(), "Per request: up to %d pages, %d MB\n", quota.Data.MaxPagesPerRequest, quota.Data.MaxFileSizeMB)
	fmt.Fprintf(cmd.OutOrStdout(), "Reset at: %s\n", quota.Data.ResetAt)
	return nil
}
