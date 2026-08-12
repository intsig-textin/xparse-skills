package telemetry

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"gitlab.intsig.net/xparse/xparse-client/internal/config"
)

func resolveTask(contextPath, clientVersion string, now time.Time) (Task, int) {
	conversationHash := hashConversationID(os.Getenv("CODEBUDDY_SESSION_ID"))
	source := ""
	if conversationHash != "" {
		source = "CODEBUDDY_SESSION_ID"
	}
	if contextPath != "" {
		contextFile, status := readTaskContext(contextPath)
		task := Task{
			TaskContextID: newUUID(), Profile: config.ProfileWorkBuddy,
			ConversationIDHash: conversationHash, ConversationIDSource: source,
			ContextStatus: status, UserIntent: contextFile.UserIntent,
			ToolCallReason:  contextFile.ToolCallReason,
			IntentTruncated: contextFile.intentTruncated,
			ReasonTruncated: contextFile.reasonTruncated, ClientVersion: clientVersion,
		}
		if conversationHash != "" {
			_ = saveSessionEntry(conversationHash, sessionEntry{
				TaskContextID: task.TaskContextID, ConversationIDHash: conversationHash,
				ConversationIDSource: source, ContextStatus: status,
				NextEventIndex: 2, UpdatedAt: now,
			}, now)
		}
		return task, 1
	}

	if conversationHash != "" {
		if entry, index, ok := reserveSessionEntry(conversationHash, now); ok {
			return Task{
				TaskContextID: entry.TaskContextID, Profile: config.ProfileWorkBuddy,
				ConversationIDHash:   entry.ConversationIDHash,
				ConversationIDSource: entry.ConversationIDSource,
				ContextStatus:        entry.ContextStatus, ClientVersion: clientVersion,
			}, index
		}
	}

	return Task{
		TaskContextID: newUUID(), Profile: config.ProfileWorkBuddy,
		ConversationIDHash: conversationHash, ConversationIDSource: source,
		ContextStatus: "missing", ClientVersion: clientVersion,
	}, 1
}

func readTaskContext(path string) (taskContextFile, string) {
	file, err := os.Open(path)
	if err != nil {
		return taskContextFile{}, "invalid"
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 32*1024))
	decoder.DisallowUnknownFields()
	var contextFile taskContextFile
	if err := decoder.Decode(&contextFile); err != nil || contextFile.SchemaVersion != taskContextSchema {
		return taskContextFile{}, "invalid"
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		return taskContextFile{}, "invalid"
	}
	status := "present"
	contextFile.UserIntent, contextFile.intentTruncated = truncateRunes(contextFile.UserIntent, maxUserIntentRunes)
	if contextFile.intentTruncated {
		status = "oversized"
	}
	contextFile.ToolCallReason, contextFile.reasonTruncated = truncateRunes(contextFile.ToolCallReason, maxToolReasonRunes)
	if contextFile.reasonTruncated {
		status = "oversized"
	}
	return contextFile, status
}

func hashConversationID(value string) string {
	if value == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(value))
	return hex.EncodeToString(hash[:])
}

func newUUID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		now := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), os.Getpid())))
		copy(bytes, now[:16])
	}
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x",
		bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func sessionCachePath() (string, error) {
	dir, err := config.Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "telemetry", "session_context.json"), nil
}

func loadSessionState() sessionState {
	state := sessionState{Sessions: make(map[string]sessionEntry)}
	path, err := sessionCachePath()
	if err != nil {
		return state
	}
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &state) != nil || state.Sessions == nil {
		state.Sessions = make(map[string]sessionEntry)
	}
	return state
}

func saveSessionEntry(key string, entry sessionEntry, now time.Time) error {
	path, err := sessionCachePath()
	if err != nil {
		return err
	}
	return withFileLock(path+".lock", func() error {
		state := loadSessionState()
		for sessionKey, candidate := range state.Sessions {
			if now.Sub(candidate.UpdatedAt) > maxSessionAge {
				delete(state.Sessions, sessionKey)
			}
		}
		state.Sessions[key] = entry
		return writePrivateJSON(path, &state)
	})
}

func reserveSessionEntry(key string, now time.Time) (sessionEntry, int, bool) {
	path, err := sessionCachePath()
	if err != nil {
		return sessionEntry{}, 0, false
	}
	var reserved sessionEntry
	var index int
	err = withFileLock(path+".lock", func() error {
		state := loadSessionState()
		entry, ok := state.Sessions[key]
		if !ok || now.Sub(entry.UpdatedAt) > maxSessionAge || entry.TaskContextID == "" {
			return os.ErrNotExist
		}
		index = entry.NextEventIndex
		if index < 1 {
			index = 1
		}
		entry.NextEventIndex = index + 1
		entry.UpdatedAt = now
		state.Sessions[key] = entry
		reserved = entry
		return writePrivateJSON(path, &state)
	})
	return reserved, index, err == nil
}

func truncateRunes(value string, limit int) (string, bool) {
	runes := []rune(value)
	if len(runes) <= limit {
		return value, false
	}
	return string(runes[:limit]), true
}
