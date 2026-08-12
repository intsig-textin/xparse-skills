package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/cache"
)

var cacheCmd = &cobra.Command{
	Use:   "cache",
	Short: "Manage local document cache",
	Long:  `List or clean locally cached document parse results.`,
}

var cacheListCmd = &cobra.Command{
	Use:     "ls",
	Aliases: []string{"list"},
	Short:   "List cached documents",
	RunE:    runCacheList,
}

var cacheCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove all cached data",
	Long:  `Remove all cached doc_info and parse results from ~/.xparse-cli.`,
	RunE:  runCacheClean,
}

func init() {
	cacheCmd.AddCommand(cacheListCmd)
	cacheCmd.AddCommand(cacheCleanCmd)
	rootCmd.AddCommand(cacheCmd)
}

func runCacheList(cmd *cobra.Command, args []string) error {
	entries, err := cache.ListAll()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to list cache: %s\n", err)
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No cached documents.")
		return nil
	}

	data, _ := json.MarshalIndent(entries, "", "  ")
	fmt.Println(string(data))
	return nil
}

func runCacheClean(cmd *cobra.Command, args []string) error {
	if err := cache.CleanAll(); err != nil {
		fmt.Fprintf(os.Stderr, "failed to clean cache: %s\n", err)
		return err
	}
	fmt.Println("Cache cleaned.")
	return nil
}
