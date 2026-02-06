# my mind

> 100% vibe coded using [Amp](https://ampcode.com/)

A CLI that uses AI to analyze text, web pages, images, PDFs, and tweets — generating relevant tags and summaries.
Output is saved as Markdown with YAML frontmatter, ready for [Obsidian](https://obsidian.md/).

Uses the [Gemini API](https://ai.google.dev/) (free tier).

## Setup

```bash
export GEMINI_API_KEY="your-api-key"
export MYMIND_VAULT="/path/to/your/obsidian/vault"
go build -o mymind .
```

Or add both to a `.env` file in your working directory:

```
GEMINI_API_KEY=your-api-key
MYMIND_VAULT=/path/to/your/obsidian/vault
```

## Usage

```bash
# Analyze a web page
mymind https://example.com/article

# Analyze a tweet
mymind https://x.com/user/status/123456

# Analyze an image URL
mymind https://example.com/photo.jpg

# Analyze a PDF
mymind paper.pdf

# Save a text note
mymind "An interesting idea about distributed systems"

# Analyze a screenshot from clipboard
mymind clipboard

# Read a note from stdin
echo "Some thought" | mymind -

# Preview without saving
mymind --dry-run "test note"

# Custom AI instructions
mymind --prompt "focus on actionable takeaways" https://example.com/article
```

## Subcommands

```bash
# Search notes (use #tag for tag search)
mymind search golang
mymind search #machine-learning
mymind search -n 5 "distributed systems"

# List recent notes
mymind list
mymind list -n 20
mymind list --all

# Open vault in file manager
mymind open

# Process multiple inputs from a file
mymind batch urls.txt

# Summarize recent notes
mymind recap
mymind recap --period 2w
mymind recap --period 24h

# Update related links between all notes
mymind link

# Generate tags for existing notes in vault
mymind scan
mymind scan -force
```

## Flags

| Flag | Description |
|------|-------------|
| `--vault <path>` | Path to notes vault (overrides `MYMIND_VAULT`) |
| `--folder <name>` | Subfolder for new notes (overrides `MYMIND_FOLDER`, default: `inbox`) |
| `--model <name>` | Gemini model (overrides `MYMIND_MODEL`) |
| `--prompt <text>` | Custom AI instructions (overrides `MYMIND_PROMPT`) |
| `--dry-run` | Print markdown to stdout, don't write files |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GEMINI_API_KEY` | Yes | — | Google Gemini API key |
| `MYMIND_VAULT` | Yes | — | Path to your notes vault |
| `MYMIND_FOLDER` | No | `inbox` | Subfolder for new notes |
| `MYMIND_MODEL` | No | `gemini-2.0-flash` | Gemini model to use |
| `MYMIND_PROMPT` | No | — | Custom AI prompt instructions |

Variables can also be set in a `.env` file in the working directory.

## Batch File Format

One input per line. Empty lines and lines starting with `#` are ignored:

```
# Articles to read
https://example.com/article-1
https://example.com/article-2

# Images
https://example.com/photo.jpg
```

## Features

- **Vault-based** — all notes live in your Obsidian vault, not the project directory
- **Duplicate detection** — skips inputs already saved (based on content hash)
- **Twitter/X support** — extracts tweet text via oEmbed API
- **PDF analysis** — sends PDFs to Gemini for multimodal analysis
- **Clipboard support** — analyze screenshots directly (Wayland/X11)
- **Obsidian-ready** — YAML frontmatter with tags, embedded images via `![[image]]`
- **Collapsible original content** — web page text stored in `<details>` for searchability
- **Related notes** — auto-links notes sharing tags via `[[wikilinks]]` for Obsidian's Graph View; `mymind link` updates all notes retroactively
- **Scan existing notes** — `mymind scan` generates tags for notes you already have

## Clipboard Support

Requires `wl-paste` (Wayland) or `xclip` (X11).
