package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/cache"
	"gitlab.intsig.net/xparse/xparse-client/internal/client"
	"gitlab.intsig.net/xparse/xparse-client/internal/exitcode"
	"gitlab.intsig.net/xparse/xparse-client/internal/models"
)

var ensureParsedCmd = &cobra.Command{
	Use:   "ensure_parsed <doc_id> <page_count>",
	Short: "Parse document fully and cache results",
	Long: `Ensure the document is fully parsed and cached.
Calls xparse API with automatic segmentation for documents > 50 pages.
Subsequent get_outline, read_content, search_text calls will use the cache.

Examples:
  xparse-cli ensure_parsed abc123def456 30
  xparse-cli ensure_parsed abc123def456 303`,
	Args: cobra.ExactArgs(2),
	RunE: runEnsureParsed,
}

// ensureParsedOutput is the JSON output of ensure_parsed.
type ensureParsedOutput struct {
	Success       bool `json:"success"`
	Cached        bool `json:"cached"`
	Segments      int  `json:"segments"`
	TotalElements int  `json:"total_elements"`
	TotalTitles   int  `json:"total_titles"`
}

func runEnsureParsed(cmd *cobra.Command, args []string) error {
	docID := args[0]
	var pageCount int
	if _, err := fmt.Sscanf(args[1], "%d", &pageCount); err != nil || pageCount <= 0 {
		return usageErr("invalid page_count: "+args[1],
			"[fix] provide a positive integer for page_count")
	}

	// Check cache
	if cache.Has(docID) {
		result, err := cache.Get(docID)
		if err == nil {
			out := &ensureParsedOutput{
				Success:       true,
				Cached:        true,
				Segments:      0,
				TotalElements: len(result.Elements),
				TotalTitles:   countTitles(result.Elements),
			}
			return outputJSON(out)
		}
	}

	// Get filepath from memory cache
	fpath, err := cache.GetFilepathByDocID(docID)
	if err != nil {
		return generalErr("doc_id not found in session: "+docID,
			"[fix] call get_doc_info first to register the document")
	}

	// Resolve credentials and create client
	credSrc, err := resolveCredentials(cmd)
	if err != nil {
		return err
	}
	apiClient := newClient(cmd, credSrc)

	const maxPagesPerCall = 50

	opts := &client.ParseOptions{
		IncludeHierarchy:      true,
		IncludeInlineObjects:  true,
		IncludeCharDetails:    false,
		IncludeImageData:      false,
		IncludeTableStructure: true,
		IncludePages:          true,
		IncludeTitleTree:      true,
		TableView:             "markdown",
	}

	var segments int
	var finalResult *models.ParseResult

	if pageCount <= maxPagesPerCall {
		// Single call
		segments = 1
		resp, err := apiClient.ParseFile(fpath, opts)
		if err != nil {
			return generalErr(exitcode.ErrNetworkRequest,
				"[retry] ensure_parsed failed for "+docID)
		}
		if resp.Code != 200 {
			return apiErr(resp.Code, resp.Message, resp.XRequestID)
		}
		finalResult, err = responseToParseResult(resp, fpath, pageCount)
		if err != nil {
			return generalErr("failed to process parse response: "+err.Error(), "")
		}
	} else {
		// Segmented parsing (serial)
		ranges := splitPageRanges(pageCount, maxPagesPerCall)
		segments = len(ranges)

		var segmentResults []*models.ParseResult
		for _, pageRange := range ranges {
			segOpts := *opts
			segOpts.PageRange = pageRange

			var resp *client.ParseResponse
			var callErr error

			// Retry once on failure
			for attempt := 0; attempt < 2; attempt++ {
				resp, callErr = apiClient.ParseFile(fpath, &segOpts)
				if callErr == nil && resp.Code == 200 {
					break
				}
				if attempt == 0 {
					time.Sleep(2 * time.Second)
				}
			}

			if callErr != nil {
				return generalErr(exitcode.ErrNetworkRequest,
					fmt.Sprintf("[retry] segment %s failed for %s", pageRange, docID))
			}
			if resp.Code != 200 {
				return apiErr(resp.Code, resp.Message, resp.XRequestID)
			}

			segResult, err := responseToParseResult(resp, fpath, 0)
			if err != nil {
				return generalErr("failed to process segment "+pageRange+": "+err.Error(), "")
			}
			segmentResults = append(segmentResults, segResult)
		}

		finalResult = mergeParseResults(segmentResults, pageCount)
	}

	// Write to disk cache
	if err := cache.Set(docID, finalResult); err != nil {
		return generalErr("failed to write cache: "+err.Error(),
			"[ask human] check disk space at ~/.xparse-cli/cache/")
	}

	out := &ensureParsedOutput{
		Success:       true,
		Cached:        false,
		Segments:      segments,
		TotalElements: len(finalResult.Elements),
		TotalTitles:   countTitles(finalResult.Elements),
	}
	return outputJSON(out)
}

