package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func runScan(vault, apiKey, model, customPrompt string, args []string) {
	fs := flag.NewFlagSet("scan", flag.ExitOnError)
	force := fs.Bool("force", false, "re-process notes that already have tags")
	fs.Parse(args)

	dir := vault

	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(files) == 0 {
		fmt.Println("no markdown files found")
		return
	}

	fmt.Printf("Found %d markdown files in %s\n", len(files), dir)

	processed, skipped, errors := 0, 0, 0
	for _, path := range files {
		note, err := parseFrontmatter(path)
		hasFrontmatter := err == nil

		if hasFrontmatter && len(note.Tags) > 0 && !*force {
			skipped++
			continue
		}

		body, err := readNoteContent(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not read %s: %v\n", path, err)
			errors++
			continue
		}

		if strings.TrimSpace(body) == "" {
			skipped++
			continue
		}

		fmt.Printf("Processing %s...\n", filepath.Base(path))

		content := ContentInput{Kind: "note", Text: body}
		result, err := analyze(apiKey, model, customPrompt, content)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: AI analysis failed for %s: %v\n", filepath.Base(path), err)
			errors++
			continue
		}

		hash := contentHash(content)

		if hasFrontmatter {
			err = updateExistingNote(path, note, result, hash)
		} else {
			err = addFrontmatterToNote(path, body, result, hash)
		}

		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not update %s: %v\n", filepath.Base(path), err)
			errors++
			continue
		}

		processed++
	}

	fmt.Printf("\nDone: %d processed, %d skipped, %d errors (of %d files)\n", processed, skipped, errors, len(files))

	if processed > 0 {
		fmt.Println("Updating related links...")
		runLink(dir, nil)
	}
}

func readNoteContent(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	content := string(data)

	if strings.HasPrefix(strings.TrimSpace(content), "---") {
		lines := strings.SplitN(content, "\n", -1)
		inFront := false
		var bodyLines []string
		for _, line := range lines {
			if strings.TrimSpace(line) == "---" {
				if !inFront {
					inFront = true
					continue
				}
				inFront = false
				continue
			}
			if !inFront {
				bodyLines = append(bodyLines, line)
			}
		}
		return strings.TrimSpace(strings.Join(bodyLines, "\n")), nil
	}

	return strings.TrimSpace(content), nil
}

func updateExistingNote(path string, note Note, result AIResult, hash string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	content := string(data)
	body := extractBodyAfterFrontmatter(content)

	now := note.Created
	if now.IsZero() {
		now = time.Now()
	}

	title := result.Title
	if note.Title != "" && title == "untitled" {
		title = note.Title
	}

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "title: %q\n", title)
	fmt.Fprintf(&sb, "created: %q\n", now.Format(time.RFC3339))
	fmt.Fprintf(&sb, "kind: %s\n", note.Kind)
	if note.Source != "" {
		fmt.Fprintf(&sb, "source: %q\n", note.Source)
	}
	if hash != "" {
		fmt.Fprintf(&sb, "source_hash: %q\n", hash)
	}
	sb.WriteString("tags:\n")
	for _, tag := range result.Tags {
		fmt.Fprintf(&sb, "  - %s\n", tag)
	}
	sb.WriteString("---\n")
	sb.WriteString(body)

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func addFrontmatterToNote(path, body string, result AIResult, hash string) error {
	now := time.Now()

	var sb strings.Builder
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "title: %q\n", result.Title)
	fmt.Fprintf(&sb, "created: %q\n", now.Format(time.RFC3339))
	sb.WriteString("kind: note\n")
	if hash != "" {
		fmt.Fprintf(&sb, "source_hash: %q\n", hash)
	}
	sb.WriteString("tags:\n")
	for _, tag := range result.Tags {
		fmt.Fprintf(&sb, "  - %s\n", tag)
	}
	sb.WriteString("---\n\n")
	sb.WriteString(body)
	sb.WriteString("\n")

	return os.WriteFile(path, []byte(sb.String()), 0o644)
}

func extractBodyAfterFrontmatter(content string) string {
	lines := strings.SplitN(content, "\n", -1)
	inFront := false
	var idx int
	for i, line := range lines {
		if strings.TrimSpace(line) == "---" {
			if !inFront {
				inFront = true
				continue
			}
			idx = i + 1
			break
		}
	}
	if idx >= len(lines) {
		return "\n"
	}
	return "\n" + strings.Join(lines[idx:], "\n")
}
