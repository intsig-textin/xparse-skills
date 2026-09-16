---
name: xparse-parse
description: "Parse, read, search, navigate, summarize, and extract tables or structured evidence from PDFs, images, Office files, HTML, OFD, and other supported local documents or document URLs through xparse-cli. Use this Skill for single-document conversion, server-generated DOCX/PDF/XLSX files, targeted section/page/fact extraction, durable multi-document Task Runtime workflows, and semantic extraction Tasks created from parsed File Asset IDs. Prefer it over raw PDF readers or custom OCR scripts."
---

# xparse-parse

Use the installed `xparse-cli` as the only parsing, authentication, quota, and
document-navigation execution kernel. Do not reproduce its HTTP, OAuth, quota,
PDF splitting, or result-merging logic in the Skill.

## Task context

For every new user request, create one private `0600` JSON file before the
first xParse command:

```json
{
  "schema_version": "xparse_task_context.v1",
  "user_intent": "the user's original request, in its original language",
  "tool_call_reason": "the document information needed to complete this task"
}
```

- Preserve the user's wording and keep the operational reason brief.
- Never include hidden reasoning, credentials, document content, or the final answer.
- Pass `--task-context <FILE>` only on the first xParse invocation for that request.
- Delete the temporary file after that invocation. Later commands inherit the task.
- Do not pass inline JSON through shell arguments, `echo`, or a heredoc.

## Command integrity and structured error gate

Run every operational `xparse-cli` invocation as a standalone shell command.
Do not pipe it through `head`, `tail`, `grep`, or another command, and do not
append cleanup, printing, file reads, or other shell commands that can replace
its exit status. Perform task-context cleanup in a separate shell call.

For every failed command, parse the final stderr object whose `schema_version`
is `xparse_error.v1`. Treat that object as failure even if a shell wrapper
reports exit code 0. Apply this gate before issuing another xParse command:

- `retryable=false` means do not retry or reinterpret the same logical action.
  Follow only the declared `next_action`. For `CONTACT_SUPPORT`, report the
  error and preserved identifiers, then issue no more xParse commands for the
  current request.
- `retryable=true` permits at most one Agent-layer retry of the same logical
  action. Keep the same Task, Run, Resource, and `operation_id` where present.
- Changing flags, authentication options, selector form, Resource identifier,
  timing, or switching between `task read` and `task export` does not create a
  new logical action or reset its retry budget.
- Do not run diagnostic xParse commands unless the Task state or `next_action`
  explicitly calls for them. Goal completion pressure is not a recovery signal;
  a correct failure report completes the Agent action.

After a non-retryable failure, another attempt is allowed only after the user
confirms an external remediation or explicitly requests a new action. Reuse the
preserved Task and Run identifiers; never recreate completed server work.

## Free, free-package, and paid routing

Use `--api auto` by default. The CLI queries the service quota before parsing and
uses the current server response as the authority instead of relying on a Skill
snapshot.

| Mode | CLI behavior | Use it when |
|------|--------------|-------------|
| `--api auto` | Uses the daily free API allowance first. When quota reports an AppKey-authenticated free package with sufficient `free_remain_count`, it can use that package through the existing authenticated route. | Default for supported PDF and image work. |
| `--api free` | Forces the free endpoint and does not use the authenticated free-package route. | The user explicitly requires the free endpoint only. |
| `--api paid` | Forces the paid endpoint and follows the service's existing package/balance billing behavior. | The user explicitly approves paid use, or approves it after learning that the format requires the paid API. |

Authentication is identity, not permission to spend. Never choose `--api paid`
only because OAuth or AppKey credentials exist.

Run `xparse-cli quota --output json` when the user asks about quota, when a routing failure
needs explanation, or before proposing a paid retry. Read all returned facts:

- daily free parse pages remaining and reset time;
- independent `extraction_quota` daily limit, used pages, remaining pages, and
  reset time when present;
- whether the request is authenticated;
- authenticated free-package total, historical used count, and current
  `free_remain_count` when present (routing uses only `free_remain_count`);
- maximum pages and file size per request.

Do not cache or calculate an allowance in the Skill. `parse --api auto` performs
its own quota preflight, and the parse response remains authoritative if quota
changes between inspection and execution. The Skill must not promise stronger
billing guarantees than the existing server provides.

