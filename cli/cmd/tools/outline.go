package tools

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/cache"
	"github.com/intsig-textin/xparse-skills/cli/internal/models"
)

var (
	outlineDepth    int
	outlineParentID string
)

var getOutlineCmd = &cobra.Command{
	Use:   "get_outline <doc_id>",
	Short: "Get document outline/table of contents (from cache, zero API calls)",
	Long: `Return the hierarchical document outline from cached parse results.
Requires ensure_parsed to have been called first.

By default only returns the top 2 levels of headings to keep output compact.
Use --depth 0 to get all levels, or --parent-id to drill into a section.

Examples:
  xparse-cli get_outline abc123def456
  xparse-cli get_outline abc123def456 --depth 3
  xparse-cli get_outline abc123def456 --depth 0
  xparse-cli get_outline abc123def456 --parent-id e7f3a1`,
	Args: cobra.ExactArgs(1),
	RunE: runGetOutline,
}

func init() {
	getOutlineCmd.Flags().IntVar(&outlineDepth, "depth", 2, "Max heading depth to return (0 = all levels)")
	getOutlineCmd.Flags().StringVar(&outlineParentID, "parent-id", "", "Only return children of this element_id")
}

// outlineOutput is the JSON output of get_outline.
type outlineOutput struct {
	DocID       string                `json:"doc_id"`
	PageCount   int                   `json:"page_count"`
	HasTOC      bool                  `json:"has_toc"`
	OutlineText string                `json:"outline_text"`
	Entries     []models.OutlineEntry `json:"entries,omitempty"`
	Truncated   bool                  `json:"truncated,omitempty"`
	TotalTitles int                   `json:"total_titles,omitempty"`
}

func runGetOutline(cmd *cobra.Command, args []string) error {
	docID := args[0]

	if !cache.Has(docID) {
		return generalErr("cache miss for doc_id: "+docID,
			"[fix] call ensure_parsed first")
	}

	result, err := cache.Get(docID)
	if err != nil {
		return generalErr("failed to read cache: "+err.Error(), "")
	}

	pageCount := 0
	if result.Metadata != nil {
		pageCount = result.Metadata.PageCount
	}

	// Determine has_toc: count total nodes in title_tree > 2
	totalNodes := countTitleNodes(result.TitleTree)
	hasTOC := totalNodes > 2

	// Resolve the tree to flatten: either full tree or subtree under parent-id
	treeToFlatten := result.TitleTree
	if outlineParentID != "" {
		subtree := findChildrenByParentID(result.TitleTree, outlineParentID)
		if subtree == nil {
			return generalErr("parent-id not found: "+outlineParentID,
				"[fix] check available IDs from get_outline without --parent-id")
		}
		treeToFlatten = subtree
	}

	// Build entries with depth limit (0 = unlimited)
	var entries []models.OutlineEntry
	flattenNodes(treeToFlatten, result.Elements, "", nil, &entries, 1, outlineDepth)

	// Assign unique short_ids, then replace element_id with the truncated form
	// so that agents use short IDs directly from the element_id field.
	assignShortIDs(entries)
	for i := range entries {
		entries[i].ElementID = entries[i].ShortID
	}

	// Generate outline text
	outlineText := generateOutlineText(entries)

	truncated := outlineDepth > 0 && totalNodes > len(entries)

	// When truncated, omit entries: they are incomplete and would mislead the agent
	// into thinking all sections are covered. The agent must use --parent-id to drill
	// down and obtain a complete entries list for the target sub-tree.
	var outEntries []models.OutlineEntry
	if !truncated {
		outEntries = entries
	}

	out := &outlineOutput{
		DocID:       docID,
		PageCount:   pageCount,
		HasTOC:      hasTOC,
		OutlineText: outlineText,
		Entries:     outEntries,
		Truncated:   truncated,
		TotalTitles: totalNodes,
	}

	return outputJSON(out)
}

