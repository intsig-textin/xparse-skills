package telemetry

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/credential"
)

func TestTaskContextCreatesAndSwitchesTaskWithinSession(t *testing.T) {
	setupTelemetryTest(t)
	t.Setenv("CODEBUDDY_SESSION_ID", "conversation-one")
	now := time.Now()
	contextOne := writeContextFile(t, "检查施工方案风险", "需要读取风险控制章节")

	first, firstIndex := resolveTask(contextOne, "2.2.0", now)
	continued, secondIndex := resolveTask("", "2.2.0", now.Add(time.Second))
	contextTwo := writeContextFile(t, "提取发票金额", "需要读取票面字段")
	second, thirdIndex := resolveTask(contextTwo, "2.2.0", now.Add(2*time.Second))

	if first.ContextStatus != "present" || first.UserIntent != "检查施工方案风险" {
		t.Fatalf("first task = %+v", first)
	}
	if first.TaskContextID != continued.TaskContextID || firstIndex != 1 || secondIndex != 2 {
		t.Fatalf("same session did not inherit task: first=%s/%d continued=%s/%d", first.TaskContextID, firstIndex, continued.TaskContextID, secondIndex)
	}
	if second.TaskContextID == first.TaskContextID || thirdIndex != 1 {
		t.Fatalf("new context did not switch task: first=%s second=%s index=%d", first.TaskContextID, second.TaskContextID, thirdIndex)
	}
	if first.ConversationIDHash == "conversation-one" || len(first.ConversationIDHash) != 64 {
		t.Fatalf("conversation ID was not hashed: %q", first.ConversationIDHash)
	}
	assertUUID(t, first.TaskContextID)
}

func TestInvalidAndOversizedContextNeverReturnsError(t *testing.T) {
	setupTelemetryTest(t)
	invalidPath := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalidPath, []byte(`{"schema_version":"bad"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	invalid, _ := resolveTask(invalidPath, "2.2.0", time.Now())
	if invalid.ContextStatus != "invalid" || invalid.UserIntent != "" {
		t.Fatalf("invalid task = %+v", invalid)
	}

	oversizedPath := writeContextFile(t, strings.Repeat("意", maxUserIntentRunes+1), "reason")
	oversized, _ := resolveTask(oversizedPath, "2.2.0", time.Now())
	if oversized.ContextStatus != "oversized" || !oversized.IntentTruncated || len([]rune(oversized.UserIntent)) != maxUserIntentRunes {
		t.Fatalf("oversized task status=%s length=%d", oversized.ContextStatus, len([]rune(oversized.UserIntent)))
	}
}

func TestParallelSessionReservationsHaveUniqueIndexes(t *testing.T) {
	setupTelemetryTest(t)
	t.Setenv("CODEBUDDY_SESSION_ID", "parallel-session")
	contextPath := writeContextFile(t, "并行读取", "需要读取多个章节")
	first, _ := resolveTask(contextPath, "2.2.0", time.Now())

	const count = 20
	indexes := make(chan int, count)
	taskIDs := make(chan string, count)
	var group sync.WaitGroup
	for range count {
		group.Add(1)
		go func() {
			defer group.Done()
			task, index := resolveTask("", "2.2.0", time.Now())
			indexes <- index
			taskIDs <- task.TaskContextID
		}()
	}
	group.Wait()
	close(indexes)
	close(taskIDs)
	seen := make(map[int]bool)
	for index := range indexes {
		if seen[index] {
			t.Fatalf("duplicate task event index: %d", index)
		}
		seen[index] = true
	}
	for taskID := range taskIDs {
		if taskID != first.TaskContextID {
			t.Fatalf("parallel reservation switched task: %s != %s", taskID, first.TaskContextID)
		}
	}
}

func TestFinishUploadsAnonymousEventAndParseLinks(t *testing.T) {
	setupTelemetryTest(t)
	t.Setenv("CODEBUDDY_SESSION_ID", "session")
	contextPath := writeContextFile(t, "读取文档", "需要分页读取")
	var received BatchRequest
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "" {
			t.Errorf("anonymous telemetry sent Authorization header")
		}
		if request.Header.Get("X-From") != "workbuddy" {
			t.Errorf("X-From = %q", request.Header.Get("X-From"))
		}
		if err := json.NewDecoder(request.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
		}
		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"code":200,"data":{"accepted":1,"duplicate":0,"rejected":0}}`))
	}))
	defer server.Close()
	t.Setenv("XPARSE_BASE_URL", server.URL)

	Begin("parse", contextPath, "2.2.1")
	RecordParseLink("request-1", "job-1", "file-1")
	RecordParseLink("request-2", "job-2", "file-1")
	finish(t.Context(), "parse", CommandSummary{
		Args:   map[string]any{"api": "auto"},
		Inputs: []InputSummary{{Kind: "cached_document", DocumentRef: "abcdef123456", Ext: ".pdf"}},
	}, nil)

	if len(received.Tasks) != 1 || received.Tasks[0].UserIntent != "读取文档" {
		t.Fatalf("received tasks = %+v", received.Tasks)
	}
	if len(received.Events) != 1 || !received.Events[0].Success || received.Events[0].SubtoolName != "parse" {
		t.Fatalf("received events = %+v", received.Events)
	}
	if len(received.ParseLinks) != 2 || received.ParseLinks[0].SegmentIndex != 0 || received.ParseLinks[1].SegmentIndex != 1 {
		t.Fatalf("received parse links = %+v", received.ParseLinks)
	}
	if received.ParseLinks[0].EventID != received.Events[0].EventID || received.ParseLinks[1].EventID != received.Events[0].EventID {
		t.Fatalf("parse links do not reference event")
	}
	state := loadOutbox()
	if len(state.Items) != 0 {
		t.Fatalf("outbox still contains %d items", len(state.Items))
	}
	path, err := outboxPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("outbox mode = %o", info.Mode().Perm())
	}
}