// splitPageRanges divides total pages into segments of max size.
func splitPageRanges(pageCount, maxPerCall int) []string {
	var ranges []string
	start := 1
	for start <= pageCount {
		end := start + maxPerCall - 1
		if end > pageCount {
			end = pageCount
		}
		ranges = append(ranges, fmt.Sprintf("%d-%d", start, end))
		start = end + 1
	}
	return ranges
}

// responseToParseResult converts a ParseResponse to our internal ParseResult model.
func responseToParseResult(resp *client.ParseResponse, fpath string, totalPageCount int) (*models.ParseResult, error) {
	if !resp.HasResult() {
		return nil, fmt.Errorf("response has no result data")
	}

	result := &models.ParseResult{
		Markdown: resp.Data.Markdown,
	}

	// Parse metadata
	if resp.Data.Metadata != nil {
		result.Metadata = &models.DocumentMetadata{
			Filepath:  fpath,
			Filename:  resp.Data.Metadata.Filename,
			Filetype:  resp.Data.Metadata.Filetype,
			PageCount: resp.Data.Metadata.PageCount,
		}
		if totalPageCount > 0 {
			result.Metadata.PageCount = totalPageCount
		}
	}

	// Parse elements
	if resp.Data.Elements != nil {
		if err := json.Unmarshal(resp.Data.Elements, &result.Elements); err != nil {
			return nil, fmt.Errorf("failed to parse elements: %w", err)
		}
	}

	// Parse title_tree
	if resp.Data.TitleTree != nil {
		if err := json.Unmarshal(resp.Data.TitleTree, &result.TitleTree); err != nil {
			return nil, fmt.Errorf("failed to parse title_tree: %w", err)
		}
	}

	// Parse pages
	if resp.Data.Pages != nil {
		if err := json.Unmarshal(resp.Data.Pages, &result.Pages); err != nil {
			return nil, fmt.Errorf("failed to parse pages: %w", err)
		}
	}

	return result, nil
}

// mergeParseResults merges multiple segment results into one.
func mergeParseResults(segments []*models.ParseResult, totalPageCount int) *models.ParseResult {
	if len(segments) == 0 {
		return &models.ParseResult{}
	}

	merged := &models.ParseResult{}

	// Metadata: use first segment, override page_count with total
	if segments[0].Metadata != nil {
		meta := *segments[0].Metadata
		meta.PageCount = totalPageCount
		merged.Metadata = &meta
	}

	// Elements: concatenate in order
	for _, seg := range segments {
		merged.Elements = append(merged.Elements, seg.Elements...)
	}

	// Pages: concatenate in order
	for _, seg := range segments {
		merged.Pages = append(merged.Pages, seg.Pages...)
	}

	// TitleTree: merge with deduplication
	var treeSeg [][]models.TitleNode
	for _, seg := range segments {
		if len(seg.TitleTree) > 0 {
			treeSeg = append(treeSeg, seg.TitleTree)
		}
	}
	merged.TitleTree = mergeTitleTrees(treeSeg)

	// Markdown: concatenate
	var mdParts []string
	for _, seg := range segments {
		if seg.Markdown != "" {
			mdParts = append(mdParts, seg.Markdown)
		}
	}
	merged.Markdown = strings.Join(mdParts, "\n\n")

	return merged
}

// mergeTitleTrees merges title tree segments with deduplication by element_id.
func mergeTitleTrees(treeSegments [][]models.TitleNode) []models.TitleNode {
	var merged []models.TitleNode
	seenIDs := make(map[string]bool)

	for _, tree := range treeSegments {
		for _, node := range tree {
			mergeNodeRecursive(node, &merged, seenIDs)
		}
	}
	return merged
}

func mergeNodeRecursive(node models.TitleNode, targetList *[]models.TitleNode, seenIDs map[string]bool) {
	if seenIDs[node.ElementID] {
		existing := findNodeByID(*targetList, node.ElementID)
		if existing != nil {
			for _, child := range node.Children {
				mergeNodeRecursive(child, &existing.Children, seenIDs)
			}
		}
	} else {
		seenIDs[node.ElementID] = true
		newNode := models.TitleNode{
			ElementID:  node.ElementID,
			PageNumber: node.PageNumber,
			Title:      node.Title,
			Level:      node.Level,
		}
		for _, child := range node.Children {
			mergeNodeRecursive(child, &newNode.Children, seenIDs)
		}
		*targetList = append(*targetList, newNode)
	}
}

func findNodeByID(nodes []models.TitleNode, elementID string) *models.TitleNode {
	for i := range nodes {
		if nodes[i].ElementID == elementID {
			return &nodes[i]
		}
		if found := findNodeByID(nodes[i].Children, elementID); found != nil {
			return found
		}
	}
	return nil
}

func countTitles(elements []models.Element) int {
	count := 0
	for _, e := range elements {
		if e.Type == "Title" {
			count++
		}
	}
	return count
}
