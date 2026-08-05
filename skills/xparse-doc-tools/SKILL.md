---
name: xparse-doc-tools
description: Navigate and extract content from parsed documents (PDF/images). Use when user wants to read sections, search text, get outlines, or extract tables — NOT for full document dumps (use xparse-parse instead). Prefer this over raw file reading or custom extraction scripts.
compatibility: Requires the `xparse-cli` binary with tool primitive commands (get_doc_info, ensure_parsed, get_outline, read_content, read_pages, search_text, get_confidence). Free API supports PDF and images; paid API unlocks additional formats.

---

# xparse-doc-tools

## WorkBuddy command profile

When this Skill is installed by the TextIn xParse WorkBuddy Connector, prefix
every command in this document with `xparse-cli --profile workbuddy` instead of
plain `xparse-cli`. Do not rely on Connector environment variables being
inherited by WorkBuddy task shells. Outside WorkBuddy, use the commands as
written.

For each new WorkBuddy user request, create a private `0600` JSON file with
schema `xparse_task_context.v1`, the user's original-language `user_intent`, and
a brief operational `tool_call_reason`. Pass `--task-context <FILE>` only on
the first xParse command for that request, then delete the temporary file.
Later commands inherit the active task from `CODEBUDDY_SESSION_ID`; the next
user request must pass a newly created context even if the session is reused.
Never use inline JSON, shell `echo`, or a heredoc for this context, and never
include hidden reasoning, credentials, document content, or the final answer.

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

## ⚠️ Critical: Plan-then-Batch Execution

**This is the #1 rule for efficiency. Violations waste 50-70% of tokens.**

### Plan Phase (after get_outline)

Before ANY `read_content` call, complete ALL navigation:
1. List ALL section IDs needed for the user's question
2. If `truncated=true`, do ALL `--parent-id` drill-downs to collect IDs
3. Use `search_text` to locate specific data points

### Execute Phase (single batch)

Issue ALL `read_content` / `search_text` calls in **ONE tool-call message**.

```
✅ CORRECT: One message with 8 parallel read_content calls → 1 turn
❌ WRONG:   8 separate messages each with 1 read_content  → 8 turns
```

**Why this matters**: Each additional turn re-reads the entire context. 10 serial calls on a 30K-token context = 300K input tokens wasted. One parallel batch = 30K total.

### Budget Limits

- Target: **≤8 `read_content` calls** per task
- Prefer reading a **parent section** (includes children) over reading each child separately
- Use `search_text` for fact-finding (numbers, names, dates) — the `context` field often has the answer without needing `read_content`

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

## Prefer `search_text` Over `read_content` for Facts

When extracting specific data points (revenue, profit, percentages, names):
- Call `search_text` first — the `context` field (±surrounding text) often contains the answer
- Only escalate to `read_content` if you need full table structure or surrounding paragraph context
- For large sections (>30 pages), ALWAYS use `search_text --scope` before `read_content`

## Command Reference

| Command | Returns | Notes |
|---------|---------|-------|
| `xparse-cli get_doc_info <filepath>` | `doc_id`, `page_count`, `doc_type` | Local, instant. Always call first. |
| `xparse-cli ensure_parsed <doc_id> <page_count>` | `success`, `cached`, `total_elements`, `total_titles` | Idempotent. >50 pages auto-segments (50pp/segment). Only command that writes cache. |
| `xparse-cli get_outline <doc_id> [--depth N] [--parent-id <id>]` | `has_toc`, `outline_text`, `entries[]`, `truncated`, `total_titles` | Default depth=2. If `ensure_parsed` returned `total_titles`>30, use `--depth 1`. `--depth 0` for all. `outline_text` always has `{element_id}` inline. |
| `xparse-cli read_content <doc_id> <element_id>` | section→`content_markdown`,`children[]`; table→`markdown`,`html`; paragraph→`content_markdown` | Auto-detects type. ~1-5K tokens for typical section, ~500 tokens for table/paragraph. |
| `xparse-cli read_pages <doc_id> <start> <end>` | per-page `content_markdown`, `tables[]`, `images[]` | Max 20 pages/call. ~500-1K tokens/page. Use when no heading structure. |
| `xparse-cli search_text <doc_id> <pattern> [--regex] [--max-results N] [--scope <element_id>]` | `matches[]` with `element_id`, `context`, `heading_ref_id`, `page` | Case-insensitive substring by default. Max 20 results default. `--scope` limits to section and descendants. |
| `xparse-cli get_confidence <doc_id> --element-id <id>` | `confidence` (0-1), `low_confidence_spans[]` | Optional. Separate API call. Only for suspect OCR quality. |

### Output size guidance

- Prefer `search_text` over `read_content` for large sections (>30 pages) — get precise hits first, then extract only what's needed.
- For large documents with high-frequency keywords, use `search_text --scope <section_id>` to avoid irrelevant matches filling up `max-results`.
- `read_pages` of 20 pages ≈ 10-20K tokens. Split into smaller ranges if context budget is tight.
- When `ensure_parsed` returns `total_titles` > 30, use `get_outline --depth 1` initially, then `--parent-id` drill-down only for relevant branches.

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

# Step 3: Navigate — total_titles=45 (from ensure_parsed) > 30, so use depth 1
xparse-cli get_outline a1b2c3d4e5f6 --depth 1
# → {"truncated":true,"outline_text":"# 第一章 公司概况 [1-15] {e7f3a1}\n# 第二章 财务 [16-80] {b2c3d4}\n..."}

xparse-cli get_outline a1b2c3d4e5f6 --parent-id e7f3a1
# → {"truncated":false,"outline_text":"## 1.1 基本情况 [1-5] {a3b2c1}\n## 1.2 主营业务 [6-15] {d4e5f6}"}

xparse-cli get_outline a1b2c3d4e5f6 --parent-id b2c3d4
# → {"truncated":false,"outline_text":"## 2.1 收入分析 [16-30] {f1a2b3}\n## 2.2 利润分析 [31-50] {c4d5e6}"}

# Step 4: Execute — ALL calls in ONE parallel batch (not one per turn!)
xparse-cli read_content a1b2c3d4e5f6 a3b2c1   # ┐
xparse-cli read_content a1b2c3d4e5f6 d4e5f6   # │
xparse-cli read_content a1b2c3d4e5f6 f1a2b3   # ├─ ALL in one tool-call message
xparse-cli read_content a1b2c3d4e5f6 c4d5e6   # │
xparse-cli search_text  a1b2c3d4e5f6 "净利润"  # ┘
```

**Anti-pattern (DO NOT do this):**
```bash
# ❌ WRONG: Each call in a separate turn
# Turn 1:
xparse-cli read_content a1b2c3d4e5f6 a3b2c1
# Turn 2 (after reading result):
xparse-cli read_content a1b2c3d4e5f6 d4e5f6
# Turn 3 (after reading result):
xparse-cli read_content a1b2c3d4e5f6 f1a2b3
# ... This wastes 3x the input tokens!
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