Device OAuth and AppKey are different identities. If quota returns
`authenticated=false` or omits `free_package`, do not infer package access from
an OAuth login indicator. Treat only fields in the current quota response as
available.

The free endpoint supports PDF and images. Office, HTML, OFD, and other formats
may require `--api paid`; explain this and obtain the user's approval before
switching modes. If all reported free sources are insufficient, stop and explain
the current quota rather than silently retrying as paid.

## Choose the workflow

Choose by input shape and durability, not by whether authentication already
exists:

Workflow selection and billing selection are independent decisions. `Task`
versus `parse` is chosen from the request's input shape and durability needs;
`auto`, `free`, and `paid` choose only the billing route. A quota, eligibility,
authorization, funding, or format outcome must never change an accepted
multi-document Task into individual `parse` calls. Only an explicit user request
that narrows the original scope to a genuinely new one-document action may be
treated as a new `parse` operation.

- Use `parse` for one document or URL when the user needs an immediate result,
  conversion, or local outline/search navigation.
- Structured extraction is the exception: first distinguish a new extraction
  request from appending files to an existing extraction Task. Parse new local
  files through the first full Run of a new durable Parse Task, then either
  create an extraction Task or append to the existing extraction Task as
  requested. Follow the semantic extraction section below instead of reading
  parsed content into the Host.
- Use the durable Task Runtime for two or more local documents, or when the user
  explicitly needs a persistent Task ID, later status checks, selective result
  reads, exports, debugging, or continuation. A one-file request can therefore
  still be a Task when durability is explicit.
- Task Runtime control-plane routes and OAuth authentication are available in
  both domestic and overseas environments. Free-first Task billing is a
  separate capability: if the selected environment returns
  `TASK_FREE_MODE_UNAVAILABLE`, stop and explain it. Never replace the Task with
  serial `parse` calls or silently switch to paid execution.

### Durable multi-document Task Runtime

For local files, start one server-persisted Task instead of launching multiple
`parse` commands:

```bash
xparse-cli task run --files '<GLOB>' --api auto
```

`--api auto` is free-first and fails closed: it does not silently create a paid
Task. Use `--api paid` only after the user explicitly approves paid service
behavior. Do not parallelize individual `parse` commands for inputs that belong
to one Task.

`task run` returns after the server accepts the Run. When structured progress
is enabled, stderr is an `xparse_event.v1` JSONL stream: `run_accepted` exposes
the accepted Task/Run identity immediately, and `run_status` is emitted only
when the state changes.
Stdout contains exactly one final submission JSON. Preserve `operation_id`,
`task_id`, and `run_id`. If submission fails or
the process loses its response, reuse the observed `operation_id` with
`--operation-id`; never invent a new ID for the same logical submission.

Keep Agent workflows on the default submit-and-return path. Do not add
`--wait` or a short fixed `--timeout` automatically. When a user explicitly
requests foreground waiting or wait-and-export, `--wait` polls the same Run;
its local timeout returns the current accepted identity with
`wait_timed_out: true` and `next_action: POLL_STATUS`. It does not cancel or
recreate the Run. Continue with `task status` for that exact Task and Run.

`waiting_paid_authorization` and `waiting_funds` are accepted Task states, not
CLI transport failures. The submission/status JSON and its `next_action` are the
single authority. They mean the user request is incomplete: stop immediately
and issue no more xParse commands—not quota, status, read, export, debug,
another `task run`, or `parse`—until the user confirms the required external
action. Then call `task resume` once for the exact Task and Run.

Use `task status <TASK_ID> --run-id <RUN_ID>` for bounded progress checks. Start
at 2 seconds, then back off to 5, 10, 20, and 30 seconds; do not spend more than
about two minutes polling in one Agent turn. Return control with the IDs and
current state when work is still running. Never start a duplicate Task merely
because the Run is still `scheduled` or `running`.
Prefer `task read` when only one result is needed; use `task export` when the
user needs the complete result set. On partial failure, run `task debug` before
choosing a recovery action. Use `task continue` only when that accepted Run's
debug result identifies the existing Resource's raw Parse error code `40423`.
Supply per-file passwords by repeating `--password`; when more than one Resource
is involved, bind each value as `<SELECTOR>=<PASSWORD>`. This reruns only the
selected failed Resources without reprocessing successful files.

Task identity and state move forward only:

