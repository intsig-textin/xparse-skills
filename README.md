# xparse-skills

Agent skills and CLI for document parsing powered by [TextIn xParse API](https://docs.textin.com/api-reference/endpoint/xparse/v1/parse-sync).

Turn PDFs, images, and Office documents into clean Markdown or structured JSON — directly inside your AI coding agent.

## Skills

Install into your agent with one command:

```bash
npx skills add intsig-textin/xparse-skills
```

### xparse-parse

Parse documents into Markdown or structured JSON via `xparse-cli`.

**Supported formats:** PDF · Images (JPG/PNG/BMP/TIFF/WebP) · Word · PowerPoint · Excel · HTML · OFD · RTF

**Use when:**
- User provides a local file or document URL to read, convert, or extract content from
- Task requires turning a document into agent-friendly text before further processing
- Preparing content for summarization, analysis, or downstream workflows

**Quick start:**

```bash
xparse-cli parse report.pdf                # Markdown → stdout
xparse-cli parse report.pdf --view json    # Structured JSON output
xparse-cli parse report.pdf --output ./result/   # Save to directory
```

> See [SKILL.md](skills/xparse-parse/SKILL.md) for full routing rules, error handling, and references.

## CLI

`xparse-cli` is the underlying binary. It can also be used standalone.

**Install:**

```bash
# Linux / macOS
source <(curl -fsSL https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.sh)

# Windows (PowerShell)
irm https://dllf.intsig.net/download/2026/Solution/xparse-cli/install.ps1 | iex
```

**Key commands:**

| Command | Description |
|---------|-------------|
| `xparse-cli parse <file>` | Parse a document to Markdown or JSON |
| `xparse-cli auth` | Configure API credentials (interactive) |
| `xparse-cli download --from result.json` | Download images from parse results |
| `xparse-cli update` | Self-update to the latest version |
| `xparse-cli version` | Show version info |

**Free vs paid API:**

| | Free | Paid |
|-|------|------|
| Supported formats | PDF, images | All formats |
| Credentials | Not required | `xparse-cli auth` |
| File size limit | 10 MB | 500 MB |

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
