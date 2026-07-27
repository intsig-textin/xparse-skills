package tools

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/cache"
	"github.com/intsig-textin/xparse-skills/cli/internal/client"
	"github.com/intsig-textin/xparse-skills/cli/internal/models"
)

var (
	confElementID string
	confPage      int
	confText      string
)

var getConfidenceCmd = &cobra.Command{
	Use:   "get_confidence <doc_id>",
	Short: "Get OCR confidence for a specific element or page",
	Long: `Query OCR confidence with character-level details.
Requires a separate API call with --include-char-details.

Examples:
  xparse-cli get_confidence abc123def456 --element-id f6204d
  xparse-cli get_confidence abc123def456 --page 5
  xparse-cli get_confidence abc123def456 --element-id f6204d --text "17325"`,
	Args: cobra.ExactArgs(1),
	RunE: runGetConfidence,
}

func init() {
	getConfidenceCmd.Flags().StringVar(&confElementID, "element-id", "", "Element ID to check confidence for")
	getConfidenceCmd.Flags().IntVar(&confPage, "page", 0, "Page number to check confidence for")
	getConfidenceCmd.Flags().StringVar(&confText, "text", "", "Specific text fragment to check")
}

// confidenceOutput is the JSON output of get_confidence.
type confidenceOutput struct {
	Scope              string        `json:"scope"`
	ScopeRef           string        `json:"scope_ref"`
	Confidence         float64       `json:"confidence"`
	LowConfidenceSpans []lowConfSpan `json:"low_confidence_spans,omitempty"`
	TextConfidence     *float64      `json:"text_confidence,omitempty"`
	TextAlternatives   []string      `json:"text_alternatives,omitempty"`
}

type lowConfSpan struct {
	Text         string   `json:"text"`
	Confidence   float64  `json:"confidence"`
	Alternatives []string `json:"alternatives,omitempty"`
}

type charDetail struct {
	Char         string   `json:"char"`
	Confidence   float64  `json:"confidence"`
	Alternatives []string `json:"alternatives,omitempty"`
}

type elementWithChars struct {
	ElementID   string       `json:"element_id"`
	Type        string       `json:"type"`
	Text        string       `json:"text"`
	PageNumber  int          `json:"page_number"`
	CharDetails []charDetail `json:"char_details,omitempty"`
}

func runGetConfidence(cmd *cobra.Command, args []string) error {
	docID := args[0]

	if confElementID == "" && confPage == 0 {
		return usageErr("at least one of --element-id or --page is required",
			"[fix] specify --element-id <id> or --page <number>")
	}

	// Determine target page
	var targetPage int
	var scope, scopeRef string

	if confElementID != "" {
		scope = "element"
		scopeRef = confElementID
		if cache.Has(docID) {
			result, err := cache.Get(docID)
			if err == nil {
				targetPage = lookupElementPage(confElementID, result.Elements)
			}
		}
		if targetPage == 0 {
			return generalErr("element not found in cache: "+confElementID,
				"[fix] ensure the element_id is correct and ensure_parsed has been called")
		}
	} else {
		scope = "page"
		scopeRef = fmt.Sprintf("page:%d", confPage)
		targetPage = confPage
	}

	// Get filepath
	fpath, err := cache.GetFilepathByDocID(docID)
	if err != nil {
		return generalErr("doc_id not found in session: "+docID,
			"[fix] call get_doc_info first")
	}

	// Make API call with char-details
	credSrc, err := resolveCredentials(cmd)
	if err != nil {
		return err
	}
	apiClient := newClient(cmd, credSrc)

	opts := &client.ParseOptions{
		PageRange:             fmt.Sprintf("%d-%d", targetPage, targetPage),
		IncludeHierarchy:      true,
		IncludeInlineObjects:  false,
		IncludeCharDetails:    true,
		IncludeImageData:      false,
		IncludeTableStructure: false,
		IncludePages:          false,
		IncludeTitleTree:      false,
		TableView:             "html",
	}

	resp, err := apiClient.ParseFile(fpath, opts)
	if err != nil {
		return generalErr("API call failed: "+err.Error(),
			"[retry] get_confidence for "+docID)
	}
	if resp.Code != 200 {
		return apiErr(resp.Code, resp.Message, resp.XRequestID)
	}

	// Parse elements with char_details
	var elementsWithChars []elementWithChars
	if resp.Data != nil && resp.Data.Elements != nil {
		if err := json.Unmarshal(resp.Data.Elements, &elementsWithChars); err != nil {
			return generalErr("failed to parse char_details response: "+err.Error(), "")
		}
	}

	// Extract chars for scope
	var chars []charDetail
	if scope == "element" {
		chars = getCharsForElement(elementsWithChars, confElementID)
	} else {
		for _, elem := range elementsWithChars {
			chars = append(chars, elem.CharDetails...)
		}
	}

	if len(chars) == 0 {
		return outputJSON(&confidenceOutput{
			Scope:      scope,
			ScopeRef:   scopeRef,
			Confidence: 0,
		})
	}

	// Compute overall confidence
	confidence := meanConfidence(chars)

	// Extract low-confidence spans (threshold 0.85)
	lowSpans := extractLowConfidenceSpans(chars, 0.85)

	out := &confidenceOutput{
		Scope:              scope,
		ScopeRef:           scopeRef,
		Confidence:         confidence,
		LowConfidenceSpans: lowSpans,
	}

	// If text specified, locate and compute its confidence
	if confText != "" {
		textChars := locateTextInChars(chars, confText)
		if len(textChars) > 0 {
			tc := meanConfidence(textChars)
			out.TextConfidence = &tc
			out.TextAlternatives = collectAlternatives(textChars)
		}
	}

	return outputJSON(out)
}

