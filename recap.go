package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"google.golang.org/genai"
)

func parsePeriod(s string) (time.Duration, error) {
	if len(s) < 2 {
		return 0, fmt.Errorf("invalid period: %q", s)
	}

	unit := s[len(s)-1]
	numStr := s[:len(s)-1]
	num, err := strconv.Atoi(numStr)
	if err != nil {
		return 0, fmt.Errorf("invalid period number: %q", s)
	}

	switch unit {
	case 'h':
		return time.Duration(num) * time.Hour, nil
	case 'd':
		return time.Duration(num) * 24 * time.Hour, nil
	case 'w':
		return time.Duration(num) * 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unknown period unit %q, use h/d/w", string(unit))
	}
}

func runRecap(outDir, apiKey, model string, args []string) {
	fs := flag.NewFlagSet("recap", flag.ExitOnError)
	period := fs.String("period", "7d", "time period to summarize (e.g., 7d, 2w, 24h)")
	fs.Parse(args)

	dur, err := parsePeriod(*period)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	cutoff := time.Now().Add(-dur)

	notes, err := loadNotes(outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var filtered []Note
	for _, n := range notes {
		if n.Created.After(cutoff) {
			filtered = append(filtered, n)
		}
	}

	if len(filtered) == 0 {
		fmt.Fprintf(os.Stderr, "No notes found in the last %s.\n", *period)
		return
	}

	for i := range filtered {
		if err := loadNoteBody(&filtered[i]); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not load %s: %v\n", filtered[i].Path, err)
		}
	}

	var sb strings.Builder
	for _, n := range filtered {
		fmt.Fprintf(&sb, "## %s\n", n.Title)
		fmt.Fprintf(&sb, "Date: %s\n", n.Created.Format("2006-01-02 15:04"))
		if len(n.Tags) > 0 {
			fmt.Fprintf(&sb, "Tags: %s\n", strings.Join(n.Tags, ", "))
		}
		if n.Summary != "" {
			fmt.Fprintf(&sb, "\n%s\n", n.Summary)
		}
		sb.WriteString("\n---\n\n")

		if sb.Len() > 30000 {
			break
		}
	}

	recapText := sb.String()
	if len(recapText) > 30000 {
		recapText = recapText[:30000]
	}

	fmt.Fprintf(os.Stdout, "Generating recap for %d notes from the last %s...\n", len(filtered), *period)

	recap, err := generateRecap(apiKey, model, recapText, *period)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	now := time.Now()
	slug := slugify(fmt.Sprintf("recap-%s", *period))
	mdPath := uniquePath(filepath.Join(outDir, slug+".md"))

	var out strings.Builder
	out.WriteString("---\n")
	fmt.Fprintf(&out, "title: %q\n", fmt.Sprintf("Recap %s", *period))
	fmt.Fprintf(&out, "created: %q\n", now.Format(time.RFC3339))
	out.WriteString("kind: recap\n")
	out.WriteString("tags:\n")
	out.WriteString("  - recap\n")
	out.WriteString("---\n\n")
	out.WriteString(recap)
	out.WriteString("\n")

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if err := os.WriteFile(mdPath, []byte(out.String()), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stdout, "Saved recap to %s\n", mdPath)
}

func generateRecap(apiKey, model, notesText, period string) (string, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return "", fmt.Errorf("creating Gemini client: %w", err)
	}

	prompt := fmt.Sprintf(
		"You are summarizing personal notes collected over the last %s. Identify themes, key insights, and patterns. Write a concise markdown summary with sections. Here are the notes:\n\n%s",
		period, notesText,
	)

	contents := []*genai.Content{
		genai.NewContentFromParts([]*genai.Part{
			genai.NewPartFromText(prompt),
		}, genai.RoleUser),
	}

	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.3)),
	}

	resp, err := client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return "", fmt.Errorf("generating recap: %w", err)
	}

	return resp.Text(), nil
}
