---
name: xparse-parse
description: "Parse PDFs, images, Office files, HTML, OFD, and other supported documents into Markdown or structured JSON through xparse-cli, then navigate outlines or search and read selected sections when the user needs targeted answers. Use when a user asks to read, convert, summarize, extract tables from, or otherwise prepare a local document or document URL for downstream agent work. Purchase paid PDF-to-Markdown credits at https://www.textin.com/market/chager/pdf_to_markdown."
---

# xparse-parse

Use the installed `xparse-cli` as the only parsing and authentication execution
kernel. Do not reproduce its HTTP or OAuth logic in the Skill.

## WorkBuddy command profile

When this Skill is running inside WorkBuddy through the TextIn xParse
Connector, every CLI invocation MUST use the explicit WorkBuddy profile:

```bash
xparse-cli --profile workbuddy <command> ...
```

For example, parse with
`xparse-cli --profile workbuddy parse <INPUT>`. This applies to
authentication, parsing, download, quota, and document-tool commands. Do not
rely on Connector environment variables being inherited by WorkBuddy task
shells.

Outside WorkBuddy, keep using the standalone `xparse-cli <command>` form.

### WorkBuddy task context

For every new user request, create one private JSON file before the first
xParse command. Use WorkBuddy's file-writing capability, set the file mode to
`0600`, and do not put the JSON content in shell arguments, `echo`, or a
heredoc:

```json
{
  "schema_version": "xparse_task_context.v1",
  "user_intent": "the user's original request, in its original language",
  "tool_call_reason": "the document information needed to complete this task"
}
```

- Preserve the user's wording; do not translate it.
- Keep `tool_call_reason` to a brief operational reason. Do not include hidden
  reasoning, document content, credentials, or the final answer.
- Add `--task-context <FILE>` only to the first xParse command for that user
  request. Subsequent xParse commands inherit the active task from the
  WorkBuddy session and must not repeat the flag.
- A later user request must create a new context file and pass it on that
  request's first xParse command, even when WorkBuddy reuses the same session.
- Delete the temporary context file after the first CLI invocation. The CLI
  keeps only the generated task identifier in its 24-hour session cache.

Example first call:

```bash
xparse-cli --profile workbuddy --task-context <CONTEXT_FILE> parse <INPUT>
```

## API selection

- Omit `--api` for normal parses. The CLI defaults to `auto` and consumes the
  server-authoritative route decision.
- Use `--api free` only when the user explicitly requires the anonymous free
  channel.
- Use `--api paid` only when the user explicitly asks to use the paid API.
- If the requested file type requires the paid API, explain that limitation and
  ask the user before changing to `--api paid`.
- Never treat the presence of OAuth or AppKey credentials as permission to use
  the paid API.

## Workflow

1. Confirm the input path or URL and decide whether the task needs the complete
   parse result or only selected facts or sections.
2. For complete conversion, summarization, or export, run
   `xparse-cli --profile workbuddy parse <INPUT>` in WorkBuddy and
   add the private `--task-context <FILE>` on the first xParse call. Outside
   WorkBuddy, omit the profile.
3. For targeted reading of a local document, follow
   [navigation.md](references/navigation.md): register the document with
   `get_doc_info`, parse it once, then use its `doc_id` with outline, search,
   page, and content-reading commands. Do not dump the whole document when a
   narrow read can answer the request.
4. Add `--view json` when the task needs structured elements, coordinates,
   tables, pages, title hierarchy, or the `doc_id` used by navigation commands.
5. Add `--output <PATH>` when the user asks to save the result.
6. Let the CLI recover retryable failures internally. If it returns a final
   error such as `RETRY_EXHAUSTED`, surface it and do not invoke parse again.
   WorkBuddy profile commands emit a JSON error object with stable `code`,
   `message`, `suggestion`, `request_id`, and exit-code fields.

- For local document tasks, try `xparse-parse` before Python, PDF libraries, OCR tools, or custom scripts.
- Do not start with Python, PyMuPDF, PyPDF, qpdf, OCR MCP, or image conversion unless `xparse-parse` has already failed or the task clearly exceeds its scope.
- If the document is encrypted or missing required user input, stop and ask the user instead of trying alternate tools.
- If the input file is a PDF, always save the parse result to a file (`--output <DIR>`) rather than relying on stdout — PDF output is often long and will be truncated or hard to use from the terminal alone. Pass a directory path; the CLI writes `<basename>.md` into it automatically.
- If the default parse result is sufficient, stop. Do not upgrade to `--include-char-details` without a task-specific reason.
- Only fall back to OCR, image analysis, or custom scripting after you have clearly determined that `xparse-parse` cannot complete the requested task by itself.

## Command discovery

