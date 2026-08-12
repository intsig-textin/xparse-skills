// Package cmd implements the CLI commands using cobra.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/cmd/tools"
	"gitlab.intsig.net/xparse/xparse-client/internal/config"
	"gitlab.intsig.net/xparse/xparse-client/internal/exitcode"
	"gitlab.intsig.net/xparse/xparse-client/internal/telemetry"
)

var (
	appIDFlag       string
	secretCodeFlag  string
	baseURLFlag     string
	profileFlag     string
	verboseFlag     bool
	taskContextFlag string
)

var rootCmd = &cobra.Command{
	Use:     "xparse-cli",
	Short:   "Textin xParser CLI — parse documents for Agents",
	Version: version,
	Long: `Textin xParser CLI is a command-line tool for document parsing powered by Textin xParser API.
Designed as Agent infrastructure — zero config, stdout-friendly, structured errors.

Supports: PDF, Images (png, jpg, bmp, tiff, webp), Doc(x), Ppt(x), Xls(x), HTML, TXT, OFD, RTF

  # Zero config — free API, markdown to stdout
  xparse-cli parse report.pdf

  # JSON view
  xparse-cli parse report.pdf --view json

  # Save to directory
  xparse-cli parse report.pdf --output ./result/

  # Specific pages
  xparse-cli parse report.pdf --page-range "1-5"

  # Batch from file list
  xparse-cli parse --list files.txt --output ./result/

  # Use paid API
  xparse-cli parse report.pdf --api paid

For more information, visit https://www.textin.com`,
	SilenceUsage:  true,
	SilenceErrors: true,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		if err := config.SetProfile(profileFlag); err != nil {
			return usageErr(err.Error(), "[fix] use --profile workbuddy or omit --profile")
		}
		telemetry.Begin(cmd.Name(), taskContextFlag, version)
		return nil
	},
}

// boolStringFlags lists flags registered as StringVar with NoOptDefVal.
// For these flags, "--flag true/false" (space-separated) must be normalized
// to "--flag=true/false" before cobra parses args, because pflag's NoOptDefVal
// prevents consuming the next token as the flag value.
var boolStringFlags = map[string]bool{
	"--include-hierarchy":       true,
	"--include-inline-objects":  true,
	"--include-char-details":    true,
	"--include-image-data":      true,
	"--include-table-structure": true,
	"--include-pages":           true,
	"--include-title-tree":      true,
}

func normalizeArgs() {
	args := os.Args[1:]
	normalized := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if boolStringFlags[arg] && i+1 < len(args) {
			next := strings.ToLower(args[i+1])
			if next == "true" || next == "false" {
				normalized = append(normalized, arg+"="+next)
				i++ // skip next
				continue
			}
		}
		normalized = append(normalized, arg)
	}
	os.Args = append(os.Args[:1], normalized...)
}

func Execute() error {
	normalizeArgs()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	err := rootCmd.ExecuteContext(ctx)
	if err == nil {
		return nil
	}

	// Already a custom exitError — pass through as-is
	if _, ok := err.(*exitError); ok {
		return err
	}

	// Cobra errors (unknown flag, unknown command, wrong args, etc.) → exit code 2
	msg := formatCobraError(err)
	token := extractInvalidToken(err)
	fmt.Fprintln(os.Stderr, msg)
	fmt.Fprintf(os.Stderr, "> [fix] remove or fix %s; run xparse-cli help for available commands and flags\n", token)
	return &exitError{code: exitcode.UsageError, msg: msg}
}

// cobraErrorPatterns maps Cobra error prefixes to extraction functions.
// Each function extracts the invalid token from the error message.
var cobraErrorPatterns = []struct {
	prefix  string
	extract func(msg string) string
}{
	{"unknown flag: ", func(msg string) string {
		return strings.TrimPrefix(msg, "unknown flag: ")
	}},
	{"unknown shorthand flag: ", func(msg string) string {
		if idx := strings.LastIndex(msg, " in "); idx != -1 {
			return msg[idx+4:]
		}
		return ""
	}},
	{"unknown command ", func(msg string) string {
		if start := strings.IndexByte(msg, '"'); start != -1 {
			if end := strings.IndexByte(msg[start+1:], '"'); end != -1 {
				return msg[start+1 : start+1+end]
			}
		}
		return ""
	}},
}

// extractInvalidToken extracts the invalid token from a Cobra error for use in suggestions.
func extractInvalidToken(err error) string {
	msg := err.Error()
	for _, p := range cobraErrorPatterns {
		if strings.HasPrefix(msg, p.prefix) {
			if token := p.extract(msg); token != "" {
				return token
			}
		}
	}
	return "the invalid parameter"
}

// formatCobraError normalizes Cobra errors into "invalid parameter: <token>".
func formatCobraError(err error) string {
	msg := err.Error()
	for _, p := range cobraErrorPatterns {
		if strings.HasPrefix(msg, p.prefix) {
			if token := p.extract(msg); token != "" {
				return exitcode.ErrInvalidFlag + ": " + token
			}
		}
	}
	return msg
}

func init() {
	// Both "xparse-cli help [command]" and "xparse-cli [command] --help" are supported.

	rootCmd.PersistentFlags().StringVar(&appIDFlag, "app-id", "", "Textin App ID (overrides env and config)")
	rootCmd.PersistentFlags().StringVar(&secretCodeFlag, "secret-code", "", "Textin Secret Code (overrides env and config)")
	rootCmd.PersistentFlags().StringVar(&baseURLFlag, "base-url", "", "API base URL (for private deployments)")
	rootCmd.PersistentFlags().StringVar(&profileFlag, "profile", "", "Credential profile: workbuddy")
	rootCmd.PersistentFlags().BoolVar(&verboseFlag, "verbose", false, "Verbose mode, print HTTP details")
	rootCmd.PersistentFlags().StringVar(&taskContextFlag, "task-context", "", "Path to a WorkBuddy task context JSON file")

	// Register document tool primitives
	tools.RegisterCommands(rootCmd)
}
