# Targeted Document Navigation

Use this workflow when the user needs selected facts, tables, pages, or
sections from a local document. For a complete Markdown or JSON result, use the
normal `parse` workflow in `SKILL.md`.

Inside WorkBuddy, insert `--profile workbuddy` immediately after `xparse-cli`
for every command. Apply the task-context rule from `SKILL.md` only to the first
xParse command for the user request.

## Standard workflow

1. Run `get_doc_info <FILE>` to validate the local file and obtain its stable
   `doc_id` and `page_count` without a service call.
2. Run `parse <FILE> --view json`. A successful parse writes
   the navigation cache and exposes the same `doc_id` in its JSON result or
   diagnostic metadata.
3. Complete navigation with `get_outline`, `search_text`, or `read_pages`.
4. Read only the selected elements with `read_content`.

Do not generate the deprecated cache-preparation command. If `parse` succeeds
but does not expose a usable `doc_id`, stop and update the Connector-managed CLI
instead of inventing an ID or reparsing through another tool.

## Plan-then-Batch execution

Finish navigation before reading content:

1. List every section or fact needed for the answer.
2. If an outline is truncated, complete all relevant `--parent-id` drill-downs.
3. Use scoped searches to locate precise facts inside large sections.
4. Issue independent `read_content` and `search_text` calls in one parallel
   tool-call message when the host supports parallel calls.

Target at most eight `read_content` calls. Prefer a parent section over many
small child reads, and prefer a search hit's context when it already answers
the question.

## Navigation decision table

| User intent | Action sequence |
|---|---|
| Known section, complete outline | `get_outline` → `read_content` |
| Known section, truncated outline | `get_outline` → drill down with `--parent-id` → `read_content` |
| Known keyword, unknown location | `search_text` → use hit context or read the returned element |
| Known keyword within a section | `get_outline` → `search_text --scope <SECTION_ID>` → optional `read_content` |
| Known page range | `read_pages <DOC_ID> <START> <END>`; at most 20 pages per call |
| No usable headings | `search_text` → fall back to bounded `read_pages` |

Never guess an `element_id`. When `truncated=true`, drill down before reading a
section.

## Command reference

| Command | Purpose |
|---|---|
| `xparse-cli get_doc_info <FILE>` | Return local `doc_id`, `page_count`, and document type |
| `xparse-cli get_outline <DOC_ID> [--depth N] [--parent-id <ID>]` | Read the cached title hierarchy |
| `xparse-cli search_text <DOC_ID> <PATTERN> [--scope <ID>]` | Search cached text and return context plus element IDs |
| `xparse-cli read_content <DOC_ID> <ELEMENT_ID>` | Read one section, table, or paragraph |
| `xparse-cli read_pages <DOC_ID> <START> <END>` | Read a bounded page range from cache |
| `xparse-cli get_confidence <DOC_ID> --element-id <ID>` | Inspect OCR confidence only when quality is uncertain |

## Recovery and stopping rules

- Cache miss after a successful parse: stop and report a CLI compatibility
  problem. Do not reparse through a second Skill-layer retry loop.
- Unknown `doc_id`: rerun `get_doc_info` for the exact local path.
- Unknown `element_id`: refresh the outline or search result; never guess.
- Page range too large: split it into ranges of at most 20 pages.
- Exhausted quota, unsupported or encrypted input, invalid credentials, or a
  repeated service failure: follow `error-handling.md` and stop when it says to
  request user action.
