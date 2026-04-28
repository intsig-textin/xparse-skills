---
name: xparse-doc-tools
description: Navigate and extract content from parsed documents using tool primitives. Use this skill when the user wants to read specific sections, search text, get outlines, or extract tables from a PDF/document that has been (or needs to be) parsed by xparse. This skill orchestrates a 4-step workflow (get_doc_info → ensure_parsed → get_outline → read_content) to efficiently access document content with minimal API calls. Prefer this over reading raw files or writing custom extraction scripts.
compatibility: Requires the `xparse-cli` binary with tool primitive commands (get_doc_info, ensure_parsed, get_outline, read_content, read_pages, search_text, get_confidence). Free API supports PDF and images; paid API unlocks additional formats.

---

# xparse-doc-tools

## Overview

Use the xparse-cli document tool primitives to navigate and extract content from documents. The workflow minimizes API calls by caching parsed results locally.

## Routing Rules

- Use `xparse-doc-tools` when the user wants to read, search, or extract structured content from a document (sections, tables, pages, outlines).
- Use `xparse-parse` instead if the user just needs a full markdown dump of the entire document.
- Do not write custom PDF extraction scripts (PyMuPDF, pdfplumber, etc.) when these primitives can do the job.
- If `ensure_parsed` has already been called for a document, all subsequent reads are zero API calls from cache.

## Setup

Check if installed: `xparse-cli version`

If `command not found` after install, try the absolute path: `~/.local/bin/xparse-cli version`

Update to latest version: `xparse-cli update`

If not found, install:

| Platform | Command |
|----------|---------|
| Linux / macOS | ` source <(curl -fsSL https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.sh) ` |
| Windows (PowerShell) | `irm https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.ps1 \| iex` |

Verify tool primitives are available:
```bash
xparse-cli get_doc_info --help
```

## Standard Workflow (4 Steps)

Always follow this sequence for any document task:

```
Step 1: get_doc_info    → Get doc_id, page_count (local, zero API)
Step 2: ensure_parsed   → Full parse + cache (API call, once per document)
Step 3: get_outline     → Get table of contents (from cache, zero API)
Step 4: read_content    → Read specific sections/elements (from cache, zero API)
```

## Quick Reference

| Goal | Command |
|------|---------|
| Get doc metadata | `xparse-cli get_doc_info <filepath>` |
| Parse and cache document | `xparse-cli ensure_parsed <doc_id> <page_count>` |
| Get document outline | `xparse-cli get_outline <doc_id>` |
| Read section/element content | `xparse-cli read_content <doc_id> <element_id>` |
| Read by page range | `xparse-cli read_pages <doc_id> <start_page> <end_page>` |
| Search text in document | `xparse-cli search_text <doc_id> <pattern>` |
| Search with regex | `xparse-cli search_text <doc_id> <pattern> --regex` |
| Check OCR confidence | `xparse-cli get_confidence <doc_id> --element-id <id>` |
| List cached documents | `xparse-cli cache ls` |
| Clean all cache | `xparse-cli cache clean` |

## Tool Primitives

### 1. `get_doc_info` — Document Metadata (Local, Zero API)

```bash
xparse-cli get_doc_info <filepath>
```

Returns: `doc_id`, `filepath`, `filename`, `page_count`, `doc_type`

- Pure local operation using PDF library, millisecond response
- `doc_id` = `sha256(abs_filepath)[:12]`, used as cache key for all subsequent calls
- `doc_type`: contract / report / manual / invoice / presentation / other
- Always call this first to obtain `doc_id` and `page_count`

### 2. `ensure_parsed` — Full Parse + Cache (API Call)

```bash
xparse-cli ensure_parsed <doc_id> <page_count>
```

Returns: `success`, `cached`, `segments`, `total_elements`, `total_titles`

- If already cached, returns immediately (zero API calls)
- Documents <= 50 pages: single API call
- Documents > 50 pages: automatic segmentation (50 pages/segment, serial calls, merged result)
- After this call, all subsequent tools read from cache with zero API cost
- This is the **only** command that writes to the parse cache

### 3. `get_outline` — Document Outline (From Cache)

```bash
xparse-cli get_outline <doc_id>
```

Returns: `doc_id`, `page_count`, `has_toc`, `outline_text`, `entries[]`

- Each entry has: `element_id` (6-8 char truncated ID), `heading`, `heading_path`, `level`, `page_start`, `page_end`, `parent_id`
- `outline_text` is a readable Markdown-formatted TOC with page ranges and element_ids
- `has_toc` = true if title_tree has > 2 nodes
- Use `element_id` from entries directly for `read_content`

