// Package models defines shared data types for parsed document results.
package models

// ParseResult is the cached full parse result for a document.
type ParseResult struct {
	Metadata  *DocumentMetadata `json:"metadata"`
	Elements  []Element         `json:"elements"`
	TitleTree []TitleNode       `json:"title_tree"`
	Markdown  string            `json:"markdown"`
	Pages     []PageInfo        `json:"pages"`
}

// DocumentMetadata holds document-level metadata.
type DocumentMetadata struct {
	Filepath  string      `json:"filepath"`
	Filename  string      `json:"filename"`
	Filetype  string      `json:"filetype"`
	PageCount int         `json:"page_count"`
	DataSource *DataSource `json:"data_source,omitempty"`
}

// DataSource holds the URL reference.
type DataSource struct {
	URL string `json:"url,omitempty"`
}

// Element represents a parsed document element.
type Element struct {
	ElementID   string           `json:"element_id"`
	Type        string           `json:"type"`
	Text        string           `json:"text"`
	PageNumber  int              `json:"page_number"`
	Coordinates []float64        `json:"coordinates,omitempty"`
	Metadata    *ElementMetadata `json:"metadata,omitempty"`
	SubType     string           `json:"sub_type,omitempty"`
	ImageData   *ImageData       `json:"image_data,omitempty"`
}

// ElementMetadata holds element-specific metadata.
type ElementMetadata struct {
	CategoryDepth  int         `json:"category_depth"`
	ParentID       string      `json:"parent_id,omitempty"`
	ChildrenIDs    []string    `json:"children_ids,omitempty"`
	IsContinuation bool        `json:"is_continuation,omitempty"`
	DataSource     *DataSource `json:"data_source,omitempty"`
	RefElementID   string      `json:"ref_element_id,omitempty"`
	Height         int         `json:"height,omitempty"`
	Width          int         `json:"width,omitempty"`
}

// ImageData holds image-specific data.
type ImageData struct {
	ImageURL string `json:"image_url"`
	MimeType string `json:"mime_type"`
}

// TitleNode represents a node in the hierarchical title tree.
type TitleNode struct {
	ElementID  string      `json:"element_id"`
	PageNumber int         `json:"page_number"`
	Title      string      `json:"title"`
	Level      int         `json:"level"`
	Children   []TitleNode `json:"children,omitempty"`
}

// PageInfo holds per-page metadata.
type PageInfo struct {
	PageNumber   int      `json:"page_number"`
	ElementIDs   []string `json:"element_ids,omitempty"`
	PageHeight   int      `json:"page_height,omitempty"`
	PageWidth    int      `json:"page_width,omitempty"`
	Angle        int      `json:"angle,omitempty"`
	DPI          int      `json:"dpi,omitempty"`
	PageImageURL string   `json:"page_image_url,omitempty"`
	Status       string   `json:"status,omitempty"`
}

// DocInfo holds the result of get_doc_info.
type DocInfo struct {
	DocID     string `json:"doc_id"`
	Filepath  string `json:"filepath"`
	Filename  string `json:"filename"`
	PageCount int    `json:"page_count"`
	DocType   string `json:"doc_type"`
}

// OutlineEntry is a flattened outline entry for get_outline output.
type OutlineEntry struct {
	ElementID   string   `json:"element_id"`
	ShortID     string   `json:"short_id"`
	Heading     string   `json:"heading"`
	HeadingPath []string `json:"heading_path"`
	Level       int      `json:"level"`
	PageStart   int      `json:"page_start"`
	PageEnd     int      `json:"page_end"`
	ParentID    string   `json:"parent_id,omitempty"`
}
