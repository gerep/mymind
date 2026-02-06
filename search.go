package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func runSearch(outDir string, args []string) {
	fs := flag.NewFlagSet("search", flag.ExitOnError)
	maxResults := fs.Int("n", 20, "max results")
	fs.Parse(args)

	query := strings.Join(fs.Args(), " ")
	if query == "" {
		fmt.Fprintln(os.Stderr, "usage: mymind search [-n max] <query>")
		os.Exit(1)
	}

	tokens := strings.Fields(query)
	var tagTokens, textTokens []string
	for _, t := range tokens {
		if strings.HasPrefix(t, "#") {
			tagTokens = append(tagTokens, strings.ToLower(t[1:]))
		} else {
			textTokens = append(textTokens, strings.ToLower(t))
		}
	}

	notes, err := loadNotes(outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	var matches []Note
	for i := range notes {
		if !matchNote(&notes[i], tagTokens, textTokens) {
			continue
		}
		matches = append(matches, notes[i])
		if len(matches) >= *maxResults {
			break
		}
	}

	if len(matches) == 0 {
		fmt.Println("no matching notes found")
		return
	}

	for _, n := range matches {
		date := n.Created.Format("2006-01-02")
		tags := strings.Join(n.Tags, ", ")
		fmt.Fprintf(os.Stdout, "%-12s %-40s [%s]  %s\n", date, n.Title, tags, n.Path)
	}
}

func matchNote(n *Note, tagTokens, textTokens []string) bool {
	for _, tag := range tagTokens {
		if !hasTag(n.Tags, tag) {
			return false
		}
	}

	for _, tok := range textTokens {
		if strings.Contains(strings.ToLower(n.Title), tok) {
			continue
		}
		if n.Summary == "" {
			if err := loadNoteBody(n); err != nil {
				continue
			}
		}
		if !strings.Contains(strings.ToLower(n.Summary), tok) {
			return false
		}
	}

	return true
}

func hasTag(tags []string, target string) bool {
	for _, t := range tags {
		if strings.ToLower(t) == target {
			return true
		}
	}
	return false
}
