# mymind

An [Obsidian](https://obsidian.md/) plugin that bookmarks links, images, tweets, and PDFs with AI-generated titles and summaries.

Uses the [Gemini API](https://ai.google.dev/) (free tier).

## Commands

Open the command palette (`Ctrl/Cmd + P`) and search for:

- **Bookmark URL** — paste a link (web page, tweet, image URL) and save it with an AI summary
- **Bookmark clipboard image** — analyze an image from your clipboard
- **Bookmark image from file** — pick an image file from disk

Bookmarks are saved as Markdown in your vault's `bookmarks/` folder (configurable in settings).

## Install

### With BRAT (recommended)

1. Install the [BRAT](https://github.com/TfTHacker/obsidian42-brat) plugin
2. Add `gerep/mymind` as a beta plugin

### Manual

1. Download `main.js` and `manifest.json` from the [latest release](https://github.com/gerep/mymind/releases)
2. Create `.obsidian/plugins/mymind/` in your vault
3. Copy both files there
4. Enable the plugin in Settings → Community plugins

## Setup

1. Get a [Gemini API key](https://aistudio.google.com/apikey) (free)
2. Open Settings → mymind
3. Paste your API key

## Settings

| Setting | Default | Description |
|---------|---------|-------------|
| Gemini API key | — | Your Google Gemini API key |
| Model | `gemini-2.0-flash` | Gemini model to use |
| Custom prompt | — | Additional instructions for AI analysis |
| Bookmarks folder | `bookmarks` | Folder in vault for saved bookmarks |

## What it supports

- **Web pages** — fetches and extracts page content
- **Tweets** — extracts tweet text via oEmbed API
- **Image URLs** — downloads and analyzes the image
- **Clipboard images** — screenshots, copied images
- **Image files** — pick from disk via file dialog

## Output format

Each bookmark is saved as a Markdown file:

```markdown

# AI-generated title

Summary of the content.

**Source:** https://example.com/article
```

Images are saved alongside the Markdown file with Obsidian embeds (`![[image.png]]`).

## Development

```bash
npm install
npm run build
make install   # copies to your vault (uses MYMIND_VAULT env var)
make dev       # watch mode
```
