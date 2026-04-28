// Package tools implements document operation primitive commands.
// These are agent-friendly commands that work with cached parse results.
package tools

import "github.com/spf13/cobra"

// RegisterCommands adds all tool primitive commands to the given root command.
func RegisterCommands(rootCmd *cobra.Command) {
	rootCmd.AddCommand(getDocInfoCmd)
	rootCmd.AddCommand(ensureParsedCmd)
	rootCmd.AddCommand(getOutlineCmd)
	rootCmd.AddCommand(readContentCmd)
	rootCmd.AddCommand(readPagesCmd)
	rootCmd.AddCommand(searchTextCmd)
	rootCmd.AddCommand(getConfidenceCmd)
}
