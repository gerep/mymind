package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func renderMarkdown(content ContentInput, result AIResult, hash string) string {
	slug := slugify(result.Title)
	if slug == "" {
		slug = "note"
	}

	var imageName string
	if content.Kind == "image" {
		imageName = slug + content.ImageExt
	}

	return buildMarkdown(content, result, hash, time.Now(), imageName)
}

func writeMarkdown(outDir string, content ContentInput, result AIResult, hash string) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	slug := slugify(result.Title)
	if slug == "" {
		slug = "note"
	}
	mdPath := uniquePath(filepath.Join(outDir, slug+".md"))
	baseName := strings.TrimSuffix(filepath.Base(mdPath), ".md")

	var imageName string
	if content.Kind == "image" {
		imageName = baseName + content.ImageExt
		imgPath := filepath.Join(outDir, imageName)
		if err := os.WriteFile(imgPath, content.ImageData, 0o644); err != nil {
			return "", fmt.Errorf("writing image file: %w", err)
		}
	}

	md := buildMarkdown(content, result, hash, time.Now(), imageName)

	if err := os.WriteFile(mdPath, []byte(md), 0o644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return mdPath, nil
}

func buildMarkdown(content ContentInput, result AIResult, hash string, now time.Time, imageName string) string {
	var sb strings.Builder

	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "title: %q\n", result.Title)
	fmt.Fprintf(&sb, "created: %q\n", now.Format(time.RFC3339))
	fmt.Fprintf(&sb, "kind: %s\n", content.Kind)
	if content.Source != "" {
		fmt.Fprintf(&sb, "source: %q\n", content.Source)
	}
	if hash != "" {
		fmt.Fprintf(&sb, "source_hash: %q\n", hash)
	}
	sb.WriteString("tags:\n")
	for _, tag := range result.Tags {
		fmt.Fprintf(&sb, "  - %s\n", tag)
	}
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# %s\n\n", result.Title)

	if content.Kind == "image" && imageName != "" {
		fmt.Fprintf(&sb, "![[%s]]\n\n", imageName)
	}

	sb.WriteString(result.Summary)
	sb.WriteString("\n")

	if content.Source != "" && content.Source != "clipboard" {
		fmt.Fprintf(&sb, "\n**Source:** %s\n", content.Source)
	}

	if content.Kind == "link" && content.Text != "" {
		sb.WriteString("\n---\n\n")
		sb.WriteString("<details>\n<summary>Original content</summary>\n\n")
		sb.WriteString(content.Text)
		sb.WriteString("\n\n</details>\n")
	}

	return sb.String()
}

func uniquePath(p string) string {
	if _, err := os.Stat(p); os.IsNotExist(err) {
		return p
	}
	ext := filepath.Ext(p)
	base := strings.TrimSuffix(p, ext)
	for i := 2; ; i++ {
		candidate := fmt.Sprintf("%s-%d%s", base, i, ext)
		if _, err := os.Stat(candidate); os.IsNotExist(err) {
			return candidate
		}
	}
}

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

func slugify(s string) string {
	s = strings.ToLower(s)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 50 {
		s = s[:50]
		s = strings.TrimRight(s, "-")
	}
	return s
}
