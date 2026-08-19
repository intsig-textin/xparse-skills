# Durable Task Runtime

Use this workflow for two or more local documents, or whenever the user asks
for a persistent Task that can be inspected or resumed later. It is currently
available only in the domestic environment. URLs remain on the single-document
`parse` path.

Inside WorkBuddy, prefix every example with `xparse-cli --profile workbuddy`.
The examples below omit that prefix only for readability.

## Start one Task

Use explicit file arguments or a quoted glob. The CLI expands globs
deterministically, removes duplicates, uploads the files, creates one Task and
one Run, waits by default, and prints JSON containing the `task_id` and `run_id`.

```bash
xparse-cli task run a.pdf b.pdf --api auto --out-dir ./task-output
xparse-cli task run --files 'contracts/*.pdf' --api auto --out-dir ./task-output
```

Rules:

- Preserve `task_id`; all later commands use it.
- Use `--api auto` unless the user explicitly requires the free endpoint or has
  approved paid execution.
- `auto` and `free` use the domestic Agent free-first Task endpoint and fail
  closed. Never retry them as `paid` without approval.
- Use `--wait=false` only when returning immediately is useful. Check that Task
  later with `task status` rather than starting it again.
- `--out-dir` is optional for starting a Task. When present, the CLI exports all
  completed results and writes `task-manifest.json` after a terminal Run.
- Task inputs must be non-empty local files. Use `parse` for URLs.

Task-level parse defaults can be provided as a JSON file:

```bash
xparse-cli task run --files 'docs/*.pdf' --config ./parse-config.json --api auto
```

Do not put `document.password` in that file. Passwords are resource-specific.

## Inspect and consume results

```bash
xparse-cli task status <TASK_ID>
xparse-cli task read <TASK_ID> contract-a.pdf
xparse-cli task read <TASK_ID> <RESOURCE_ID>
xparse-cli task export <TASK_ID> --out-dir ./task-output
xparse-cli task debug <TASK_ID>
```

- Prefer `read` when the user's question needs one or a few named documents;
  do not load every result into context.
- Use `export` when the user requests all outputs or local files for downstream
  processing. Read `task-manifest.json` to distinguish completed and failed
  resources, including colliding basenames.
- Use `debug` only for compact per-file errors and recovery evidence.
- `status`, `read`, `export`, and `debug` default to the latest Run. Preserve an
  explicit `--run-id` only when the user needs an earlier Run.

## State decisions

| Run state | Next action |
|-----------|-------------|
| `scheduled`, `running` | Continue the default wait or check later with `task status`. |
| `completed` | Read selected results or export all results. |
| `partial_failed`, `failed` | Run `task debug`, preserve successful results, and handle each failed resource. |
| `waiting_paid_authorization` | Stop and ask the user to approve paid execution. Do not infer approval from login state. |
| `waiting_funds` | Stop and ask the user to add sufficient funds before retrying settlement. |
| `cancelled` | Report cancellation; do not recreate work without a new request. |

If waiting times out, keep the returned Task ID and use `task status`. Do not
start an identical Task merely because the local process stopped waiting.

The current CLI does not expose a paid-authorization resume command. When a Run
is `waiting_paid_authorization`, preserve the Task ID and obtain the user's
decision, but do not invent a continuation command or recreate the Task as paid.

## Continue password-protected files

After `task debug` identifies password failures, obtain passwords from the user
and pass exactly one JSON object through stdin. Keys can be a resource ID, file
name, or original path. Avoid shell history by writing the JSON to a private
`0600` file or another protected stdin source; delete the temporary file after
use.

```bash
xparse-cli task continue <TASK_ID> --passwords-stdin --out-dir ./task-output < ./passwords.json
```

`continue` updates access only for matched resources and reruns those resources.
It does not reparse already successful files. Never pass passwords as command
arguments, put them in `--config`, print them, or copy them into the final
answer.

## WorkBuddy task context

The private `xparse_task_context.v1` file described in the main Skill applies to
Task Runtime too. Add `--task-context <FILE>` only to the first xParse invocation
for the user request, including when that first invocation is `task run`, and
delete the file immediately after the command consumes it.
