package tools

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/cache"
	"github.com/intsig-textin/xparse-skills/cli/internal/models"
)

var readContentCmd = &cobra.Command{
	Use:   "read_content <doc_id> <element_id>",
	Short: "Read content by element_id (from cache, zero API calls)",
	Long: `Read content of a specific element by its element_id or short_id.
Automatically adapts output format based on element type (section/table/paragraph).

Examples:
  xparse-cli read_content abc123def456 f6204d
  xparse-cli read_content abc123def456 f6204d1d59f45177ae3b1dc357d5eae97e12bd2335a19de80b20ecd3684d74e3`,
	Args: cobra.ExactArgs(2),
	RunE: runReadContent,
}

// readContentOutput is the JSON output of read_content.
type readContentOutput struct {
	ElementID       string       `json:"element_id"`
	ElementType     string       `json:"element_type"`
	Heading         string       `json:"heading"`
	HeadingPath     []string     `json:"heading_path,omitempty"`
	PageRange       [2]int       `json:"page_range"`
	ContentMarkdown string       `json:"content_markdown,omitempty"`
	ContentLength   int          `json:"content_length,omitempty"`
	Children        []childEntry `json:"children,omitempty"`
	Caption         string       `json:"caption,omitempty"`
	RowCount        int          `json:"row_count,omitempty"`
	ColCount        int          `json:"col_count,omitempty"`
	Headers         []string     `json:"headers,omitempty"`
	Rows            [][]string   `json:"rows,omitempty"`
	Markdown        string       `json:"markdown,omitempty"`
	HTML            string       `json:"html,omitempty"`
}

type childEntry struct {
	ElementID string `json:"element_id"`
	Heading   string `json:"heading"`
	PageRange [2]int `json:"page_range"`
}

func runReadContent(cmd *cobra.Command, args []string) error {
	docID := args[0]
	inputID := args[1]

	if !cache.Has(docID) {
		return generalErr("cache miss for doc_id: "+docID,
			"[fix] call ensure_parsed first")
	}

	result, err := cache.Get(docID)
	if err != nil {
		return generalErr("failed to read cache: "+err.Error(), "")
	}

	// Resolve element_id
	fullID := resolveElementID(inputID, result.Elements, result.TitleTree)
	if fullID == "" {
		return generalErr(
			fmt.Sprintf("element_id '%s' not found", inputID),
			"[fix] use element_id from get_outline or search_text output")
	}

	// Build outline entries for section detection
	entries := flattenTitleTree(result.TitleTree, result.Elements)
	assignShortIDs(entries)

	// Determine type
	elemType := resolveType(fullID, entries, result.Elements)

	var out *readContentOutput
	switch elemType {
	case "section":
		out = buildSectionOutput(fullID, entries, result.Elements)
	case "table":
		out = buildTableOutput(fullID, result.Elements)
	default:
		out = buildParagraphOutput(fullID, result.Elements, entries)
	}

	return outputJSON(out)
}

// resolveElementID expands a short_id to full element_id.
func resolveElementID(inputID string, elements []models.Element, titleTree []models.TitleNode) string {
	if len(inputID) > 8 {
		for _, e := range elements {
			if e.ElementID == inputID {
				return inputID
			}
		}
		return ""
	}

	// Short ID: prefix match
	for _, e := range elements {
		if strings.HasPrefix(e.ElementID, inputID) {
			return e.ElementID
		}
	}

	// Also check title_tree nodes
	if found := findInTitleTree(inputID, titleTree); found != "" {
		return found
	}

	return ""
}

func findInTitleTree(prefix string, nodes []models.TitleNode) string {
	for _, n := range nodes {
		if strings.HasPrefix(n.ElementID, prefix) {
			return n.ElementID
		}
		if found := findInTitleTree(prefix, n.Children); found != "" {
			return found
		}
	}
	return ""
}

// resolveType determines if an element_id is a section, table, or paragraph.
func resolveType(fullID string, entries []models.OutlineEntry, elements []models.Element) string {
	for _, e := range entries {
		if e.ElementID == fullID {
			return "section"
		}
	}
	for _, e := range elements {
		if e.ElementID == fullID {
			if e.Type == "Table" {
				return "table"
			}
			return "paragraph"
		}
	}
	return "paragraph"
}

