// Package parsecache converts successful parse responses into the document
// cache consumed by the navigation commands.
package parsecache

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/intsig-textin/xparse-skills/cli/internal/cache"
	"github.com/intsig-textin/xparse-skills/cli/internal/client"
	"github.com/intsig-textin/xparse-skills/cli/internal/models"
)

// Store writes a successful, full-document local parse to the navigation
// cache. Page-range parses are intentionally skipped because caching a partial
// document under the stable document ID would make later navigation incomplete.
func Store(source, pageRange string, response *client.ParseResponse) (string, bool, error) {
	if strings.TrimSpace(pageRange) != "" {
		return "", false, nil
	}
	if response == nil || !response.HasResult() {
		return "", false, fmt.Errorf("parse response has no result data")
	}
	absPath, err := filepath.Abs(source)
	if err != nil {
		return "", false, fmt.Errorf("resolve source path: %w", err)
	}
	result, err := fromResponse(response, absPath)
	if err != nil {
		return "", false, err
	}
	docID := cache.DocID(absPath)
	info := &models.DocInfo{DocID: docID, Filepath: absPath, Filename: filepath.Base(absPath)}
	if result.Metadata != nil {
		info.Filename = result.Metadata.Filename
		info.PageCount = result.Metadata.PageCount
		info.DocType = result.Metadata.Filetype
	}
	if info.Filename == "" {
		info.Filename = filepath.Base(absPath)
	}
	if info.DocType == "" {
		info.DocType = strings.TrimPrefix(strings.ToLower(filepath.Ext(absPath)), ".")
	}
	if err := cache.SetDocInfo(docID, info); err != nil {
		return "", false, fmt.Errorf("write document info cache: %w", err)
	}
	if err := cache.Set(docID, result); err != nil {
		return "", false, fmt.Errorf("write parse cache: %w", err)
	}
	return docID, true, nil
}

func fromResponse(response *client.ParseResponse, absPath string) (*models.ParseResult, error) {
	data := response.Data
	result := &models.ParseResult{Markdown: data.Markdown}
	result.Metadata = &models.DocumentMetadata{Filepath: absPath, Filename: filepath.Base(absPath)}
	if data.Metadata != nil {
		result.Metadata.Filename = data.Metadata.Filename
		result.Metadata.Filetype = data.Metadata.Filetype
		result.Metadata.PageCount = data.Metadata.PageCount
	}
	for _, target := range []struct {
		name string
		raw  json.RawMessage
		out  any
	}{
		{name: "elements", raw: data.Elements, out: &result.Elements},
		{name: "title_tree", raw: data.TitleTree, out: &result.TitleTree},
		{name: "pages", raw: data.Pages, out: &result.Pages},
	} {
		if len(target.raw) == 0 {
			continue
		}
		if err := json.Unmarshal(target.raw, target.out); err != nil {
			return nil, fmt.Errorf("decode %s: %w", target.name, err)
		}
	}
	return result, nil
}
