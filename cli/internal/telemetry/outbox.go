package telemetry

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/intsig-textin/xparse-skills/cli/internal/authsession"
	"github.com/intsig-textin/xparse-skills/cli/internal/config"
	"github.com/intsig-textin/xparse-skills/cli/internal/credential"
)

func enqueueAndUpload(ctx context.Context, item queuedEvent, bearerToken string) error {
	if err := appendOutbox(item, time.Now()); err != nil {
		return err
	}
	selected, err := claimBatch(time.Now())
	if err != nil {
		return err
	}
	if len(selected) == 0 {
		return nil
	}
	deliveryCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
	defer cancel()
	if bearerToken == "" {
		bearerToken, err = telemetryBearerToken(deliveryCtx)
		if err != nil {
			_ = completeBatch(selected, false, time.Now())
			return err
		}
	}
	if err := uploadBatch(deliveryCtx, selected, bearerToken); err != nil {
		_ = completeBatch(selected, false, time.Now())
		return err
	}
	return completeBatch(selected, true, time.Now())
}

func outboxPath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "telemetry", "outbox.json"), nil
}

func loadOutbox() outboxState {
	var state outboxState
	path, err := outboxPath()
	if err != nil {
		return state
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &state) != nil {
		return outboxState{}
	}
	return state
}

func saveOutbox(state *outboxState) error {
	path, err := outboxPath()
	if err != nil {
		return err
	}
	return writePrivateJSON(path, state)
}

func appendOutbox(item queuedEvent, now time.Time) error {
	path, err := outboxPath()
	if err != nil {
		return err
	}
	return withFileLock(path+".lock", func() error {
		state := loadOutbox()
		state.Items = append(state.Items, item)
		pruneOutbox(&state, now)
		return saveOutbox(&state)
	})
}

func claimBatch(now time.Time) ([]queuedEvent, error) {
	path, err := outboxPath()
	if err != nil {
		return nil, err
	}
	var selected []queuedEvent
	err = withFileLock(path+".lock", func() error {
		state := loadOutbox()
		pruneOutbox(&state, now)
		selected = selectBatch(state.Items, now)
		selectedIDs := selectedEventIDs(selected)
		for index := range state.Items {
			if selectedIDs[state.Items[index].Event.EventID] {
				state.Items[index].NextAttemptAt = now.Add(time.Minute)
			}
		}
		return saveOutbox(&state)
	})
	return selected, err
}

func completeBatch(selected []queuedEvent, success bool, now time.Time) error {
	path, err := outboxPath()
	if err != nil {
		return err
	}
	return withFileLock(path+".lock", func() error {
		state := loadOutbox()
		if success {
			removeSelected(&state, selected)
		} else {
			backoffSelected(&state, selected, now)
		}
		return saveOutbox(&state)
	})
}

