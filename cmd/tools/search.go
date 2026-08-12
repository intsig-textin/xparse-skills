package tools

import (
	"regexp"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/cache"
	"gitlab.intsig.net/xparse/xparse-client/internal/models"
)

var (
	searchRegex      bool
	searchMaxResults int
	searchScope      string
)

var searchTextCmd = &cobra.Command{
	Use:   "search_text <doc_id> <pattern>",
	Short: "Full-text keyword search (from cache, zero API calls)",
	Long: `Search document elements by keyword or regex pattern.
Returns matching elements with context and heading reference.

Examples:
  xparse-cli search_text abc123def456 "净利润"
  xparse-cli search_text abc123def456 "\\d+\\.\\d+亿" --regex
  xparse-cli search_text abc123def456 "revenue" --max-results 50`,
	Args: cobra.ExactArgs(2),
	RunE: runSearchText,
}

func init() {
	searchTextCmd.Flags().BoolVar(&searchRegex, "regex", false, "Use regex matching (default: keyword substring)")
	searchTextCmd.Flags().IntVar(&searchMaxResults, "max-results", 20, "Maximum number of results")
	searchTextCmd.Flags().StringVar(&searchScope, "scope", "", "Limit search to this element_id and its descendants")
}

// searchOutput is the JSON output of search_text.
type searchOutput struct {
	TotalMatches int           `json:"total_matches"`
	Matches      []searchMatch `json:"matches"`
}

type searchMatch struct {
	MatchText        string `json:"match_text"`
	ElementID        string `json:"element_id"`
	ElementType      string `json:"element_type"`
	Page             int    `json:"page"`
	Context          string `json:"context"`
	HeadingRefID     string `json:"heading_ref_id,omitempty"`
	Heading          string `json:"heading,omitempty"`
	HeadingPageRange [2]int `json:"heading_page_range,omitempty"`
}

func runSearchText(cmd *cobra.Command, args []string) error {
	docID := args[0]
	pattern := args[1]

	if !cache.Has(docID) {
		return generalErr("cache miss for doc_id: "+docID,
			"[fix] call ensure_parsed first")
	}

	result, err := cache.Get(docID)
	if err != nil {
		return generalErr("failed to read cache: "+err.Error(), "")
	}

	// Resolve scope filter
	var scopeSet map[string]bool
	if searchScope != "" {
		fullScopeID := resolveElementID(searchScope, result.Elements, result.TitleTree)
		if fullScopeID == "" {
			return generalErr("scope element_id '"+searchScope+"' not found",
				"[fix] use element_id from get_outline or search_text output")
		}
		scopeSet = make(map[string]bool)
		collectDescendants(fullScopeID, result.Elements, scopeSet)
	}

	// Build outline entries for heading index
	entries := flattenTitleTree(result.TitleTree, result.Elements)
	assignShortIDs(entries)

	// Build heading reverse index
	headingIndex := buildHeadingIndex(entries, result.Elements)

	// Compile regex if needed
	var re *regexp.Regexp
	if searchRegex {
		re, err = regexp.Compile("(?i)" + pattern)
		if err != nil {
			return usageErr("invalid regex pattern: "+err.Error(),
				"[fix] check regex syntax")
		}
	}

	var matches []searchMatch
	for _, elem := range result.Elements {
		if elem.Text == "" {
			continue
		}
		if scopeSet != nil && !scopeSet[elem.ElementID] {
			continue
		}
		if len(matches) >= searchMaxResults {
			break
		}

		var matchText string
		var matchStart, matchEnd int

		if searchRegex {
			loc := re.FindStringIndex(elem.Text)
			if loc == nil {
				continue
			}
			matchStart, matchEnd = loc[0], loc[1]
			matchText = elem.Text[matchStart:matchEnd]
		} else {
			// Case-insensitive substring match
			lowerText := strings.ToLower(elem.Text)
			lowerPattern := strings.ToLower(pattern)
			idx := strings.Index(lowerText, lowerPattern)
			if idx < 0 {
				continue
			}
			matchStart = idx
			matchEnd = idx + len(pattern)
			matchText = elem.Text[matchStart:matchEnd]
		}

		// Build context (±75 chars around match)
		contextStart := matchStart - 75
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := matchEnd + 75
		if contextEnd > len(elem.Text) {
			contextEnd = len(elem.Text)
		}
		context := elem.Text[contextStart:contextEnd]
		if contextStart > 0 {
			context = "..." + context
		}
		if contextEnd < len(elem.Text) {
			context = context + "..."
		}

		// Lookup heading
		h := headingIndex[elem.ElementID]

		match := searchMatch{
			MatchText:   matchText,
			ElementID:   truncateID(elem.ElementID, 6),
			ElementType: elem.Type,
			Page:        elem.PageNumber,
			Context:     context,
		}
		if h != nil {
			match.HeadingRefID = h.shortID
			match.Heading = h.text
			match.HeadingPageRange = h.pageRange
		}

		matches = append(matches, match)
	}

	return outputJSON(&searchOutput{
		TotalMatches: len(matches),
		Matches:      matches,
	})
}

// headingRef holds heading reference info for the reverse index.
type headingRef struct {
	shortID   string
	text      string
	pageRange [2]int
}

// buildHeadingIndex creates a reverse index: element_id → nearest heading.
func buildHeadingIndex(entries []models.OutlineEntry, elements []models.Element) map[string]*headingRef {
	entryByID := make(map[string]*models.OutlineEntry)
	for i := range entries {
		entryByID[entries[i].ElementID] = &entries[i]
	}

	index := make(map[string]*headingRef)

	// Primary: use parent_id chain to find nearest Title in outline
	for _, elem := range elements {
		if elem.Metadata == nil {
			continue
		}
		parentID := elem.Metadata.ParentID
		for parentID != "" {
			if entry, ok := entryByID[parentID]; ok {
				index[elem.ElementID] = &headingRef{
					shortID:   entry.ShortID,
					text:      entry.Heading,
					pageRange: [2]int{entry.PageStart, entry.PageEnd},
				}
				break
			}
			parentID = findParentID(parentID, elements)
		}
	}

	// Fallback: page_range matching for elements without parent_id
	for _, elem := range elements {
		if _, found := index[elem.ElementID]; found {
			continue
		}
		var bestEntry *models.OutlineEntry
		for i := range entries {
			e := &entries[i]
			if e.PageStart <= elem.PageNumber && elem.PageNumber <= e.PageEnd {
				if bestEntry == nil || e.Level > bestEntry.Level {
					bestEntry = e
				}
			}
		}
		if bestEntry != nil {
			index[elem.ElementID] = &headingRef{
				shortID:   bestEntry.ShortID,
				text:      bestEntry.Heading,
				pageRange: [2]int{bestEntry.PageStart, bestEntry.PageEnd},
			}
		}
	}

	return index
}

func findParentID(elementID string, elements []models.Element) string {
	for _, e := range elements {
		if e.ElementID == elementID && e.Metadata != nil {
			return e.Metadata.ParentID
		}
	}
	return ""
}

// collectDescendants adds targetID and all its descendants (via parent_id chain) to the set.
func collectDescendants(targetID string, elements []models.Element, set map[string]bool) {
	set[targetID] = true
	for _, elem := range elements {
		if elem.Metadata != nil && elem.Metadata.ParentID == targetID {
			collectDescendants(elem.ElementID, elements, set)
		}
	}
}