func TestUploadFailureKeepsOriginalCommandErrorAndBacksOff(t *testing.T) {
	setupTelemetryTest(t)
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()
	t.Setenv("XPARSE_BASE_URL", server.URL)

	Begin("read_pages", "", "2.2.0")
	original := errors.New("original command failure")
	wrapped := WrapRunE("read_pages", func(_ *cobra.Command, _ []string) CommandSummary {
		return CommandSummary{Args: map[string]any{"start_page": 1, "end_page": 2}}
	}, func(_ *cobra.Command, _ []string) error {
		return original
	})
	if err := wrapped(&cobra.Command{}, nil); !errors.Is(err, original) {
		t.Fatalf("wrapped error = %v, want original", err)
	}

	state := loadOutbox()
	if len(state.Items) != 1 || state.Items[0].Attempt != 1 || !state.Items[0].NextAttemptAt.After(state.Items[0].CreatedAt) {
		t.Fatalf("failed upload state = %+v", state)
	}
}

func TestTelemetryUsesValidOAuthWithoutPersistingToken(t *testing.T) {
	setupTelemetryTest(t)
	store, err := credential.DefaultStore()
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Save(&credential.OAuthToken{
		AccessToken: "private-access-token", ExpiresAt: time.Now().Add(time.Hour),
	}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer private-access-token" {
			t.Errorf("Authorization = %q", request.Header.Get("Authorization"))
		}
		_, _ = writer.Write([]byte(`{"code":200,"data":{"accepted":1,"duplicate":0,"rejected":0}}`))
	}))
	defer server.Close()
	t.Setenv("XPARSE_BASE_URL", server.URL)

	Begin("get_outline", "", "2.2.0")
	finish(t.Context(), "get_outline", CommandSummary{Args: map[string]any{}}, nil)

	path, err := outboxPath()
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "private-access-token") {
		t.Fatal("outbox persisted OAuth token")
	}
}

func TestPruneOutboxDropsExpiredItems(t *testing.T) {
	state := outboxState{Items: []queuedEvent{{CreatedAt: time.Now().Add(-maxOutboxAge - time.Minute)}}}
	pruneOutbox(&state, time.Now())
	if len(state.Items) != 0 || state.DroppedCount != 1 {
		t.Fatalf("pruned state = %+v", state)
	}
}

func TestParallelOutboxAppendsDoNotLoseEvents(t *testing.T) {
	setupTelemetryTest(t)
	const count = 20
	var group sync.WaitGroup
	for index := range count {
		group.Add(1)
		go func() {
			defer group.Done()
			now := time.Now()
			_ = appendOutbox(queuedEvent{
				Task:      Task{TaskContextID: newUUID()},
				Event:     Event{EventID: newUUID(), TaskEventIndex: index + 1},
				CreatedAt: now, NextAttemptAt: now,
			}, now)
		}()
	}
	group.Wait()
	state := loadOutbox()
	if len(state.Items) != count {
		t.Fatalf("outbox items = %d, want %d", len(state.Items), count)
	}
}

func setupTelemetryTest(t *testing.T) {
	t.Helper()
	t.Setenv("XPARSE_CONFIG_DIR", t.TempDir())
	t.Setenv("XPARSE_BASE_URL", "")
	t.Setenv("CODEBUDDY_SESSION_ID", "")
	if err := config.SetProfile(config.ProfileWorkBuddy); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = config.SetProfile("")
		recorder.Lock()
		recorder.current = nil
		recorder.Unlock()
	})
}

func writeContextFile(t *testing.T, intent, reason string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "context.json")
	data, err := json.Marshal(taskContextFile{SchemaVersion: taskContextSchema, UserIntent: intent, ToolCallReason: reason})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func assertUUID(t *testing.T, value string) {
	t.Helper()
	pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !pattern.MatchString(value) {
		t.Fatalf("invalid UUID: %q", value)
	}
}
