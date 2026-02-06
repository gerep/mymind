package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func writeMarkdown(outDir string, content ContentInput, result AIResult) (string, error) {
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}

	now := time.Now()
	slug := slugify(result.Title)
	if slug == "" {
		slug = "note"
	}
	baseName := fmt.Sprintf("%s-%s", now.Format("20060102-150405"), slug)
	mdFilename := baseName + ".md"
	path := filepath.Join(outDir, mdFilename)

	var imageName string
	if content.Kind == "image" {
		imageName = baseName + content.ImageExt
		imgPath := filepath.Join(outDir, imageName)
		if err := os.WriteFile(imgPath, content.ImageData, 0o644); err != nil {
			return "", fmt.Errorf("writing image file: %w", err)
		}
	}

	var sb strings.Builder

	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "title: %q\n", result.Title)
	fmt.Fprintf(&sb, "created: %q\n", now.Format(time.RFC3339))
	if content.Source != "" {
		fmt.Fprintf(&sb, "source: %q\n", content.Source)
	}
	sb.WriteString("tags:\n")
	for _, tag := range result.Tags {
		fmt.Fprintf(&sb, "  - %s\n", tag)
	}
	sb.WriteString("---\n\n")

	fmt.Fprintf(&sb, "# %s\n\n", result.Title)

	if content.Kind == "image" {
		fmt.Fprintf(&sb, "![[%s]]\n\n", imageName)
	}

	sb.WriteString(result.Summary)
	sb.WriteString("\n")

	if content.Source != "" {
		fmt.Fprintf(&sb, "\n**Source:** %s\n", content.Source)
	}

	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		return "", fmt.Errorf("writing file: %w", err)
	}

	return path, nil
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
