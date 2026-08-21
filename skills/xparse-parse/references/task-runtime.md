# Durable Task Runtime

Use this workflow for two or more local documents, or whenever the user asks
for a persistent Task that can be inspected or resumed later. Its control-plane
routes and OAuth authentication are available in domestic and overseas
environments. URLs remain on the single-document `parse` path.

Inside WorkBuddy, prefix every example with `xparse-cli --profile workbuddy`.
The examples below omit that prefix only for readability.

## Start one Task

Use explicit file arguments or a quoted glob. The CLI expands globs
deterministically, removes duplicates, uploads the files, creates one Task and
one Run, and returns JSON containing the `task_id`, `run_id`, and initial status
without waiting for document processing to finish.

```bash
xparse-cli task run a.pdf b.pdf --api auto
xparse-cli task run --files 'contracts/*.pdf' --api auto
```

In WorkBuddy, stderr contains line-delimited `xparse_event.v1` progress events
and stdout contains one final `xparse_task_submission.v1` JSON object. Upload
progress is intentionally coarse (`completed_files` / `total_files`) and does
not report byte-level percentages.

Rules:

- Preserve `operation_id`, `task_id`, and `run_id` as soon as they appear.
- Treat accepted Task identity as irreversible. An `operation_id` alone permits
  only the documented single idempotent submission retry; after `task_id` or
  `run_id` is observed, do not issue `task run` again for that submission.
- Keep workflow and billing decisions separate. A waiting or rejected billing
  route never changes a multi-document Task into individual `parse` calls.
- Use `--api auto` unless the user explicitly requires the free endpoint or has
  approved paid execution.
- `auto` and `free` use the Agent free-first Task endpoint and fail closed. An
  overseas environment may explicitly return `TASK_FREE_MODE_UNAVAILABLE` when
  no equivalent free billing source exists. Never retry as `paid` without
  approval and never replace the batch with serial `parse` commands.
- One `operation_id` identifies one logical submission. The CLI namespaces it
  into Task and Run idempotency keys. If a transient failure, timeout, or lost
  response makes the outcome ambiguous, retry the same command once with
  `--operation-id <OPERATION_ID>`. The same ID with changed files, mode, or
  config returns `IDEMPOTENCY_CONFLICT`; use a new ID only for a genuinely new
  operation.
- Check the exact returned Run with `task status <TASK_ID> --run-id <RUN_ID>`
  rather than starting the Task again. Do not infer the target from “latest”.
- In Agent workflows, keep the default submit-and-return behavior. A standalone
  user can explicitly add `--wait` to keep the command in the foreground.
- Use `task export --out-dir <DIR>` after completion. `task run --out-dir` only
  exports when combined with explicit `--wait` and the Run reaches a terminal
  state.
- Task inputs must be non-empty local files. Use `parse` for URLs.

Task-level parse defaults can be provided as a JSON file:

```bash
xparse-cli task run --files 'docs/*.pdf' --config ./parse-config.json --api auto
```

Do not put `document.password` in that file. Passwords are resource-specific.

## Inspect and consume results

```bash
xparse-cli task status <TASK_ID> --run-id <RUN_ID>
xparse-cli task read <TASK_ID> contract-a.pdf --run-id <RUN_ID>
xparse-cli task read <TASK_ID> <RESOURCE_ID> --run-id <RUN_ID>
xparse-cli task export <TASK_ID> --run-id <RUN_ID> --out-dir ./task-output
xparse-cli task debug <TASK_ID> --run-id <RUN_ID>
```

- Prefer `read` when the user's question needs one or a few named documents;
  do not load every result into context. The CLI requests only the selected
  Resource body.
- Use `export` when the user requests all outputs or local files for downstream
  processing. It pages result metadata and fetches completed bodies one at a
  time. Read `task-manifest.json` to distinguish completed and failed resources,
  including colliding basenames.
- Use `debug` only for compact per-file errors and recovery evidence.
- Pass the preserved `--run-id` to `status`, `read`, `export`, and `debug` when
  continuing an Agent workflow. Omitting it selects the latest Run and is only
  appropriate for an interactive user who explicitly wants the latest state.

If `read` or `export` emits `xparse_error.v1`, apply the main Skill's structured
error gate before any other command. A non-retryable result-access failure does
not authorize another selector, omitting `--run-id`, switching between read and
export, running debug on a completed Run, creating another Task, serial parsing,
or substituting cached content. Report the exact Task/Run/Resource/request IDs
and stop. Only a user-confirmed external remediation or explicit new request
can reopen the action, and it must reuse the preserved Task and Run IDs.

For Agent polling, issue one-shot status checks with bounded backoff: 2, 5, 10,
20, then 30 seconds. Stop after roughly two minutes in one turn if the Run is
still `scheduled` or `running`; report the IDs and current status so a later
turn can continue. Do not hold `task run` open or duplicate the submission.

## State decisions

| Run state | Next action |
|-----------|-------------|
| `scheduled`, `running` | Keep both IDs and check later with `task status <TASK_ID> --run-id <RUN_ID>`; do not create another Task. |
| `completed` | Read selected results or export all results. A non-retryable result-access error is terminal for the current request; do not regress to submission. |
| `partial_failed`, `failed` | Run `task debug`, preserve successful results, and handle each failed resource. |
| `waiting_paid_authorization` | Stop and ask the user to approve paid execution. Do not infer approval from login state. |
| `waiting_funds` | Stop and ask the user to add sufficient funds before retrying settlement. |
| `cancelled` | Report cancellation; do not recreate work without a new request. |

Waiting states are successful Task state projections with an actionable
`next_action`; they are not completed user work. After observing either waiting
state, run no additional xParse diagnostics or alternate workflow. Wait for the
human action, then resume the exact identifiers below.

If an explicit `--wait` times out, keep the returned Task ID and use
`task status`. Do not start an identical Task merely because the local process
stopped waiting.

After the required human action, resume the exact waiting Run:

```bash
xparse-cli task resume <TASK_ID> --run-id <RUN_ID> --approve-paid
xparse-cli task resume <TASK_ID> --run-id <RUN_ID> --after-funding
```

Use `--approve-paid` only after explicit paid authorization. Use
`--after-funding` only after the user confirms funds were added. The commands
are mutually exclusive and resume submission returns without waiting by
default; poll the same Run ID afterward. Never recreate the Task as paid, split
the files into another Task, or process them with serial/parallel `parse` calls.

## Continue password-protected files

After `task debug` identifies password failures, obtain passwords from the user
and pass exactly one JSON object through stdin. Keys can be a resource ID, file
name, or original path. Avoid shell history by writing the JSON to a private
`0600` file or another protected stdin source; delete the temporary file after
use.

```bash
xparse-cli task continue <TASK_ID> --passwords-stdin < ./passwords.json
```

`continue` updates access only for matched resources and reruns those resources.
It submits the selected rerun and returns by default. Preserve the new Run ID,
poll it separately, then call `task export --run-id <RUN_ID> --out-dir
./task-output` after completion. `continue --out-dir` is valid only together
with explicit `--wait`. It does not reparse already successful files. Never
pass passwords as command arguments, put them in `--config`, print them, or copy
them into the final answer.

## WorkBuddy task context

The private `xparse_task_context.v1` file described in the main Skill applies to
Task Runtime too. Add `--task-context <FILE>` only to the first xParse invocation
for the user request, including when that first invocation is `task run`, and
delete the file immediately after the command consumes it.