func buildSectionOutput(fullID string, entries []models.OutlineEntry, elements []models.Element) *readContentOutput {
	var entry *models.OutlineEntry
	for i, e := range entries {
		if e.ElementID == fullID {
			entry = &entries[i]
			break
		}
	}
	if entry == nil {
		return &readContentOutput{ElementID: truncateID(fullID, 6), ElementType: "section"}
	}

	// Find title element to get children_ids
	var titleElem *models.Element
	for i, e := range elements {
		if e.ElementID == fullID {
			titleElem = &elements[i]
			break
		}
	}

	// Collect section elements
	var sectionElements []models.Element
	if titleElem != nil && titleElem.Metadata != nil && len(titleElem.Metadata.ChildrenIDs) > 0 {
		for _, cid := range titleElem.Metadata.ChildrenIDs {
			for _, e := range elements {
				if e.ElementID == cid {
					sectionElements = append(sectionElements, e)
					break
				}
			}
		}
	} else {
		// Fallback: elements with parent_id == fullID
		for _, e := range elements {
			if e.Metadata != nil && e.Metadata.ParentID == fullID {
				sectionElements = append(sectionElements, e)
			}
		}
	}

	// Build content markdown
	var mdParts []string
	for _, elem := range sectionElements {
		switch elem.Type {
		case "Title":
			continue
		case "Table":
			mdParts = append(mdParts, htmlTableToMarkdown(elem.Text))
		case "Image":
			caption := getCaptionFor(elem, elements)
			url := ""
			if elem.ImageData != nil {
				url = elem.ImageData.ImageURL
			}
			mdParts = append(mdParts, fmt.Sprintf("![%s](%s)", caption, url))
		case "Header", "Footer", "FigureCaption", "TableCaption":
			continue
		default:
			if elem.Text != "" {
				mdParts = append(mdParts, elem.Text)
			}
		}
	}
	contentMD := strings.Join(mdParts, "\n\n")

	// Build children list
	var children []childEntry
	if titleElem != nil && titleElem.Metadata != nil {
		for _, cid := range titleElem.Metadata.ChildrenIDs {
			for _, e := range elements {
				if e.ElementID == cid && e.Type == "Title" {
					pageEnd := computePageEnd(cid, elements, e.PageNumber)
					children = append(children, childEntry{
						ElementID: truncateID(cid, 6),
						Heading:   e.Text,
						PageRange: [2]int{e.PageNumber, pageEnd},
					})
					break
				}
			}
		}
	}

	return &readContentOutput{
		ElementID:       entry.ShortID,
		ElementType:     "section",
		Heading:         entry.Heading,
		HeadingPath:     entry.HeadingPath,
		PageRange:       [2]int{entry.PageStart, entry.PageEnd},
		ContentMarkdown: contentMD,
		ContentLength:   len(contentMD),
		Children:        children,
	}
}

func buildTableOutput(fullID string, elements []models.Element) *readContentOutput {
	var elem *models.Element
	for i, e := range elements {
		if e.ElementID == fullID {
			elem = &elements[i]
			break
		}
	}
	if elem == nil {
		return &readContentOutput{ElementID: truncateID(fullID, 6), ElementType: "table"}
	}

	html := elem.Text
	caption := getCaptionFor(*elem, elements)
	headers, rows := parseHTMLTable(html)

	colCount := 0
	if len(headers) > 0 {
		colCount = len(headers)
	} else if len(rows) > 0 {
		colCount = len(rows[0])
	}

	return &readContentOutput{
		ElementID:   truncateID(fullID, 6),
		ElementType: "table",
		Heading:     caption,
		PageRange:   [2]int{elem.PageNumber, elem.PageNumber},
		Caption:     caption,
		RowCount:    len(rows),
		ColCount:    colCount,
		Headers:     headers,
		Rows:        rows,
		Markdown:    htmlTableToMarkdown(html),
		HTML:        html,
	}
}

