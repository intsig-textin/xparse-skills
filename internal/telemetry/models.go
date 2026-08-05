package telemetry

import "time"

const (
	schemaVersion      = "xparse_telemetry.v1"
	taskContextSchema  = "xparse_task_context.v1"
	telemetryEndpoint  = "/api/v1/agent/telemetry/events/batch"
	maxOutboxAge       = 7 * 24 * time.Hour
	maxSessionAge      = 24 * time.Hour
	maxOutboxBytes     = 10 * 1024 * 1024
	maxBatchEvents     = 50
	maxBatchTasks      = 20
	maxBatchParseLinks = 100
	maxBatchBodyBytes  = 240 * 1024
	maxUserIntentRunes = 4000
	maxToolReasonRunes = 2000
)

type Task struct {
	TaskContextID        string `json:"task_context_id"`
	Profile              string `json:"profile"`
	ConversationIDHash   string `json:"conversation_id_hash"`
	ConversationIDSource string `json:"conversation_id_source"`
	ContextStatus        string `json:"context_status"`
	UserIntent           string `json:"user_intent,omitempty"`
	ToolCallReason       string `json:"tool_call_reason,omitempty"`
	IntentTruncated      bool   `json:"intent_truncated,omitempty"`
	ReasonTruncated      bool   `json:"reason_truncated,omitempty"`
	ClientVersion        string `json:"client_version"`
}

type Event struct {
	EventID        string         `json:"event_id"`
	TaskContextID  string         `json:"task_context_id"`
	TaskEventIndex int            `json:"task_event_index"`
	SubtoolName    string         `json:"subtool_name"`
	StartedAt      time.Time      `json:"started_at"`
	FinishedAt     time.Time      `json:"finished_at"`
	DurationMS     int64          `json:"duration_ms"`
	Success        bool           `json:"success"`
	ExitCode       int            `json:"exit_code"`
	ErrorCode      string         `json:"error_code"`
	ArgsSummary    map[string]any `json:"args_summary"`
	InputSummaries []InputSummary `json:"input_summaries"`
	ClientVersion  string         `json:"client_version"`
	Platform       string         `json:"platform"`
	Profile        string         `json:"profile"`
	ContextStatus  string         `json:"context_status"`
}

type InputSummary struct {
	Kind        string `json:"kind"`
	DocumentRef string `json:"document_ref"`
	Ext         string `json:"ext,omitempty"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	SHA256_12   string `json:"sha256_12,omitempty"`
}

type ParseLink struct {
	EventID      string `json:"event_id"`
	SegmentIndex int    `json:"segment_index"`
	XRequestID   string `json:"x_request_id,omitempty"`
	JobID        string `json:"job_id,omitempty"`
	FileID       string `json:"file_id,omitempty"`
}

type BatchRequest struct {
	SchemaVersion string      `json:"schema_version"`
	Tasks         []Task      `json:"tasks"`
	Events        []Event     `json:"events"`
	ParseLinks    []ParseLink `json:"parse_links"`
}

type CommandSummary struct {
	Args   map[string]any
	Inputs []InputSummary
}

type taskContextFile struct {
	SchemaVersion   string `json:"schema_version"`
	UserIntent      string `json:"user_intent"`
	ToolCallReason  string `json:"tool_call_reason"`
	intentTruncated bool   `json:"-"`
	reasonTruncated bool   `json:"-"`
}

type queuedEvent struct {
	Task          Task        `json:"task"`
	Event         Event       `json:"event"`
	ParseLinks    []ParseLink `json:"parse_links,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	Attempt       int         `json:"attempt"`
	NextAttemptAt time.Time   `json:"next_attempt_at"`
}

type outboxState struct {
	DroppedCount int64         `json:"dropped_count"`
	Items        []queuedEvent `json:"items"`
}

type sessionEntry struct {
	TaskContextID        string    `json:"task_context_id"`
	ConversationIDHash   string    `json:"conversation_id_hash"`
	ConversationIDSource string    `json:"conversation_id_source"`
	ContextStatus        string    `json:"context_status"`
	NextEventIndex       int       `json:"next_event_index"`
	UpdatedAt            time.Time `json:"updated_at"`
}

type sessionState struct {
	Sessions map[string]sessionEntry `json:"sessions"`
}
