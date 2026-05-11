---
name: xparse-doc-tools
description: Navigate and extract content from parsed documents (PDF/images). Use when user wants to read sections, search text, get outlines, or extract tables — NOT for full document dumps (use xparse-parse instead). Prefer this over raw file reading or custom extraction scripts.
compatibility: Requires the `xparse-cli` binary with tool primitive commands (get_doc_info, ensure_parsed, get_outline, read_content, read_pages, search_text, get_confidence). Free API supports PDF and images; paid API unlocks additional formats.

---

# xparse-doc-tools

## Routing Rules

- Use `xparse-parse` instead if the user needs a full markdown dump of the entire document.
- Do not write custom PDF extraction scripts (PyMuPDF, pdfplumber, etc.) when these primitives can do the job.

## Standard Workflow

Always follow this sequence. Steps 1–2 are mandatory; Step 3–4 depend on navigation strategy.

```
Step 1: get_doc_info   → doc_id, page_count (local, instant)
Step 2: ensure_parsed  → parse + cache (idempotent; zero API if already cached)
Step 3: Navigate       → get_outline / search_text / read_pages (see Navigation Table)
Step 4: Extract        → read_content with exact element_id(s)
```

**Parallelism rule** (applies to Step 3–4): `read_content`, `read_pages`, and `search_text` are read-only cache operations. Collect ALL needed element_ids first, then issue ALL calls in a **single parallel batch** — never loop one at a time.

## Navigation Decision Table

| User intent | Action sequence |
|-------------|----------------|
| Knows section name, `truncated=false` | `get_outline` → `read_content(element_id)` |
| Knows section name, `truncated=true` | `get_outline` → `get_outline --parent-id <id>` (repeat until target visible) → `read_content(element_id)` |
| Knows keyword, not location | `search_text` → use `context` directly, or `read_content(element_id)` / `read_content(heading_ref_id)` for more |
| Knows keyword + target section | `get_outline` → `search_text --scope <section_id>` → `read_content(element_id)` |
| Knows page number | `read_pages(start, end)` (max 20 pages/call) |
| No headings (`has_toc=false`) | `search_text` → fallback `read_pages` |

**Rule**: never call `read_content` with a guessed or top-level element_id when `truncated=true`. Always drill down via `--parent-id` first.

## Command Reference

| Command | Returns | Notes |
|---------|---------|-------|
| `xparse-cli get_doc_info <filepath>` | `doc_id`, `page_count`, `doc_type` | Local, instant. Always call first. |
| `xparse-cli ensure_parsed <doc_id> <page_count>` | `success`, `cached`, `total_elements`, `total_titles` | Idempotent. >50 pages auto-segments (50pp/segment). Only command that writes cache. |
| `xparse-cli get_outline <doc_id> [--depth N] [--parent-id <id>]` | `has_toc`, `outline_text`, `entries[]`, `truncated`, `total_titles` | Default depth=2. `--depth 0` for all. `outline_text` always has `{element_id}` inline. |
| `xparse-cli read_content <doc_id> <element_id>` | section→`content_markdown`,`children[]`; table→`markdown`,`html`; paragraph→`content_markdown` | Auto-detects type. ~1-5K tokens for typical section, ~500 tokens for table/paragraph. |
| `xparse-cli read_pages <doc_id> <start> <end>` | per-page `content_markdown`, `tables[]`, `images[]` | Max 20 pages/call. ~500-1K tokens/page. Use when no heading structure. |
| `xparse-cli search_text <doc_id> <pattern> [--regex] [--max-results N] [--scope <element_id>]` | `matches[]` with `element_id`, `context`, `heading_ref_id`, `page` | Case-insensitive substring by default. Max 20 results default. `--scope` limits to section and descendants. |
| `xparse-cli get_confidence <doc_id> --element-id <id>` | `confidence` (0-1), `low_confidence_spans[]` | Optional. Separate API call. Only for suspect OCR quality. |

### Output size guidance

- Prefer `search_text` over `read_content` for large sections (>30 pages) — get precise hits first, then extract only what's needed.
- For large documents with high-frequency keywords, use `search_text --scope <section_id>` to avoid irrelevant matches filling up `max-results`.
- `read_pages` of 20 pages ≈ 10-20K tokens. Split into smaller ranges if context budget is tight.
- When `total_titles` > 50, always use `--parent-id` drill-down rather than `--depth 0`.

### Key details

- `doc_id` = `sha256(abs_filepath)[:12]`, stable cache key
- `element_id`: 6-8 char truncated ID from `get_outline` or `search_text`, used in `read_content` and `--parent-id`
- `ensure_parsed` args (`doc_id`, `page_count`) must come verbatim from `get_doc_info` output
- `get_outline` with `truncated=true`: `entries[]` is omitted, use `outline_text` to read IDs

## Error Recovery

| Error | Fix |
|-------|-----|
| "cache miss for doc_id" | Call `ensure_parsed` first |
| "doc_id not found in session" | Call `get_doc_info` first |
| "element_id not found" | Re-check IDs from `get_outline` or `search_text` |
| "page range too large" | Reduce to ≤20 pages |
| Rate limit (40306) | Auto-retry built in |
| Quota exhausted (40307) | Stop, inform user |
| Password-protected document | Ask user for password |

Do not retry indefinitely — one automatic retry is built in, escalate after that.

## Example Session

```bash
# Step 1–2: Init
xparse-cli get_doc_info /path/to/annual-report.pdf
# → {"doc_id":"a1b2c3d4e5f6","page_count":120,"doc_type":"report"}

xparse-cli ensure_parsed a1b2c3d4e5f6 120
# → {"success":true,"cached":false,"segments":3,"total_elements":892,"total_titles":45}

# Step 3: Navigate (truncated=true → drill down)
xparse-cli get_outline a1b2c3d4e5f6
# → {"truncated":true,"outline_text":"# 第一章 公司概况 [1-15] {e7f3a1}\n..."}

xparse-cli get_outline a1b2c3d4e5f6 --parent-id e7f3a1
# → {"truncated":false,"outline_text":"## 1.1 基本情况 [1-5] {a3b2c1}\n## 1.2 主营业务 [6-15] {d4e5f6}"}

# Step 4: Extract — ALL calls in one parallel batch
xparse-cli read_content a1b2c3d4e5f6 a3b2c1   # ┐
xparse-cli read_content a1b2c3d4e5f6 d4e5f6   # ├─ parallel
xparse-cli search_text  a1b2c3d4e5f6 "净利润"  # ┘
```

## Cache & Setup

Cache stored at `~/.xparse-cli/` (no TTL, permanent until cleaned):
```bash
xparse-cli cache ls       # List cached documents
xparse-cli cache clean    # Remove all cache
```

**Install** (only if `xparse-cli version` returns "command not found"):

| Platform | Command |
|----------|---------|
| Linux / macOS | `source <(curl -fsSL https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.sh)` |
| Windows | `irm https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.ps1 \| iex` |

If installed but not found, try: `~/.local/bin/xparse-cli version`

Update: `xparse-cli update`