func lookupElementPage(inputID string, elements []models.Element) int {
	for _, e := range elements {
		if e.ElementID == inputID || strings.HasPrefix(e.ElementID, inputID) {
			return e.PageNumber
		}
	}
	return 0
}

func getCharsForElement(elements []elementWithChars, inputID string) []charDetail {
	for _, e := range elements {
		if e.ElementID == inputID || strings.HasPrefix(e.ElementID, inputID) {
			return e.CharDetails
		}
	}
	return nil
}

func meanConfidence(chars []charDetail) float64 {
	if len(chars) == 0 {
		return 0
	}
	sum := 0.0
	for _, c := range chars {
		sum += c.Confidence
	}
	return sum / float64(len(chars))
}

func extractLowConfidenceSpans(chars []charDetail, threshold float64) []lowConfSpan {
	var spans []lowConfSpan
	var currentText strings.Builder
	var currentChars []charDetail

	flush := func() {
		if len(currentChars) > 0 {
			spans = append(spans, lowConfSpan{
				Text:         currentText.String(),
				Confidence:   meanConfidence(currentChars),
				Alternatives: collectAlternatives(currentChars),
			})
			currentText.Reset()
			currentChars = nil
		}
	}

	for _, c := range chars {
		if c.Confidence < threshold {
			currentText.WriteString(c.Char)
			currentChars = append(currentChars, c)
		} else {
			flush()
		}
	}
	flush()

	return spans
}

func locateTextInChars(chars []charDetail, text string) []charDetail {
	var full strings.Builder
	for _, c := range chars {
		full.WriteString(c.Char)
	}
	fullStr := full.String()

	idx := strings.Index(strings.ToLower(fullStr), strings.ToLower(text))
	if idx < 0 {
		return nil
	}

	// Map string byte index to char slice index
	charStart := 0
	pos := 0
	for i, c := range chars {
		if pos >= idx {
			charStart = i
			break
		}
		pos += len(c.Char)
	}

	charEnd := charStart
	pos = 0
	for i := charStart; i < len(chars); i++ {
		pos += len(chars[i].Char)
		if pos >= len(text) {
			charEnd = i + 1
			break
		}
	}

	if charEnd > len(chars) {
		charEnd = len(chars)
	}
	return chars[charStart:charEnd]
}

func collectAlternatives(chars []charDetail) []string {
	seen := make(map[string]bool)
	var alts []string
	for _, c := range chars {
		for _, a := range c.Alternatives {
			if !seen[a] {
				seen[a] = true
				alts = append(alts, a)
			}
		}
	}
	return alts
}