// findChildrenByParentID searches the tree for a node with the given element_id
// and returns its children. Returns nil if not found.
func findChildrenByParentID(nodes []models.TitleNode, targetID string) []models.TitleNode {
	for _, node := range nodes {
		if node.ElementID == targetID || strings.HasPrefix(node.ElementID, targetID) {
			return node.Children
		}
		if found := findChildrenByParentID(node.Children, targetID); found != nil {
			return found
		}
	}
	return nil
}

// flattenTitleTree converts the nested TitleNode tree into flat OutlineEntry list.
func flattenTitleTree(titleTree []models.TitleNode, elements []models.Element) []models.OutlineEntry {
	var entries []models.OutlineEntry
	flattenNodes(titleTree, elements, "", nil, &entries, 1, 0)
	return entries
}

func flattenNodes(nodes []models.TitleNode, elements []models.Element, parentID string, path []string, entries *[]models.OutlineEntry, currentLevel int, maxDepth int) {
	for _, node := range nodes {
		currentPath := make([]string, len(path)+1)
		copy(currentPath, path)
		currentPath[len(path)] = node.Title

		pageEnd := computePageEnd(node.ElementID, elements, node.PageNumber)

		entry := models.OutlineEntry{
			ElementID:   node.ElementID,
			Heading:     node.Title,
			HeadingPath: currentPath,
			Level:       node.Level,
			PageStart:   node.PageNumber,
			PageEnd:     pageEnd,
			ParentID:    parentID,
		}
		*entries = append(*entries, entry)

		// Recurse children only if within depth limit (maxDepth 0 = unlimited)
		if maxDepth == 0 || currentLevel < maxDepth {
			flattenNodes(node.Children, elements, node.ElementID, currentPath, entries, currentLevel+1, maxDepth)
		}
	}
}

// computePageEnd recursively collects descendant page numbers to find max.
func computePageEnd(elementID string, elements []models.Element, pageStart int) int {
	var pages []int
	collectDescendantPages(elementID, elements, &pages)
	if len(pages) > 0 {
		maxPage := pageStart
		for _, p := range pages {
			if p > maxPage {
				maxPage = p
			}
		}
		return maxPage
	}
	return pageStart
}

func collectDescendantPages(targetID string, elements []models.Element, pages *[]int) {
	for _, elem := range elements {
		if elem.Metadata != nil && elem.Metadata.ParentID == targetID {
			*pages = append(*pages, elem.PageNumber)
			collectDescendantPages(elem.ElementID, elements, pages)
		}
	}
}

// assignShortIDs generates unique short IDs for each entry.
func assignShortIDs(entries []models.OutlineEntry) {
	if len(entries) == 0 {
		return
	}

	shortLen := 6
	for {
		shortMap := make(map[string]int)
		for _, e := range entries {
			sid := truncateID(e.ElementID, shortLen)
			shortMap[sid]++
		}
		hasConflict := false
		for _, count := range shortMap {
			if count > 1 {
				hasConflict = true
				break
			}
		}
		if !hasConflict || shortLen >= 12 {
			break
		}
		shortLen++
	}

	for i, e := range entries {
		entries[i].ShortID = truncateID(e.ElementID, shortLen)
		if entries[i].ParentID != "" {
			entries[i].ParentID = truncateID(entries[i].ParentID, shortLen)
		}
	}
}

func truncateID(id string, length int) string {
	if len(id) <= length {
		return id
	}
	return id[:length]
}

// countTitleNodes counts total nodes in the title tree.
func countTitleNodes(nodes []models.TitleNode) int {
	count := len(nodes)
	for _, n := range nodes {
		count += countTitleNodes(n.Children)
	}
	return count
}

// generateOutlineText creates a Markdown-formatted outline string.
func generateOutlineText(entries []models.OutlineEntry) string {
	var lines []string
	for _, e := range entries {
		prefix := strings.Repeat("#", e.Level)
		line := fmt.Sprintf("%s %s [%d-%d]  {%s}", prefix, e.Heading, e.PageStart, e.PageEnd, e.ShortID)
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}
