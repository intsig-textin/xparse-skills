package tools

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/cache"
	"github.com/intsig-textin/xparse-skills/cli/internal/models"
)

var readPagesCmd = &cobra.Command{
	Use:   "read_pages <doc_id> <start_page> <end_page>",
	Short: "Read content by page range (from cache, zero API calls)",
	Long: `Read document content for a specified page range.
Maximum span is 20 pages per call.

Examples:
  xparse-cli read_pages abc123def456 1 5
  xparse-cli read_pages abc123def456 10 20`,
	Args: cobra.ExactArgs(3),
	RunE: runReadPages,
}

// readPagesOutput is the JSON output of read_pages.
type readPagesOutput struct {
	Pages []pageOutput `json:"pages"`
}

type pageOutput struct {
	Page            int          `json:"page"`
	ContentMarkdown string       `json:"content_markdown"`
	Tables          []tableBrief `json:"tables,omitempty"`
	Images          []imageBrief `json:"images,omitempty"`
}

type tableBrief struct {
	ElementID string     `json:"element_id"`
	Caption   string     `json:"caption,omitempty"`
	RowCount  int        `json:"row_count"`
	ColCount  int        `json:"col_count"`
	Headers   []string   `json:"headers,omitempty"`
	Rows      [][]string `json:"rows,omitempty"`
	Markdown  string     `json:"markdown"`
}

type imageBrief struct {
	ElementID string `json:"element_id"`
	Caption   string `json:"caption,omitempty"`
}

const maxPageSpan = 20

func runReadPages(cmd *cobra.Command, args []string) error {
	docID := args[0]
	var startPage, endPage int
	if _, err := fmt.Sscanf(args[1], "%d", &startPage); err != nil || startPage < 1 {
		return usageErr("invalid start_page: "+args[1], "[fix] provide a positive integer")
	}
	if _, err := fmt.Sscanf(args[2], "%d", &endPage); err != nil || endPage < 1 {
		return usageErr("invalid end_page: "+args[2], "[fix] provide a positive integer")
	}
	if endPage < startPage {
		return usageErr("end_page must be >= start_page", "[fix] swap start_page and end_page")
	}
	if endPage-startPage+1 > maxPageSpan {
		return usageErr(
			fmt.Sprintf("page range too large (%d pages), max is %d", endPage-startPage+1, maxPageSpan),
			fmt.Sprintf("[fix] use a range of at most %d pages", maxPageSpan))
	}

	if !cache.Has(docID) {
		return generalErr("cache miss for doc_id: "+docID,
			"[fix] run parse for the complete local document first")
	}

	result, err := cache.Get(docID)
	if err != nil {
		return generalErr("failed to read cache: "+err.Error(), "")
	}

	var pages []pageOutput
	for page := startPage; page <= endPage; page++ {
		pageOut := buildPage(page, result.Elements, result.Pages)
		pages = append(pages, pageOut)
	}

	return outputJSON(&readPagesOutput{Pages: pages})
}

func buildPage(pageNumber int, elements []models.Element, pagesInfo []models.PageInfo) pageOutput {
	var pageElements []models.Element

	// Try pages[].element_ids first
	var pageInfo *models.PageInfo
	for i, p := range pagesInfo {
		if p.PageNumber == pageNumber {
			pageInfo = &pagesInfo[i]
			break
		}
	}

	if pageInfo != nil && len(pageInfo.ElementIDs) > 0 {
		for _, eid := range pageInfo.ElementIDs {
			for _, e := range elements {
				if e.ElementID == eid {
					pageElements = append(pageElements, e)
					break
				}
			}
		}
	} else {
		// Fallback: filter by page_number
		for _, e := range elements {
			if e.PageNumber == pageNumber {
				pageElements = append(pageElements, e)
			}
		}
		sortByCoordinates(pageElements)
	}

	var mdParts []string
	var tables []tableBrief
	var images []imageBrief

	for _, elem := range pageElements {
		switch elem.Type {
		case "Table":
			html := elem.Text
			caption := getCaptionFor(elem, elements)
			headers, rows := parseHTMLTable(html)
			colCount := 0
			if len(headers) > 0 {
				colCount = len(headers)
			} else if len(rows) > 0 {
				colCount = len(rows[0])
			}
			md := htmlTableToMarkdown(html)
			tables = append(tables, tableBrief{
				ElementID: truncateID(elem.ElementID, 6),
				Caption:   caption,
				RowCount:  len(rows),
				ColCount:  colCount,
				Headers:   headers,
				Rows:      rows,
				Markdown:  md,
			})
			mdParts = append(mdParts, md)
		case "Image":
			caption := getCaptionFor(elem, elements)
			images = append(images, imageBrief{
				ElementID: truncateID(elem.ElementID, 6),
				Caption:   caption,
			})
			url := ""
			if elem.ImageData != nil {
				url = elem.ImageData.ImageURL
			}
			mdParts = append(mdParts, fmt.Sprintf("![%s](%s)", caption, url))
		case "Header", "Footer", "FigureCaption", "TableCaption":
			// Skip
		default:
			if elem.Text != "" {
				mdParts = append(mdParts, elem.Text)
			}
		}
	}

	return pageOutput{
		Page:            pageNumber,
		ContentMarkdown: strings.Join(mdParts, "\n\n"),
		Tables:          tables,
		Images:          images,
	}
}

// sortByCoordinates sorts elements by their y-coordinate (top to bottom).
func sortByCoordinates(elements []models.Element) {
	for i := 1; i < len(elements); i++ {
		for j := i; j > 0; j-- {
			y1 := getY(elements[j-1])
			y2 := getY(elements[j])
			if y1 > y2 {
				elements[j-1], elements[j] = elements[j], elements[j-1]
			} else {
				break
			}
		}
	}
}

func getY(elem models.Element) float64 {
	if len(elem.Coordinates) >= 2 {
		return elem.Coordinates[1]
	}
	return 0
}
