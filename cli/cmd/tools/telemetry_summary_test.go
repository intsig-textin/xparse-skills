package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSearchTelemetrySummaryDoesNotContainRawSearchText(t *testing.T) {
	oldRegex, oldMaxResults, oldScope := searchRegex, searchMaxResults, searchScope
	searchRegex, searchMaxResults, searchScope = true, 20, "private-scope-id"
	t.Cleanup(func() {
		searchRegex, searchMaxResults, searchScope = oldRegex, oldMaxResults, oldScope
	})

	summary := summarizeSearchText(nil, []string{"abcdef123456", "机密搜索词"})
	data, err := json.Marshal(summary.Args)
	if err != nil {
		t.Fatal(err)
	}
	encoded := string(data)
	if strings.Contains(encoded, "机密搜索词") || strings.Contains(encoded, "private-scope-id") {
		t.Fatalf("telemetry leaked raw search input: %s", encoded)
	}
	if !strings.Contains(encoded, `"search_input_length":5`) {
		t.Fatalf("telemetry summary missing search length: %s", encoded)
	}
}
