package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type relatedNote struct {
	Name       string
	Title      string
	SharedTags []string
}

func findRelated(note Note, allNotes []Note) []relatedNote {
	noteFile := filepath.Base(note.Path)
	tagSet := make(map[string]bool)
	for _, t := range note.Tags {
		tagSet[t] = true
	}

	var related []relatedNote
	for _, other := range allNotes {
		if filepath.Base(other.Path) == noteFile {
			continue
		}

		var shared []string
		for _, t := range other.Tags {
			if tagSet[t] {
				shared = append(shared, t)
			}
		}

		if len(shared) > 0 {
			name := strings.TrimSuffix(filepath.Base(other.Path), ".md")
			related = append(related, relatedNote{
				Name:       name,
				Title:      other.Title,
				SharedTags: shared,
			})
		}
	}

	sort.Slice(related, func(i, j int) bool {
		if len(related[i].SharedTags) != len(related[j].SharedTags) {
			return len(related[i].SharedTags) > len(related[j].SharedTags)
		}
		return related[i].Name < related[j].Name
	})

	return related
}

func buildRelatedSection(related []relatedNote) string {
	if len(related) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## Related\n\n")
	for _, r := range related {
		tags := strings.Join(r.SharedTags, ", ")
		fmt.Fprintf(&sb, "- [[%s|%s]] (%s)\n", r.Name, r.Title, tags)
	}
	return sb.String()
}

func updateNoteRelated(notePath string, related []relatedNote) error {
	data, err := os.ReadFile(notePath)
	if err != nil {
		return err
	}

	content := string(data)
	content = stripRelatedSection(content)

	section := buildRelatedSection(related)
	if section != "" {
		content = strings.TrimRight(content, "\n") + "\n" + section
	}

	return os.WriteFile(notePath, []byte(content), 0o644)
}

func stripRelatedSection(content string) string {
	idx := strings.Index(content, "\n## Related\n")
	if idx == -1 {
		return content
	}
	return content[:idx+1]
}

func appendRelatedToNewNote(notePath, vaultDir string) {
	notes, err := loadNotes(vaultDir)
	if err != nil || len(notes) == 0 {
		return
	}

	var current Note
	for _, n := range notes {
		if n.Path == notePath {
			current = n
			break
		}
	}
	if current.Path == "" {
		return
	}

	related := findRelated(current, notes)
	if len(related) > 0 {
		_ = updateNoteRelated(notePath, related)
	}

	for _, n := range notes {
		if n.Path == notePath {
			continue
		}
		rel := findRelated(n, notes)
		_ = updateNoteRelated(n.Path, rel)
	}
}

func runLink(vault string, args []string) {
	notes, err := loadNotes(vault)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(notes) == 0 {
		fmt.Println("no notes found")
		return
	}

	updated := 0
	for _, note := range notes {
		related := findRelated(note, notes)
		oldData, err := os.ReadFile(note.Path)
		if err != nil {
			continue
		}

		oldContent := stripRelatedSection(string(oldData))
		newSection := buildRelatedSection(related)
		newContent := strings.TrimRight(oldContent, "\n") + "\n" + newSection

		if string(oldData) != newContent {
			if err := os.WriteFile(note.Path, []byte(newContent), 0o644); err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not update %s: %v\n", note.Path, err)
				continue
			}
			updated++
		}
	}

	fmt.Printf("Updated %d of %d notes\n", updated, len(notes))
}