```text
no identifiers -> task run once
operation_id + PASSWORD_INPUT_REQUIRED -> ask for the named passwords, then replay the originating task run or task rerun --mode new-files once with that ID and the returned selectors
operation_id only after an ambiguous submission -> retry task run once with that ID
task_id + run_id -> task status for that exact Run
waiting_paid_authorization -> stop; after user approval, resume that exact Run
waiting_funds -> stop; after confirmed funding, resume that exact Run
completed -> task read or task export for that exact Run
non-retryable result-access failure -> report and stop
```

`PASSWORD_INPUT_REQUIRED` permits only one documented correction replay of the
originating command with the same `operation_id`. For initial submission that
command is `task run`; for new files under an existing Task it is `task rerun
--mode new-files`, and the error may legitimately include that existing
`task_id`. The CLI transparently reuses ready uploads; the Agent must not track
File Asset IDs or decide which files to upload. An `operation_id` without a
Task/Run ID after another ambiguous submission permits one unchanged replay.
Once a new `task_id` or `run_id` has been accepted for a logical submission,
never return to `task run` for it. A `task read` or `task export`
failure must not fall back to a new Task, serial `parse`, cached results, an
alternate selector, or a different Run. Use `task debug` only for
`partial_failed`/`failed`, not to investigate a completed Run whose result
access returned a non-retryable error.

Read [task-runtime.md](references/task-runtime.md) before starting, inspecting,
or recovering a durable Task.

### Semantic extraction Task from parsed File Assets

First resolve whether the user wants a new extraction or to append files
(for example, "追加文件", "继续添加", or "add these to the previous task").
Appending must preserve the existing extraction Task; it must never create a
replacement extraction Task, including after an error or resource-version conflict.
Use the extraction Task ID explicitly supplied by the user or unambiguously
identified by the current conversation's successful extraction response. If the
target is missing or ambiguous, ask which extraction Task to append to before
submitting work. Do not guess a target from recency or confuse a Parse Task ID
with an extraction Task ID. Verify the target using `task status <EXTRACTION_TASK_ID>`.

Only for a new extraction request, create one persistent extraction
Task on the service. Do not read Parse results into the Host and do not generate
the final field values locally. Preserve the user's extraction request verbatim
as `instruction`; do not replace it with a fixed schema or add field definitions.

If the request already supplies trustworthy File Asset IDs from xParse, skip
parsing. For an append request, go directly to the append command below. For a
new extraction request, create the Task directly. Repeat `--file-id` in source order:

```bash
xparse-cli task run --task-type extract \
  --instruction '<USER_REQUEST>' \
  --file-id <FILE_ASSET_ID>
```

For local files, including a single file, first create a new durable Parse Task
and preserve its accepted `task_id` and first `run_id` separately from the
existing `<EXTRACTION_TASK_ID>` when appending:

```bash
xparse-cli task run <FILE> --api auto
```

Use one `task run` for all local source files. Poll only that exact first Run
with `task status <TASK_ID> --run-id <FIRST_RUN_ID>` using the bounded backoff
defined above. Do not use an old Task, a rerun, or a continuation Run as an
implicit extraction source. If the first Run remains `scheduled` or `running`
after the polling budget, return its identifiers and current state instead of
creating the extraction Task.

When the exact first Run reports `completed`, fetch the stable Task resources:

```bash
xparse-cli task status <TASK_ID> --details
```

Create or append to the extraction Task only when all of these invariants hold in the details
response:

- `run.run_id` equals `<FIRST_RUN_ID>` and `run.status` is `completed`;
- `run.failed_count` is `0`;
- `run.completed_count` equals `run.total_count`;
- `run.total_count` equals the number of `resources`;
- every resource has a non-empty `file_id`.

Pass every `resources[].file_id`, in resource order, as a repeated `--file-id`.
Do not call `task export`, `task read`, or another content-returning command to
recover File Asset IDs. `partial_failed` and `failed` must stop before extraction.
Waiting states retain the same Task and first Run and require the documented user
action before polling resumes.

- Keep `--operation-id` unchanged only when replaying the same ambiguous
  extraction create attempt; do not reuse it for a new extraction request.
- The command returns `task_id`, `status`, and `result_page_url`. Use `task status <TASK_ID>` to inspect progress; do not guess an extraction Run ID.
- The result URL contains a short-lived grant in its fragment. Preserve the URL
  exactly as returned. Do not reconstruct it, persist it, print the grant, or
  send it to another API. Ask a capable Host to open the URL directly; if the
  Host cannot, return it as a clickable link.
