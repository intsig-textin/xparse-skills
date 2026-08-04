package exitcode

import (
	"strings"
	"testing"
)

func TestInsufficientBalanceSuggestionIncludesPurchaseURL(t *testing.T) {
	const purchaseURL = "https://www.textin.com/market/chager/pdf_to_markdown"

	info := FromAPICode(40003, "Insufficient balance", "")
	if info == nil {
		t.Fatal("FromAPICode(40003) returned nil")
	}
	if info.Retryable {
		t.Fatal("insufficient balance must not be retryable")
	}
	if !strings.Contains(info.Suggestion, purchaseURL) {
		t.Fatalf("suggestion = %q, want purchase URL %s", info.Suggestion, purchaseURL)
	}
}
