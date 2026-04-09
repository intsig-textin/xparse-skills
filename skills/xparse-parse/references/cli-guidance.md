# CLI Guidance

> Installation and quick start see [SKILL.md](../SKILL.md).

## Paid API (optional, higher quota)

```bash
xparse-cli auth                                     # Interactive credential setup
```

Or set environment variables:

```bash
export XPARSE_APP_ID=your_app_id
export XPARSE_SECRET_CODE=your_secret_code
```

| `--api` value | Behavior |
|---------------|----------|
| _(omitted)_ | Paid if credentials exist, else free |
| `free` | Force free API |
| `paid` | Force paid API |

Credential priority: CLI flags → env vars → `~/.xparse-cli/config.yaml`

See [textin-key-setup.md](textin-key-setup.md) for full credential setup.

## Output Views

Choose how to see results:

```bash
# Markdown to stdout (default)
xparse-cli parse document.pdf

# JSON (explicit)
xparse-cli parse document.pdf --view json

# Save to directory (auto-names as <basename>.json/.md)
xparse-cli parse document.pdf --output ./result/

# Save to specific file (default view is markdown)
xparse-cli parse document.pdf --output result.md
```

## Common Scenarios

| Scenario | Command |
|----------|---------|
| Read document content | `xparse-cli parse doc.pdf` |
| Inspect parse result as JSON | `xparse-cli parse doc.pdf --view json` |
| Specific pages only | `xparse-cli parse doc.pdf --page-range 1-5` |
| Encrypted document | `xparse-cli parse doc.pdf --password secret123` |
| Save to directory | `xparse-cli parse doc.pdf --output ./result/` |
| Save to specific file | `xparse-cli parse doc.pdf --output ./parsed.md` |

## Advanced Options

| Scenario | Command |
|----------|---------|
| Single page only | `xparse-cli parse doc.pdf --page-range 3` |
| Multiple page ranges | `xparse-cli parse doc.pdf --page-range 1-2,5-10` |
| Character details & coordinates | `xparse-cli parse doc.pdf --view json --include-char-details --output ./parsed.json` |
| Force paid API | `xparse-cli parse doc.pdf --api paid` |

## API Capabilities — What You Get by Default

CLI automatically enables these capabilities (you don't need to specify them):

| Capability | What It Does |
|-----------|--------------|
| Hierarchy | Document structure (headings, nesting) |
| Inline objects | Embedded content (links, mentions) |
| Image data | Image extraction and analysis |
| Table structure | Table parsing with cell information |
| Pages | Page-level metadata |
| Title tree | Document outline/TOC |

**Exception:** Character details (`--include-char-details`) must be explicitly enabled—it increases response size significantly.

## Understanding Output

**JSON view** — Complete structured result with all parsed elements, title tree, and metadata.
For field details, see [api-reference.md](api-reference.md).

**Markdown view** — Clean, readable text format. Good for content summarization and review.

## Exit Codes

| Code | Meaning | Next Step |
|------|---------|-----------|
| 0 | Success | Parse succeeded, check stdout |
| 1 | API or network error | Check stderr for details; may retry |
| 2 | Parameter error | Check command syntax; fix and retry |
| 3 | API returned structured error | See stderr for error code + fix |

## Troubleshooting

For all error codes, recovery actions, and retry policy, see [error-handling.md](error-handling.md).
