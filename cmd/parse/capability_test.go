package cmd

import (
	"strings"
	"testing"

	"gitlab.intsig.net/xparse/xparse-client/internal/exitcode"
	"gitlab.intsig.net/xparse/xparse-client/internal/preflight"
)

func TestValidateAutomaticChannelSelectsExistingFreeQuota(t *testing.T) {
	snapshot := &ParseCapabilitySnapshot{
		SnapshotVersion: "parse-capability.v1",
		Supported:       true,
		Channels: []CapabilityChannel{{
			ID:                  "free",
			Available:           true,
			AutomaticUseAllowed: true,
			RemainingPages:      465,
			MaxPagesPerRequest:  50,
			MaxFileSizeBytes:    10 * 1024 * 1024,
		}},
	}
	spec := &preflight.Spec{
		SourceType:   preflight.SourceLocal,
		DetectedType: "pdf",
		SizeBytes:    1024,
		PageCount:    2,
	}
	if err := validateAutomaticChannel(snapshot, spec); err != nil {
		t.Fatal(err)
	}
}

func TestValidateAutomaticChannelNeverSelectsNewCharge(t *testing.T) {
	snapshot := &ParseCapabilitySnapshot{
		SnapshotVersion: "parse-capability.v1",
		Supported:       true,
		Channels: []CapabilityChannel{{
			ID:                  "paid",
			Available:           true,
			AutomaticUseAllowed: true,
			CreatesNewCharge:    true,
		}},
	}
	err := validateAutomaticChannel(snapshot, &preflight.Spec{SourceType: preflight.SourceURL})
	if err == nil || err.ExitCode() != exitcode.APIError ||
		!strings.Contains(err.Error(), "PAID_QUOTA_REQUIRED") {
		t.Fatalf("error = %#v", err)
	}
}

func TestValidateAutomaticChannelStopsBeforeUnplannedSplit(t *testing.T) {
	snapshot := &ParseCapabilitySnapshot{
		SnapshotVersion: "parse-capability.v1",
		Supported:       true,
		Channels: []CapabilityChannel{{
			ID:                  "free",
			Available:           true,
			AutomaticUseAllowed: true,
			MaxPagesPerRequest:  50,
			MaxFileSizeBytes:    10 * 1024 * 1024,
			SplitStrategies:     []string{"page_range", "physical_pdf"},
		}},
	}
	err := validateAutomaticChannel(snapshot, &preflight.Spec{
		SourceType: preflight.SourceLocal,
		SizeBytes:  1024,
		PageCount:  60,
	})
	if err == nil || !strings.Contains(err.Error(), "SPLIT_REQUIRED") {
		t.Fatalf("error = %#v", err)
	}
}