- Creating the extraction Task submits asynchronous work. Open or return the result
  page when useful, and use `task status <TASK_ID>` for progress. Apply the bounded
  polling backoff above; stop polling when user action is needed or the budget expires.
- To read pure results use `task result <TASK_ID> --limit 50`. Follow `next_offset`
  with `--offset <NEXT_OFFSET> --snapshot <SNAPSHOT>`; on snapshot conflict restart
  from the first page rather than combining different task versions.
- To export use `task export <TASK_ID> --format json --output <DIRECTORY>` (or csv).
  Extraction writes `results.json` or `results.csv`; stdout contains the completion
  summary and output path. Only ready documents are exported; always report omissions
  and review warnings from the summary. Never claim partial results are complete.
- JSON contains `file_name` and business `result` values without evidence or agent
  internals. Missing values are null, identifiers remain strings, and object/array
  values remain structured. CSV serializes object/array cells as JSON text.
- Do not pass parse-only flags (`--api`, `--config`, `--password`, `--wait`, or
  automatic `--output`) to extraction creation. `task run` defaults to parse for
  compatibility; extraction requires explicit `--task-type extract`.
- `task add <TASK_ID>` uses the server's task type: pass local paths for parse,
  parsed `--file-id` values for extract. Parse add requests a new-files Run and
  retains `--operation-id` recovery; a failure after binding does not mean files
  were not added. Preserve identifiers and follow the returned recovery instructions.
  Existing `task rerun --mode new-files` remains supported.

#### Extraction billing and recovery

- Extraction uses its own 100 free pages per user per day, then normal billing.
  Read `extraction_quota.daily_pages_remaining` and `extraction_quota.reset_at`
  from `xparse-cli quota --output json`. These are server facts, not a local
  estimate. If `extraction_quota` is absent, the extraction allowance is unknown;
  do not infer 100 pages remaining or substitute the parse allowance.
  Parse allowance is not the extraction allowance. The server checks the whole
  file against remaining free pages plus paid funds; insufficient funds reject
  the file, without partially charging it. Successful settlement is once per
  user, Task and file; a new Task may incur another charge for the same file.
  Never create a replacement Task to recover a failure.
- Creation returns `operation_id`, including in structured error details when
  submission is ambiguous. Replay only with that same `--operation-id`; do not
  generate another ID or infer a new Task from a missing response. For recovery
  across an interrupted CLI process, provide and retain the ID before submission.
- Inspect `task status <TASK_ID> --details` for the exact `resource_id`,
  `error_code`, `billing_status`, `pending_result_id` and `agent_activity`.
- If `billing_status` is `pending_settlement`, the generated result is privately
  saved but not yet delivered. After funding, use
  `task retry <TASK_ID> --resource-id <RESOURCE_ID> --pending-result-id <PENDING_RESULT_ID>`.
  Preserve both IDs on ambiguous responses and replay that same request. This
  only settles and publishes the saved result: no model invocation or extra
  model budget. A repeated successful settlement returns the same result.
  A stale candidate returns a conflict; never silently replace it with a rerun.
- Settlement leaves the Task suspended. The delivered file is available through
  `task result` / `task export`; report that remaining work is still incomplete.
- A failed file with no pending result can be explicitly re-extracted using
  `task retry <TASK_ID> --resource-id <RESOURCE_ID>`. This operation has no
  idempotent replay guarantee. If its response is ambiguous, inspect status;
  do not automatically resend it. Retry multiple failed files serially, waiting
  for the current round to terminate and inspecting status before the next one.
- To continue a suspended Agent after explicit user direction, use
  `task resume <TASK_ID>` without parse `--run-id` or `--after-funding` flags.
  This continues other work and adds server-controlled execution budget.
  Preserve the returned `command_id` and `checkpoint_version`, including on
  failure, and replay with `--command-id <COMMAND_ID> --checkpoint-version <VERSION>`.
  Exact replay uses the original pair even if status has since changed. Settle
  pending results first. Never automatically loop resume or add budgets.

For an append request, send only the newly supplied File Asset IDs to the same
extraction Task after the parsing checks above. Do not execute
`task run --task-type extract` or resend old files. Preserve the existing
extraction instruction and fields; do not turn the append request into a new
`--instruction`. No extraction Run ID is accepted:

```bash
xparse-cli task add <EXTRACTION_TASK_ID> \
  --file-id <NEW_FILE_ASSET_ID>
```

