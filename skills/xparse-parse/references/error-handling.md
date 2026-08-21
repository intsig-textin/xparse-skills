# Structured Error Handling

Every failed CLI command writes exactly one final `xparse_error.v1` JSON object
to stderr. Task commands may write preceding `xparse_event.v1` JSONL progress
lines to the same stream; parse each line by `schema_version` and use the final
error object for the decision. Failed commands do not write a success object to
stdout. Read `error_code`, `retryable`, and `next_action`; do not branch on
translated message text or an old numeric API code.

```json
{
  "schema_version": "xparse_error.v1",
  "error_code": "FILE_TOO_LARGE",
  "message": "file exceeds the current service limit",
  "actual_value": {"size_bytes": 12582912},
  "limit": {"source": "service", "max_file_size_bytes": 10485760},
  "retryable": false,
  "next_action": "REDUCE_FILE",
  "request_id": "optional-request-id",
  "task_id": "optional-task-id"
}
```

## Field contract

| Field | Agent rule |
|-------|------------|
| `error_code` | Stable decision key. Prefer it over `message` or `details.api_code`. |
| `message` | Concise user-facing explanation. It can be localized. |
| `actual_value` | Current file size, page count, required pages, attempts, or another observed value. It is `null` when unavailable. |
| `limit` | Current service/account capability. Only use a value with `source: service`; never replace it with a remembered constant. It is `null` when the service did not report a limit. |
| `retryable` | `true` means the same logical action can be retried safely once at the Agent layer. `false` means do not retry or try a command variant; follow only `next_action`. |
| `next_action` | Stable action enum such as `FIX_INPUT`, `REDUCE_FILE`, `REDUCE_PAGES`, `WAIT_AND_RETRY`, `AUTHENTICATE`, `UPGRADE_OR_USE_PAID`, or `CONTACT_SUPPORT`. |
| `upgrade_url` | Optional purchase/upgrade URL. Showing it does not authorize a paid retry. |
| `request_id`, `task_id` | Preserve when reporting or escalating a failure. |
| `details` | Additional diagnostics, including `operation_id`, an upstream numeric `api_code`, or batch `failures`. Preserve `operation_id` for an idempotent retry; do not use details to override base fields. |

## Stable codes and decisions

| `error_code` | Decision |
|--------------|----------|
| `FILE_NOT_FOUND` | Stop and obtain the correct accessible path. |
| `EMPTY_FILE` | Stop and obtain a non-empty file. |
| `UNSUPPORTED_FILE_TYPE` | Stop or convert to a supported type. Never silently switch to a paid route. |
| `FILE_TOO_LARGE` | Read `actual_value` and the service-sourced `limit`; reduce/split the physical file or ask before an explicit paid retry. `--page-range` does not reduce upload bytes. |
| `PAGE_LIMIT_EXCEEDED` | Reduce the page selection using the reported limit. |
| `PAID_QUOTA_REQUIRED` | Stop. Explain current daily/package values and reset time, then wait for explicit paid approval. |
| `TASK_FREE_MODE_UNAVAILABLE` | Stop. The selected environment has no supported free-first Task billing capability. Do not fall back to serial parse or paid execution. |
| `IDEMPOTENCY_CONFLICT` | Stop. The operation ID was reused with different inputs or semantics. Keep the original operation intact and use a new ID only for a genuinely new request. |
| `CAPABILITY_QUERY_FAILED` | Do not invent quota or limits. Retry only when `retryable=true`; otherwise report `request_id`. |
| `SPLIT_FAILED` | Stop; do not parse an incomplete segment set. Preserve the original file. |
| `MERGE_FAILED` | Stop; do not present partial output as a complete document. Surface segment/task identifiers. |
| `RETRY_EXHAUSTED` | The CLI already used its bounded retry budget. Do not immediately repeat the same command. Follow `next_action` and preserve `request_id`. |
| `RATE_LIMITED` / `NETWORK_ERROR` / `SERVICE_ERROR` | Retry only when `retryable=true`; respect `WAIT_AND_RETRY`. |
| `AUTHENTICATION_FAILED` / `OAUTH_FAILED` | In WorkBuddy, ask the user to reconnect the Connector. Never request or print tokens. |
| `INVALID_ARGUMENT`, `INVALID_PAGE_RANGE`, `INVALID_PASSWORD` | Correct the input, then run once with the corrected arguments. |
| `BATCH_PARTIAL_FAILURE` | Inspect every `details.failures[]` item. Never hide failed inputs or claim the batch fully succeeded. |

