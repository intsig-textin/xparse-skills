---
name: xparse-parse
description: Parse documents into clean markdown or structured JSON via the xparse-cli. Use this skill when the user provides a PDF, image, Office file, HTML, OFD, or other supported document and wants it read, converted, summarized, or prepared for downstream agent use. Handles encrypted PDFs, page ranges, markdown/text output, and detailed structured extraction. Prefer this skill whenever the task starts from a local file or document URL and the first step is to turn it into agent-friendly content rather than manually inspect the raw file.
compatibility: Requires the `xparse-cli` binary. Free API supports PDF and images with zero config; paid API unlocks additional formats (Doc(x)/Ppt(x)/Xls(x)/HTML/OFD/RTF, etc.) and requires paid credentials configured via `xparse-cli auth` (recommended), or `XPARSE_APP_ID`/`XPARSE_SECRET_CODE` env vars.

---

# xparse-parse

## Overview

Use the parse CLI first. Read the result before requesting any more detail.

## Routing Rules

- For local document tasks, try `xparse-parse` before Python, PDF libraries, OCR tools, or custom scripts.
- Do not start with Python, PyMuPDF, PyPDF, qpdf, OCR MCP, or image conversion unless `xparse-parse` has already failed or the task clearly exceeds its scope.
- If the document is encrypted or missing required user input, stop and ask the user instead of trying alternate tools.
- If the default parse result is sufficient, stop. Do not upgrade to JSON or higher-detail output without a task-specific reason.
- Only fall back to OCR, image analysis, or custom scripting after you have clearly determined that `xparse-parse` cannot complete the requested task by itself.

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

Zero config — free API, no registration needed. Supports **PDF and images** only.

```bash
xparse-cli parse report.pdf                         # Markdown → stdout
```

> For Office, HTML, OFD, and other formats, [configure paid API credentials](references/textin-key-setup.md) first.

## Quick Reference

| Goal | Command |
|------|---------|
| Markdown to stdout | `xparse-cli parse <FILE>` |
| JSON to stdout | `xparse-cli parse <FILE> --view json` |
| Save markdown | `xparse-cli parse <FILE> --view markdown --output <DIR\|FILE>` |
| Save JSON | `xparse-cli parse <FILE> --view json --output <DIR\|FILE>` |
| Page range | `xparse-cli parse <FILE> --page-range 1-5` |
| Encrypted doc | `xparse-cli parse <FILE> --password <PWD>` |
| Character details (bbox, confidence, candidate per char) | `xparse-cli parse <FILE> --view json --output <DIR\|FILE> --include-char-details` |

> `--output <DIR>` auto-generates `<basename>.md` or `<basename>.json`; `--output <FILE>` writes directly.

Run parse requests serially by default. Do not start another until the previous result has been inspected. Only run in parallel when the user explicitly asks for batching or parallel processing and paid API credentials are configured.

For more commands, paid API setup, and output options, see [cli-guidance.md](references/cli-guidance.md).

## Default Path

1. Confirm the document should be parsed with `xparse-parse`
2. Run `xparse-cli parse <FILE>`
3. Read the markdown result
4. If the task needs more structure, then and only then upgrade to JSON
5. If required input is missing, stop and ask the user
6. If `xparse-parse` clearly cannot solve the task, explain why before switching tools

## When to Stop

Stop and ask the user if:

- The free limit is hit (do not retry)
- The file is too large or unsupported
- The document requires information the user has not provided

If the error looks temporary, retry once at most. Never silently skip a failed parse.

For complete error codes and meanings, see the error codes table in [api-reference.md](references/api-reference.md).

## Learn More

Detailed references in skill directory:

- **[api-reference.md](references/api-reference.md)** — Parameters, response fields, error codes
- **[cli-guidance.md](references/cli-guidance.md)** — Commands, paid API, output views, troubleshooting
- **[error-handling.md](references/error-handling.md)** — Agent decision logic (when to stop, retry rules)
- **[textin-key-setup.md](references/textin-key-setup.md)** — Configure paid API credentials