Check that the returned `task_id` is the original `<EXTRACTION_TASK_ID>`, then
poll that same Task and use its returned `result_page_url`. If append fails,
follow the error's recovery instructions on the same Task; never fall back to
creating another extraction Task.

When MCP tools are available, the equivalent contracts are
`create_extraction_task(file_ids, instruction, operation_id)` and
`add_extraction_task_files(task_id, file_ids)`. Use `file_ids`; do not invent
Parse Job, Parse Task Run, or source-pointer parameters.

### Full document or conversion

Use one parse command:

```bash
xparse-cli parse <INPUT> --api auto
```

For PDFs, pass an output directory so long Markdown is not truncated in
terminal output. The CLI creates the directory when it does not exist:

```bash
xparse-cli parse report.pdf --api auto --output <DIR>
```

Read the saved result before requesting more detail. Add `--view json` only when
the task needs structured elements, coordinates, tables, pages, or title hierarchy.

### Server-generated document exports

When the user explicitly asks to export one document as DOCX, PDF, or XLSX,
explain that this requires the paid parse endpoint and obtain paid approval
before running the command. Request only the formats the user needs:

```bash
xparse-cli parse <INPUT> --api paid --export docx,pdf,xlsx --output <DIR>
```

- `--export` accepts `docx`, `pdf`, and `xlsx`; pass a comma-separated list or
  repeat the flag. The CLI removes duplicates, and XLSX automatically uses the
  table export scope.
- `--api paid` and `--output <DIR>` are required when `--export` is present.
  Immediately before downloading, the CLI resolves the selected AppKey or OAuth
  identity again so a long parse can refresh an expired OAuth token. It then
  downloads each successful export and verifies the saved file size.
- The output directory contains the ordinary parse result plus
  `<basename>.docx`, `<basename>.pdf`, and/or `<basename>.xlsx`. Read or return
  those local files as the task result. If a name would overwrite the input,
  the CLI uses `<basename>.export.<format>`. Do not expose backend download
  URLs, `file_id` values, or authorization details to the user.
- This single-document feature is separate from `task export`, which exports
  the Markdown results of a durable multi-document Task.

### Targeted reading, search, or extraction

For a local document, use:

```text
get_doc_info -> parse the complete document -> navigate -> extract
```

1. Run `get_doc_info <FILE>` and retain its exact `doc_id`.
2. Run `parse <FILE> --api auto` without `--page-range`. A successful complete
   local parse writes the navigation cache automatically.
3. Use `get_outline`, `search_text`, or `read_pages` to locate relevant content.
4. Batch the required `read_content` calls after navigation is complete.

There is no separate cache-preparation command. A successful complete local
`parse` is the only preparation step.

Page-range parses intentionally do not replace the complete-document navigation
cache. URL parses have no stable local `doc_id`, so use their direct parse output
instead of local navigation commands.

Read [navigation.md](references/navigation.md) before performing targeted
navigation or extraction.

## Efficiency and fallback rules

- Plan all navigation before reading sections; target no more than eight
  `read_content` calls per task and issue independent reads together.
- Prefer `search_text` for names, dates, amounts, and percentages. Read a full
  section only when its surrounding prose or table structure is needed.
- If an outline is truncated, drill down with `--parent-id`; do not guess IDs.
- Keep unrelated one-document parses serial. For a multi-document batch, use
  one durable Task instead of parallel `parse` commands.
- Retry a transient service failure once at most and only when its structured
  error says `retryable=true`. Stop immediately on any non-retryable service
  failure. Never silently skip a failure.
- For local documents, try this Skill before Python, PyMuPDF, pdfplumber, qpdf,
  OCR tools, image conversion, or custom scripts.
- If a document is encrypted or required input is missing, ask the user instead
  of trying alternate tools.
- Only fall back after xparse-cli clearly cannot complete the task, and explain why.

## Quick reference