## Durable Task states

Task commands return a persistent `task_id` and the latest Run state. Branch on
the state rather than repeating `task run`:

| State | Agent decision |
|-------|----------------|
| `scheduled`, `running` | Keep Task and Run IDs and use `task status <TASK_ID> --run-id <RUN_ID>` later. Do not recreate the Task or take over the Runtime's internal retries. |
| `completed` | Use `task read` for selected evidence or `task export` for the full set. If result access returns a non-retryable error, report it and stop; never return to `task run`. |
| `partial_failed`, `failed` | Run `task debug`; report successful and failed files separately. Do not recreate the whole Task. |
| `waiting_paid_authorization` | Stop and obtain explicit user approval. Authentication alone is not approval to spend. After approval, resume the exact Run with `task resume ... --approve-paid`. |
| `waiting_funds` | Stop and ask the user to add funds. After confirmation, resume the exact Run with `task resume ... --after-funding`. |
| `cancelled` | Report cancellation. Start a new Task only if the user asks. |

`waiting_paid_authorization` and `waiting_funds` are action-required state
projections, not `xparse_error.v1` failures. A successful CLI exit does not mean
the user request is complete. Follow only `next_action`, make no other xParse
call before the required human confirmation, and preserve the exact Task/Run
identity for resume.

For password failures such as API code `40423`, ask for the password, pass a
JSON map through stdin with `--passwords-stdin`, and run `task continue`. Never
place passwords in command arguments, a Task config file, telemetry, or the
response. See [task-runtime.md](task-runtime.md) for the recovery sequence.

## Quota and paid boundary

Run `quota --output json` when explaining `PAID_QUOTA_REQUIRED`. Routing uses
the server's current `daily_pages_remaining` and, only when returned for an
authenticated request, `free_package.free_remain_count`. The historical
`free_count` field is display-only and must not be used as remaining quota.

Authentication is not approval to spend. Never turn `--api auto` or `--api
free` into `--api paid` after an error. Wait for the user's explicit approval,
even when `upgrade_url` is present.

When the user explicitly chooses to purchase credits, use the
[TextIn xParse purchase page](https://www.textin.com/market/chager/pdf_to_markdown).

## Retry boundary

The CLI automatically retries recognized transient parse failures with a
bounded backoff. Therefore:

- when `retryable=true`, follow `next_action` and retry at most once at the Agent
  layer; for ambiguous Task submission, reuse the emitted `operation_id` with
  `--operation-id`;
- when `error_code=RETRY_EXHAUSTED`, do not immediately retry again;
- when `retryable=false`, stop the logical action and follow only
  `next_action`; for `CONTACT_SUPPORT`, report the failure and identifiers and
  execute no further xParse commands for the current request;
- changing timing, flags, authentication options, selector form, Resource ID,
  or switching between `task read` and `task export` does not reset the retry
  budget;
- after any Task/Run identifier has been observed, result-access failure must
  not fall back to `task run`, serial `parse`, cached output, or another Run;
- run each xParse command independently. A pipeline or compound shell command
  can hide the CLI exit status; the final `xparse_error.v1` still makes the
  operation a failure even when the wrapper reports exit code 0;
- never silently skip a failed parse or failed batch item.

## Reporting template

Tell the user what failed, the real observed value, the current service limit,
and the required next action. Include `request_id`/`task_id` when present. Do
not paste credentials, verbose HTTP headers, or a stale limit from this Skill.
