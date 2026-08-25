# xparse-skills

Agent skills and CLI for document parsing powered by [TextIn xParse API](https://docs.textin.com/api-reference/endpoint/xparse/v1/parse-sync).

Turn PDFs, images, and Office documents into clean Markdown or structured JSON — directly inside your AI coding agent.

## Skills

Install into your agent with one command:

```bash
npx skills add intsig-textin/xparse-skills
```

### xparse-parse

Parse complete documents or navigate targeted sections, pages, facts, and tables via `xparse-cli`.

**Supported formats:** PDF · Images (JPG/PNG/BMP/TIFF/WebP) · Word · PowerPoint · Excel · HTML · OFD · RTF

**Use when:**
- User provides a local file or document URL to read, convert, search, or extract content from
- Task requires turning a document into agent-friendly text before further processing
- Preparing content for summarization, analysis, or downstream workflows

**Quick start:**

```bash
xparse-cli parse report.pdf --api auto                       # Markdown → stdout
xparse-cli parse report.pdf --api auto --view json           # Structured JSON output
xparse-cli parse report.pdf --api auto --output ./result/    # Save to directory
```

> See [SKILL.md](skills/xparse-parse/SKILL.md) for full routing rules, error handling, and references.

## CLI

`xparse-cli` is the underlying binary. It can also be used standalone.

**Install:**

```bash
npm i -g xparse-cli
```

For users in China, use the npmmirror registry:

```bash
npm i -g xparse-cli --registry=https://registry.npmmirror.com
```

**Key commands:**

| Command | Description |
|---------|-------------|
| `xparse-cli parse <file>` | Parse a document to Markdown or JSON |
| `xparse-cli quota --output json` | Show current daily free and authenticated free-package quota |
| `xparse-cli get_doc_info <file>` | Create the stable local document ID for navigation |
| `xparse-cli get_outline <doc_id>` | Navigate the cached document outline |
| `xparse-cli search_text <doc_id> <pattern>` | Search cached document content |
| `xparse-cli auth` | Configure API credentials (interactive) |
| `xparse-cli download --from result.json` | Download images from parse results |
| `xparse-cli version` | Show version info |

**Automatic, free, and paid API selection:**

| Mode | Behavior |
|------|----------|
| `--api auto` | Default; uses current daily free quota, then an AppKey-authenticated `free_package.free_remain_count` when the service reports it |
| `--api free` | Force the free endpoint only |
| `--api paid` | Explicit paid route; requires user approval and valid OAuth/AppKey credentials |

Run `xparse-cli quota --output json` for current authentication, allowance, page,
and file-size limits instead of relying on hardcoded values.

> Full CLI documentation: [cli/README.md](cli/README.md)

## Repository Structure

```
skills/
  xparse-parse/          # Agent skill definition and references
    SKILL.md
    references/
cli/                     # xparse-cli source (Go)
  cmd/
  internal/
  install/               # Install scripts
  build.sh               # Cross-compile script
.github/workflows/
  release.yml            # Auto-release on tag push
```

## License

MIT