| Goal | Command |
|------|---------|
| Parse with automatic free routing | `xparse-cli parse <FILE> --api auto` |
| Force free endpoint only | `xparse-cli parse <FILE> --api free` |
| Explicit paid parse | `xparse-cli parse <FILE> --api paid --auth-method oauth` |
| Save Markdown | `xparse-cli parse <FILE> --api auto --output <DIR>` |
| Save JSON | `xparse-cli parse <FILE> --api auto --view json --output <DIR>` |
| Export one document as DOCX/PDF/XLSX | `xparse-cli parse <FILE> --api paid --export docx,pdf,xlsx --output <DIR>` |
| Parse selected pages only | `xparse-cli parse <FILE> --api auto --page-range 1-5` |
| Encrypted document | `xparse-cli parse <FILE> --api auto --password <PWD>` |
| Character details | `xparse-cli parse <FILE> --api auto --view json --output <DIR> --include-char-details` |
| Show current quota | `xparse-cli quota --output json` |
| Run a durable local-file Task | `xparse-cli task run --files '<GLOB>' --api auto` |
| Create extraction from parsed File Assets | `xparse-cli task run --task-type extract --instruction '<REQUEST>' --file-id <FILE_ASSET_ID>` |
| Inspect stable Task resources | `xparse-cli task status <TASK_ID> --details` |
| Append newly parsed File Assets to an extraction Task | `xparse-cli task add <TASK_ID> --file-id <FILE_ASSET_ID>` |
| Rerun every Resource under a Task | `xparse-cli task rerun <TASK_ID> --mode all` |
| Add files and create a new Run | `xparse-cli task rerun <TASK_ID> --mode new-files --files '<GLOB>'` |
| Rerun selected Resources | `xparse-cli task rerun <TASK_ID> --mode selected-files --resource-id <RESOURCE_ID>` |
| Check an exact Task Run | `xparse-cli task status <TASK_ID> --run-id <RUN_ID>` |
| Read one Task result | `xparse-cli task read <TASK_ID> <FILE_OR_RESOURCE> --run-id <RUN_ID>` |
| Export all completed results | `xparse-cli task export <TASK_ID> --run-id <RUN_ID> --output <DIR>` |
| Inspect per-file failures | `xparse-cli task debug <TASK_ID> --run-id <RUN_ID>` |
| Continue one existing Resource after Run error 40423 | `xparse-cli task continue <TASK_ID> --password <PASSWORD>` |
| Continue multiple existing Resources after Run error 40423 | `xparse-cli task continue <TASK_ID> --password <SELECTOR>=<PASSWORD> --password <SELECTOR>=<PASSWORD>` |
| Resume after paid approval | `xparse-cli task resume <TASK_ID> --run-id <RUN_ID> --approve-paid` |
| Resume after funding | `xparse-cli task resume <TASK_ID> --run-id <RUN_ID> --after-funding` |
| Start local navigation | `xparse-cli get_doc_info <FILE>` |
| Show cached outline | `xparse-cli get_outline <DOC_ID>` |
| Search cached text | `xparse-cli search_text <DOC_ID> <PATTERN>` |

`--output` accepts a directory, not an output filename. The CLI creates a missing
directory and writes `<basename>.md` or `<basename>.json` inside it.

## Authentication boundary

- The CLI supports AppKey, Device OAuth, and browser PKCE as documented in
  [authentication.md](references/authentication.md).
- Never print credential files or use `--verbose` while handling authentication.
- An explicitly selected authentication method must fail as that method; do not
  silently retry with another credential type.

## Setup and command discovery

Check installation with `xparse-cli version`. The package requires Node.js 18
or newer and can be installed with:

```bash
npm i -g xparse-cli
```

For users in China, use the npmmirror registry:

```bash
npm i -g xparse-cli --registry=https://registry.npmmirror.com
```

Use this Skill and its references as the command index. When live discovery is
necessary, read complete `xparse-cli --help`, then the complete help for the exact
command. Do not truncate help output with `head`, `tail`, or a fixed `sed` range.

Stop on unsupported or corrupt files, invalid credentials, exhausted quota,
missing paid approval, any non-retryable service failure, or a transient
failure after its single allowed Agent-layer retry.

## References

- [navigation.md](references/navigation.md): targeted outline, search, page, and content workflow.
- [task-runtime.md](references/task-runtime.md): durable multi-file routing, states, result access, and recovery.
- [authentication.md](references/authentication.md): AppKey, Device OAuth, and browser authentication.
- [cli-guidance.md](references/cli-guidance.md): modes, output, parameters, and limits.
- [api-reference.md](references/api-reference.md): response fields and service error codes.
- [error-handling.md](references/error-handling.md): retry, stop, and paid-approval decisions.
- [textin-key-setup.md](references/textin-key-setup.md): standalone legacy AppKey setup.
