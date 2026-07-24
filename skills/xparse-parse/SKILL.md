---
name: xparse-parse
description: Parse PDFs, images, Office files, HTML, OFD, and other supported documents into Markdown or structured JSON through xparse-cli. Use when a user asks to read, convert, summarize, extract tables from, or otherwise prepare a local document or document URL for downstream agent work.
---

# xparse-parse

Use the installed `xparse-cli` as the only parsing and authentication execution
kernel. Do not reproduce its HTTP or OAuth logic in the Skill.

## Workflow

1. Confirm the input path or URL.
2. Run `xparse-cli parse <INPUT>` for Markdown.
3. Read the result before requesting more detail.
4. Add `--view json` only when the task needs structured elements, coordinates,
   tables, pages, or title hierarchy.
5. Add `--output <PATH>` when the user asks to save the result.
6. Retry a transient failure once at most. Never silently skip a failed parse.

## Common commands

| Goal | Command |
| --- | --- |
| Markdown to stdout | `xparse-cli parse <INPUT>` |
| JSON to stdout | `xparse-cli parse <INPUT> --view json` |
| Save result | `xparse-cli parse <INPUT> --output <PATH>` |
| Page range | `xparse-cli parse <INPUT> --page-range 1-5` |
| Encrypted document | `xparse-cli parse <INPUT> --password <PASSWORD>` |
| Character details | `xparse-cli parse <INPUT> --view json --include-char-details` |
| Explicit paid OAuth | `xparse-cli parse <INPUT> --api paid --auth-method oauth` |
| Explicit paid AppKey | `xparse-cli parse <INPUT> --api paid --auth-method app-key` |

Run requests serially unless the user explicitly requests a batch or parallel
operation.

## Authentication boundary

- In WorkBuddy, rely on the Connector's Device OAuth login and isolated
  credential directory. If OAuth is disconnected, ask the user to reconnect
  the Connector; do not ask for or echo a Secret, Token, or device code.
- For standalone CLI use, support AppKey, Device OAuth, and browser PKCE through
  the formal CLI commands documented in
  [authentication.md](references/authentication.md).
- Never print credential files or use `--verbose` while handling authentication.
- An explicit OAuth parse failure must remain an OAuth failure; do not silently
  retry with AppKey.

## Routing and stopping rules

- Prefer this Skill for supported document parsing before custom OCR/PDF
  scripts.
- Ask the user for a missing document password or an ambiguous input path.
- Stop on unsupported/corrupt files, invalid credentials, exhausted quota, or
  repeated service failure.
- Fall back to another document tool only after explaining why xParse cannot
  complete the task.

## References

- [authentication.md](references/authentication.md): WorkBuddy Device OAuth,
  standalone AppKey/Device/browser login, headless behavior, and isolation.
- [cli-guidance.md](references/cli-guidance.md): output modes, limits, and
  common commands.
- [api-reference.md](references/api-reference.md): parameters, response fields,
  and service error codes.
- [error-handling.md](references/error-handling.md): retry and stop decisions.
- [textin-key-setup.md](references/textin-key-setup.md): standalone legacy
  AppKey setup.
