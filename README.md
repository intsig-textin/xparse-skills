# Agent Skills

A collection of [Agent Skills](https://agentskills.io/) that extend AI coding agents with document parsing capabilities.

## Installation

```bash
npx skills add intsig-textin/xparse-skills
```

## Available Skills

### [xparse-parse](skills/xparse-parse/SKILL.md)

Parse documents into clean markdown or structured JSON via the `xparse-cli`.

**Supported formats:** PDF, images (JPG/PNG/BMP/TIFF), Word (.docx), PowerPoint (.pptx), Excel (.xlsx), HTML, OFD, RTF

**Use when:**

- User provides a local document to read, convert, or extract content from
- Task requires turning a document into agent-friendly text before further processing
- Preparing document content for summarization, analysis, or downstream workflows

**Key features:**

- Zero-config free API for PDF and images — no registration needed
- Paid API for Office/HTML/OFD formats with unlimited quota
- Page range selection and encrypted PDF support
- Markdown and structured JSON output views

**Quick start:**

```bash
xparse-cli parse report.pdf                # Markdown output to stdout
xparse-cli parse report.pdf --view json    # Structured JSON output
```

> For full setup, CLI options, and error handling, see [SKILL.md](skills/xparse-parse/SKILL.md).

## License

MIT