### 4. `read_content` — Read by Element ID (From Cache)

```bash
xparse-cli read_content <doc_id> <element_id>
```

`element_id` is a 6-8 char truncated ID, obtained from `get_outline` or `search_text` results.

Auto-detects element type and adapts output format:

| Type | Input source | Output |
|------|-------------|--------|
| **section** | heading `element_id` from `get_outline` | `content_markdown`, `content_length`, `children[]` |
| **table** | table element from `search_text` | `caption`, `headers`, `rows`, `markdown`, `html` |
| **paragraph** | paragraph element from `search_text` | `content_markdown` |

### 5. `read_pages` — Read by Page Range (From Cache)

```bash
xparse-cli read_pages <doc_id> <start_page> <end_page>
```

- Maximum 20 pages per call
- Returns per-page: `content_markdown`, `tables[]`, `images[]`
- Use when document has no clear heading structure, or you need raw page-by-page content
- Skips headers/footers automatically

### 6. `search_text` — Full-text Search (From Cache)

```bash
xparse-cli search_text <doc_id> <pattern> [--regex] [--max-results N]
```

Returns: `total_matches`, `matches[]` with `match_text`, `element_id` (6-8 chars), `element_type`, `page`, `context`, `heading_ref_id` (6-8 chars), `heading`

- Default: case-insensitive substring match
- `--regex`: regex mode
- `--max-results`: default 20
- All returned IDs (`element_id`, `heading_ref_id`) are truncated 6-8 char IDs, directly usable with `read_content`

After search, use results via:
1. `context` field directly (if answer is visible)
2. `read_content <doc_id> <element_id>` for full element
3. `read_content <doc_id> <heading_ref_id>` for containing section

### 7. `get_confidence` — OCR Confidence (Separate API Call)

```bash
xparse-cli get_confidence <doc_id> --element-id <id> [--text "fragment"]
xparse-cli get_confidence <doc_id> --page <N> [--text "fragment"]
```

- Requires a separate API call with character-level details (not in standard cache)
- Use only when OCR quality is suspect (scanned documents, blurry images)
- Returns: `confidence` (0-1), `low_confidence_spans[]`, optional `text_confidence`

## Navigation Strategy

```
has_toc=true:
  get_outline → read_content(heading_short_id)   # Standard path

has_toc=false:
  get_outline → read_pages → read_content        # Fallback to page reading

Any path:
  search_text is always available for keyword lookup
```

## Cache Management

```bash
xparse-cli cache ls          # List all cached documents (doc_id, filepath, parsed status)
xparse-cli cache clean       # Remove all cached data (~/.xparse-cli/)
```

Cache is stored at `~/.xparse-cli/`:
- `~/.xparse-cli/docinfo/{doc_id}.json` — document metadata
- `~/.xparse-cli/cache/{doc_id}.json` — full parse results

Cache has no TTL. Documents are cached permanently until manually cleaned.

## Error Handling

| Error | Action |
|-------|--------|
| "cache miss for doc_id" | Call `ensure_parsed` first |
| "doc_id not found in session" | Call `get_doc_info` first |
| "element_id not found" | Check available IDs from `get_outline` or `search_text` |
| "page range too large" | Use max 20 pages per `read_pages` call |
| API rate limit (40306) | Wait and retry automatically (built-in) |
| Free quota exhausted (40307) | Stop, inform user |

## When to Stop

- If `ensure_parsed` fails due to quota/network after retries, inform the user
- If the document requires a password, ask the user
- Do not retry indefinitely — one automatic retry is built in, escalate after that

## Example Session

```bash
# Step 1: Get document info
xparse-cli get_doc_info /path/to/annual-report.pdf
# → {"doc_id":"a1b2c3d4e5f6","page_count":120,"doc_type":"report",...}

# Step 2: Parse and cache (automatic segmentation for 120 pages)
xparse-cli ensure_parsed a1b2c3d4e5f6 120
# → {"success":true,"cached":false,"segments":3,"total_elements":892,"total_titles":45}

# Step 3: Get outline
xparse-cli get_outline a1b2c3d4e5f6
# → {"has_toc":true,"outline_text":"# 第一章 公司概况 [1-15] {e7f3a1}\n...",...}

# Step 4: Read a specific section
xparse-cli read_content a1b2c3d4e5f6 e7f3a1
# → {"element_type":"section","content_markdown":"...","children":[...]}

# Search for specific content
xparse-cli search_text a1b2c3d4e5f6 "净利润"
# → {"total_matches":5,"matches":[{"match_text":"净利润","context":"...","heading":"主要财务数据",...}]}
```
