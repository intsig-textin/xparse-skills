package telemetry

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/spf13/cobra"

	"gitlab.intsig.net/xparse/xparse-client/internal/config"
)

var trackedCommands = map[string]bool{
	"parse": true, "get_doc_info": true, "ensure_parsed": true, "get_outline": true,
	"read_content": true, "read_pages": true, "search_text": true, "get_confidence": true,
}

type invocation struct {
	task           Task
	taskEventIndex int
	startedAt      time.Time
	parseLinks     []ParseLink
	bearerToken    string
}

var recorder struct {
	sync.Mutex
	current *invocation
}

type SummaryFunc func(cmd *cobra.Command, args []string) CommandSummary

func Begin(commandName, contextPath, clientVersion string) {
	if config.Profile() != config.ProfileWorkBuddy || !trackedCommands[commandName] {
		return
	}
	now := time.Now()
	task, index := resolveTask(contextPath, clientVersion, now)
	recorder.Lock()
	recorder.current = &invocation{task: task, taskEventIndex: index, startedAt: now}
	recorder.Unlock()
}

func SetBearerToken(token string) {
	if token == "" {
		return
	}
	recorder.Lock()
	if recorder.current != nil {
		recorder.current.bearerToken = token
	}
	recorder.Unlock()
}

func RecordParseLink(xRequestID, jobID, fileID string) {
	if xRequestID == "" && jobID == "" && fileID == "" {
		return
	}
	recorder.Lock()
	defer recorder.Unlock()
	if recorder.current == nil {
		return
	}
	recorder.current.parseLinks = append(recorder.current.parseLinks, ParseLink{
		SegmentIndex: len(recorder.current.parseLinks), XRequestID: xRequestID,
		JobID: jobID, FileID: fileID,
	})
}

func WrapRunE(toolName string, summarize SummaryFunc, run func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(cmd *cobra.Command, args []string) error {
		err := run(cmd, args)
		summary := CommandSummary{Args: map[string]any{}}
		if summarize != nil {
			summary = summarize(cmd, args)
			if summary.Args == nil {
				summary.Args = map[string]any{}
			}
		}
		finish(cmd.Context(), toolName, summary, err)
		return err
	}
}

func finish(ctx context.Context, toolName string, summary CommandSummary, commandErr error) {
	if ctx == nil {
		ctx = context.Background()
	}
	recorder.Lock()
	current := recorder.current
	recorder.current = nil
	recorder.Unlock()
	if current == nil {
		return
	}

	finishedAt := time.Now()
	eventID := newUUID()
	for index := range current.parseLinks {
		current.parseLinks[index].EventID = eventID
	}
	exitCode := 0
	errorCode := ""
	if commandErr != nil {
		exitCode = 1
		if coded, ok := commandErr.(interface{ ExitCode() int }); ok {
			exitCode = coded.ExitCode()
		}
		errorCode = fmt.Sprintf("cli_exit_%d", exitCode)
	}
	event := Event{
		EventID: eventID, TaskContextID: current.task.TaskContextID,
		TaskEventIndex: current.taskEventIndex, SubtoolName: toolName,
		StartedAt: current.startedAt, FinishedAt: finishedAt,
		DurationMS: finishedAt.Sub(current.startedAt).Milliseconds(),
		Success:    commandErr == nil, ExitCode: exitCode, ErrorCode: errorCode,
		ArgsSummary: summary.Args, InputSummaries: summary.Inputs,
		ClientVersion: current.task.ClientVersion, Platform: runtime.GOOS,
		Profile: config.ProfileWorkBuddy, ContextStatus: current.task.ContextStatus,
	}
	_ = enqueueAndUpload(ctx, queuedEvent{
		Task: current.task, Event: event, ParseLinks: current.parseLinks,
		CreatedAt: finishedAt, NextAttemptAt: finishedAt,
	}, current.bearerToken)
}
