// Package tools implements document operation primitive commands.
// These are agent-friendly commands that work with cached parse results.
package tools

import (
	"strconv"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/telemetry"
)

// RegisterCommands adds all tool primitive commands to the given root command.
func RegisterCommands(rootCmd *cobra.Command) {
	getDocInfoCmd.RunE = telemetry.WrapRunE("get_doc_info", summarizeGetDocInfo, runGetDocInfo)
	ensureParsedCmd.RunE = telemetry.WrapRunE("ensure_parsed", summarizeEnsureParsed, runEnsureParsed)
	getOutlineCmd.RunE = telemetry.WrapRunE("get_outline", summarizeGetOutline, runGetOutline)
	readContentCmd.RunE = telemetry.WrapRunE("read_content", summarizeCachedDocument, runReadContent)
	readPagesCmd.RunE = telemetry.WrapRunE("read_pages", summarizeReadPages, runReadPages)
	searchTextCmd.RunE = telemetry.WrapRunE("search_text", summarizeSearchText, runSearchText)
	getConfidenceCmd.RunE = telemetry.WrapRunE("get_confidence", summarizeGetConfidence, runGetConfidence)
	rootCmd.AddCommand(getDocInfoCmd)
	rootCmd.AddCommand(ensureParsedCmd)
	rootCmd.AddCommand(getOutlineCmd)
	rootCmd.AddCommand(readContentCmd)
	rootCmd.AddCommand(readPagesCmd)
	rootCmd.AddCommand(searchTextCmd)
	rootCmd.AddCommand(getConfidenceCmd)
}

func summarizeGetDocInfo(_ *cobra.Command, args []string) telemetry.CommandSummary {
	summary := telemetry.CommandSummary{Args: map[string]any{}}
	if len(args) > 0 {
		summary.Inputs = []telemetry.InputSummary{telemetry.SummarizeSource(args[0])}
	}
	return summary
}

func summarizeEnsureParsed(_ *cobra.Command, args []string) telemetry.CommandSummary {
	summary := summarizeCachedDocument(nil, args)
	if len(args) > 1 {
		if pageCount, err := strconv.Atoi(args[1]); err == nil {
			summary.Args["page_count"] = pageCount
		}
	}
	return summary
}

func summarizeGetOutline(_ *cobra.Command, args []string) telemetry.CommandSummary {
	summary := summarizeCachedDocument(nil, args)
	summary.Args["depth"] = outlineDepth
	summary.Args["has_parent_scope"] = outlineParentID != ""
	return summary
}

func summarizeCachedDocument(_ *cobra.Command, args []string) telemetry.CommandSummary {
	summary := telemetry.CommandSummary{Args: map[string]any{}}
	if len(args) > 0 {
		summary.Inputs = []telemetry.InputSummary{telemetry.SummarizeDocument(args[0])}
	}
	return summary
}

func summarizeReadPages(_ *cobra.Command, args []string) telemetry.CommandSummary {
	summary := summarizeCachedDocument(nil, args)
	if len(args) > 2 {
		startPage, startErr := strconv.Atoi(args[1])
		endPage, endErr := strconv.Atoi(args[2])
		if startErr == nil && endErr == nil {
			summary.Args["start_page"] = startPage
			summary.Args["end_page"] = endPage
		}
	}
	return summary
}

func summarizeSearchText(_ *cobra.Command, args []string) telemetry.CommandSummary {
	summary := summarizeCachedDocument(nil, args)
	if len(args) > 1 {
		summary.Args["search_input_length"] = utf8.RuneCountInString(args[1])
	}
	summary.Args["regex"] = searchRegex
	summary.Args["max_results"] = searchMaxResults
	summary.Args["has_scope"] = searchScope != ""
	return summary
}

func summarizeGetConfidence(_ *cobra.Command, args []string) telemetry.CommandSummary {
	summary := summarizeCachedDocument(nil, args)
	summary.Args["page"] = confPage
	summary.Args["scope"] = "page"
	if confElementID != "" {
		summary.Args["scope"] = "element"
	}
	summary.Args["text_length"] = utf8.RuneCountInString(confText)
	return summary
}
