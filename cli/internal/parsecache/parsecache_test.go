package parsecache

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/intsig-textin/xparse-skills/cli/internal/cache"
	"github.com/intsig-textin/xparse-skills/cli/internal/client"
)

func TestStoreWritesNavigationCacheForFullLocalParse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	source := filepath.Join(t.TempDir(), "report.pdf")
	if err := os.WriteFile(source, []byte("pdf"), 0o600); err != nil {
		t.Fatal(err)
	}
	response := &client.ParseResponse{Code: 200, Data: &client.ParseData{
		Markdown:  "# Report",
		Metadata:  &client.ParseMetadata{Filename: "report.pdf", Filetype: "pdf", PageCount: 2},
		Elements:  []byte(`[{"element_id":"title-1","type":"title","text":"Report","page_number":1}]`),
		TitleTree: []byte(`[{"element_id":"title-1","page_number":1,"title":"Report","level":1}]`),
		Pages:     []byte(`[{"page_number":1,"element_ids":["title-1"]}]`),
	}}
	docID, stored, err := Store(source, "", response)
	if err != nil {
		t.Fatal(err)
	}
	if !stored || docID == "" {
		t.Fatalf("expected stored cache with doc ID, got stored=%v docID=%q", stored, docID)
	}
	result, err := cache.Get(docID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Markdown != "# Report" || len(result.Elements) != 1 || len(result.TitleTree) != 1 || len(result.Pages) != 1 {
		t.Fatalf("unexpected cached result: %+v", result)
	}
}

func TestStoreSkipsPageRangeParse(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	docID, stored, err := Store("report.pdf", "1-2", &client.ParseResponse{Code: 200, Data: &client.ParseData{Markdown: "partial"}})
	if err != nil {
		t.Fatal(err)
	}
	if stored || docID != "" {
		t.Fatalf("partial parse must not be cached: stored=%v docID=%q", stored, docID)
	}
}