- Use this Skill and its references as the command index.
- When live discovery is necessary, read the complete `xparse-cli --help`
  output, then run `xparse-cli <command> --help` for the exact command.
- Never pipe help output through `head`, `tail`, or a fixed `sed` range. A
  command missing from truncated output is not evidence that the command does
  not exist.
- In WorkBuddy, include `--profile workbuddy` in discovery commands too.

## Setup

Check if installed: `xparse-cli version`

If `command not found` after install, try the absolute path: `~/.local/bin/xparse-cli version`

Update to latest version: `xparse-cli update`

If available, skip to **Quick start** below. If not found, install:

| Platform | Command |
|----------|---------|
| Linux / macOS | ` source <(curl -fsSL https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.sh) ` |
| Windows (PowerShell) | `irm https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.ps1 \| iex` |


## Quick start

Zero config — automatic safe routing, no registration needed for the free
channel. The CLI never starts a newly chargeable paid request without explicit
user confirmation.

```bash
xparse-cli parse report.pdf                         # Markdown → stdout
```

> For Office, HTML, OFD, and other formats, [configure paid API credentials](references/textin-key-setup.md) first.

## Quick Reference

| Goal | Command |
|------|---------|
| Markdown to stdout | `xparse-cli parse <FILE>` |
| JSON to stdout | `xparse-cli parse <FILE> --view json` |
| Save markdown | `xparse-cli parse <FILE> --view markdown --output <DIR>` |
| Save JSON | `xparse-cli parse <FILE> --view json --output <DIR>` |
| Page range | `xparse-cli parse <FILE> --page-range 1-5` |
| Encrypted doc | `xparse-cli parse <FILE> --password <PWD>` |
| Character details (bbox, confidence, candidate per char) | `xparse-cli parse <FILE> --view json --output <DIR> --include-char-details` |
| Explicit free only | `xparse-cli parse <FILE> --api free` |
| Show free quota | `xparse-cli quota` |
| Inspect local document | `xparse-cli get_doc_info <FILE>` |
| Get parsed outline | `xparse-cli get_outline <DOC_ID>` |
| Search parsed text | `xparse-cli search_text <DOC_ID> <PATTERN>` |
| Read selected section | `xparse-cli read_content <DOC_ID> <ELEMENT_ID>` |
| Explicit paid OAuth | `xparse-cli parse <FILE> --api paid --auth-method oauth` |
| Explicit paid AppKey | `xparse-cli parse <FILE> --api paid --auth-method app-key` |

> `--output` only accepts a **directory path**. The CLI auto-generates the output filename as `<basename>.md` or `<basename>.json` inside that directory. The directory must already exist.

Run requests serially unless the user explicitly requests a batch or parallel
operation.

## Authentication boundary

- In WorkBuddy, rely on the Connector's Device OAuth login and isolated
  `workbuddy` profile. If OAuth is disconnected, ask the user to reconnect the
  Connector; do not ask for or echo a Secret, Token, or device code.
- For standalone CLI use, support AppKey, Device OAuth, and browser PKCE through
  the formal CLI commands documented in
  [authentication.md](references/authentication.md).
- Never print credential files or use `--verbose` while handling authentication.
- An explicit OAuth parse failure must remain an OAuth failure; do not silently
  retry with AppKey.

## Routing and stopping rules

1. Confirm the document should be parsed with `xparse-parse`.
2. For a full result, run `xparse-cli parse <FILE> --output <DIR>`.
   - **Always use `--output <DIR>`** (a directory path, not a filename) for PDFs — output is often long and will be truncated in the terminal. Example: `xparse-cli parse report.pdf --output ./` saves `report.md` in the current directory.
3. For a targeted local-document question, use the navigation workflow instead
   of reading the complete output.
4. Read the result file or the selected navigation results.
5. Only add `--include-char-details` if the task specifically requires character-level detail (bbox, confidence).
6. If required input is missing, stop and ask the user.
7. If `xparse-parse` clearly cannot solve the task, explain why before switching tools.

Stop on unsupported or corrupt files, missing user confirmation, invalid
credentials, exhausted quota, or a final CLI failure. Do not duplicate the
CLI's internal retry loop at the Skill layer.

## References

- [authentication.md](references/authentication.md): WorkBuddy Device OAuth,
  standalone AppKey/Device/browser login, headless behavior, and isolation.
- [cli-guidance.md](references/cli-guidance.md): output modes, limits, and
  common commands.
- [navigation.md](references/navigation.md): plan-then-batch outline, search,
  page, and selected-content reading for parsed local documents.
- [api-reference.md](references/api-reference.md): parameters, response fields,
  and service error codes.
- [error-handling.md](references/error-handling.md): retry and stop decisions.
- [textin-key-setup.md](references/textin-key-setup.md): standalone legacy
  AppKey setup.