func buildParagraphOutput(fullID string, elements []models.Element, entries []models.OutlineEntry) *readContentOutput {
	var elem *models.Element
	for i, e := range elements {
		if e.ElementID == fullID {
			elem = &elements[i]
			break
		}
	}
	if elem == nil {
		return &readContentOutput{ElementID: truncateID(fullID, 6), ElementType: "paragraph"}
	}

	heading, headingPath := "", []string(nil)
	if elem.Metadata != nil && elem.Metadata.ParentID != "" {
		for _, e := range entries {
			if e.ElementID == elem.Metadata.ParentID {
				heading = e.Heading
				headingPath = e.HeadingPath
				break
			}
		}
	}

	return &readContentOutput{
		ElementID:       truncateID(fullID, 6),
		ElementType:     "paragraph",
		Heading:         heading,
		HeadingPath:     headingPath,
		PageRange:       [2]int{elem.PageNumber, elem.PageNumber},
		ContentMarkdown: elem.Text,
		ContentLength:   len(elem.Text),
	}
}

// getCaptionFor finds the caption for an Image or Table element via ref_element_id.
func getCaptionFor(elem models.Element, elements []models.Element) string {
	if elem.Metadata == nil || elem.Metadata.RefElementID == "" {
		return ""
	}
	for _, e := range elements {
		if e.ElementID == elem.Metadata.RefElementID {
			if e.Type == "FigureCaption" || e.Type == "TableCaption" {
				return e.Text
			}
		}
	}
	return ""
}

// htmlTableToMarkdown converts HTML table to Markdown format.
func htmlTableToMarkdown(html string) string {
	headers, rows := parseHTMLTable(html)
	if len(headers) == 0 && len(rows) == 0 {
		return html
	}

	var sb strings.Builder
	if len(headers) > 0 {
		sb.WriteString("| " + strings.Join(headers, " | ") + " |\n")
		sb.WriteString("|" + strings.Repeat(" --- |", len(headers)) + "\n")
	}
	for _, row := range rows {
		sb.WriteString("| " + strings.Join(row, " | ") + " |\n")
	}
	return sb.String()
}

// parseHTMLTable extracts headers and rows from an HTML table string.
func parseHTMLTable(html string) ([]string, [][]string) {
	var headers []string
	var rows [][]string

	inTH := false
	inTD := false
	inTR := false
	var currentCell strings.Builder
	var currentRow []string
	isHeaderRow := false

	i := 0
	for i < len(html) {
		if i+3 < len(html) && strings.EqualFold(html[i:i+3], "<tr") {
			end := strings.IndexByte(html[i:], '>')
			if end >= 0 {
				i += end + 1
				inTR = true
				currentRow = nil
				isHeaderRow = false
				continue
			}
		}
		if i+4 < len(html) && strings.EqualFold(html[i:i+4], "<th") {
			end := strings.IndexByte(html[i:], '>')
			if end >= 0 {
				i += end + 1
				inTH = true
				isHeaderRow = true
				currentCell.Reset()
				continue
			}
		}
		if i+4 < len(html) && strings.EqualFold(html[i:i+4], "<td") {
			end := strings.IndexByte(html[i:], '>')
			if end >= 0 {
				i += end + 1
				inTD = true
				currentCell.Reset()
				continue
			}
		}
		if i+5 <= len(html) && strings.EqualFold(html[i:i+5], "</th>") {
			if inTH {
				currentRow = append(currentRow, strings.TrimSpace(currentCell.String()))
				inTH = false
			}
			i += 5
			continue
		}
		if i+5 <= len(html) && strings.EqualFold(html[i:i+5], "</td>") {
			if inTD {
				currentRow = append(currentRow, strings.TrimSpace(currentCell.String()))
				inTD = false
			}
			i += 5
			continue
		}
		if i+5 <= len(html) && strings.EqualFold(html[i:i+5], "</tr>") {
			if inTR && len(currentRow) > 0 {
				if isHeaderRow && len(headers) == 0 {
					headers = currentRow
				} else {
					rows = append(rows, currentRow)
				}
			}
			inTR = false
			i += 5
			continue
		}
		if html[i] == '<' {
			end := strings.IndexByte(html[i:], '>')
			if end >= 0 {
				i += end + 1
				continue
			}
		}
		if inTH || inTD {
			currentCell.WriteByte(html[i])
		}
		i++
	}

	return headers, rows
}
