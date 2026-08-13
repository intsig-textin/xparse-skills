# xparse-skills

Agent skills for document parsing powered by [TextIn xParse API](https://docs.textin.com/api-reference/endpoint/xparse/v1/parse-sync).

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

## CLI ownership

This repository does not contain CLI, Connector, installer, build, or release
sources. Those components are maintained and released only from the GitLab
`xparse-client` repository. Skill documentation may reference installed
`xparse-cli` commands, but CLI implementation changes must not be added here.

## Repository Structure

```
skills/
  xparse-parse/          # Agent skill definition and references
    SKILL.md
    references/
```

## License

MIT