func writePrivateJSON(path string, value any) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	temporary, err := os.CreateTemp(dir, ".telemetry-*.tmp")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer func() {
		_ = temporary.Close()
		_ = os.Remove(temporaryPath)
	}()
	if err := temporary.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temporary.Write(data); err != nil {
		return err
	}
	if err := temporary.Sync(); err != nil {
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func pruneOutbox(state *outboxState, now time.Time) {
	kept := state.Items[:0]
	for _, item := range state.Items {
		if now.Sub(item.CreatedAt) <= maxOutboxAge {
			kept = append(kept, item)
		} else {
			state.DroppedCount++
		}
	}
	state.Items = kept
	for len(state.Items) > 0 {
		data, err := json.Marshal(state)
		if err != nil || len(data) <= maxOutboxBytes {
			break
		}
		state.Items = state.Items[1:]
		state.DroppedCount++
	}
}

func selectBatch(items []queuedEvent, now time.Time) []queuedEvent {
	selected := make([]queuedEvent, 0, maxBatchEvents)
	taskIDs := make(map[string]bool)
	parseLinkCount := 0
	estimatedBytes := 0
	for _, item := range items {
		if item.NextAttemptAt.After(now) {
			continue
		}
		if !taskIDs[item.Task.TaskContextID] && len(taskIDs) >= maxBatchTasks {
			continue
		}
		if len(selected) >= maxBatchEvents || parseLinkCount+len(item.ParseLinks) > maxBatchParseLinks {
			break
		}
		itemJSON, err := json.Marshal(item)
		if err != nil {
			continue
		}
		if len(selected) > 0 && estimatedBytes+len(itemJSON) > maxBatchBodyBytes {
			break
		}
		selected = append(selected, item)
		taskIDs[item.Task.TaskContextID] = true
		parseLinkCount += len(item.ParseLinks)
		estimatedBytes += len(itemJSON)
	}
	return selected
}

func uploadBatch(parent context.Context, items []queuedEvent, bearerToken string) error {
	requestBody := buildBatchRequest(items)
	body, err := json.Marshal(requestBody)
	if err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(parent, 300*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost,
		config.GetBaseURL(nil, cfg)+telemetryEndpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("X-From", "workbuddy")
	if bearerToken != "" {
		request.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	response, err := (&http.Client{Timeout: 300 * time.Millisecond}).Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("telemetry HTTP status %d", response.StatusCode)
	}
	var envelope struct {
		Code int `json:"code"`
		Data struct {
			Accepted  int `json:"accepted"`
			Duplicate int `json:"duplicate"`
			Rejected  int `json:"rejected"`
		} `json:"data"`
	}
	if json.Unmarshal(responseBody, &envelope) != nil || envelope.Code != http.StatusOK {
		return errors.New("telemetry response rejected")
	}
	if envelope.Data.Accepted+envelope.Data.Duplicate+envelope.Data.Rejected < len(items) {
		return errors.New("telemetry response did not account for all events")
	}
	return nil
}

func buildBatchRequest(items []queuedEvent) BatchRequest {
	tasksByID := make(map[string]Task)
	events := make([]Event, 0, len(items))
	links := make([]ParseLink, 0)
	for _, item := range items {
		if existing, ok := tasksByID[item.Task.TaskContextID]; ok {
			if existing.UserIntent == "" && item.Task.UserIntent != "" {
				existing.UserIntent = item.Task.UserIntent
				existing.ToolCallReason = item.Task.ToolCallReason
				tasksByID[item.Task.TaskContextID] = existing
			}
		} else {
			tasksByID[item.Task.TaskContextID] = item.Task
		}
		events = append(events, item.Event)
		links = append(links, item.ParseLinks...)
	}
	tasks := make([]Task, 0, len(tasksByID))
	for _, task := range tasksByID {
		tasks = append(tasks, task)
	}
	return BatchRequest{SchemaVersion: schemaVersion, Tasks: tasks, Events: events, ParseLinks: links}
}

func telemetryBearerToken(parent context.Context) (string, error) {
	store, err := credential.DefaultStore()
	if err != nil {
		return "", err
	}
	if _, err := store.Load(); err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	cfg, err := config.Load()
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(parent, 300*time.Millisecond)
	defer cancel()
	return authsession.AccessToken(ctx, nil, cfg, &http.Client{Timeout: 300 * time.Millisecond}, time.Now)
}

func backoffSelected(state *outboxState, selected []queuedEvent, now time.Time) {
	selectedIDs := selectedEventIDs(selected)
	for index := range state.Items {
		if !selectedIDs[state.Items[index].Event.EventID] {
			continue
		}
		state.Items[index].Attempt++
		backoff := time.Second * time.Duration(1<<min(state.Items[index].Attempt, 10))
		if backoff > 30*time.Minute {
			backoff = 30 * time.Minute
		}
		state.Items[index].NextAttemptAt = now.Add(backoff)
	}
}

func removeSelected(state *outboxState, selected []queuedEvent) {
	selectedIDs := selectedEventIDs(selected)
	kept := state.Items[:0]
	for _, item := range state.Items {
		if !selectedIDs[item.Event.EventID] {
			kept = append(kept, item)
		}
	}
	state.Items = kept
}

func selectedEventIDs(items []queuedEvent) map[string]bool {
	result := make(map[string]bool, len(items))
	for _, item := range items {
		result[item.Event.EventID] = true
	}
	return result
}
