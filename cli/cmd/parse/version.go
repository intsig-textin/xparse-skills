package cmd

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// Version info - set at build time via ldflags
var (
	version = "2.2.0"
	commit  = "none"
	date    = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display the version, commit hash, and build date of the xParser CLI.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintf(cmd.OutOrStdout(), "xparse-cli version %s\n", strings.TrimPrefix(version, "v"))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
