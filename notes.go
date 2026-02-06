package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Note struct {
	Path       string
	Title      string
	Created    time.Time
	Kind       string
	Source     string
	SourceHash string
	Tags       []string
	Summary    string
}

func loadNotes(outDir string) ([]Note, error) {
	var notes []Note
	err := filepath.WalkDir(outDir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".md") {
			return nil
		}
		n, err := parseFrontmatter(p)
		if err != nil {
			return nil
		}
		notes = append(notes, n)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(notes, func(i, j int) bool {
		return notes[i].Created.After(notes[j].Created)
	})

	return notes, nil
}

func parseFrontmatter(path string) (Note, error) {
	f, err := os.Open(path)
	if err != nil {
		return Note{}, err
	}
	defer f.Close()

	n := Note{Path: path}
	scanner := bufio.NewScanner(f)

	if !scanner.Scan() || strings.TrimSpace(scanner.Text()) != "---" {
		return Note{}, os.ErrInvalid
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "---" {
			break
		}

		if strings.HasPrefix(line, "  - ") {
			tag := strings.TrimSpace(strings.TrimPrefix(line, "  - "))
			n.Tags = append(n.Tags, tag)
			continue
		}

		key, val, ok := splitFrontmatterLine(line)
		if !ok {
			continue
		}

		switch key {
		case "title":
			n.Title = val
		case "created":
			t, err := time.Parse(time.RFC3339, val)
			if err == nil {
				n.Created = t
			}
		case "kind":
			n.Kind = val
		case "source":
			n.Source = val
		case "source_hash":
			n.SourceHash = val
		}
	}

	return n, scanner.Err()
}

func splitFrontmatterLine(line string) (key, val string, ok bool) {
	idx := strings.Index(line, ":")
	if idx < 0 {
		return "", "", false
	}
	key = strings.TrimSpace(line[:idx])
	raw := strings.TrimSpace(line[idx+1:])
	if raw == "" {
		return key, "", true
	}
	if unq, err := strconv.Unquote(raw); err == nil {
		return key, unq, true
	}
	return key, raw, true
}

func loadNoteBody(n *Note) error {
	data, err := os.ReadFile(n.Path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	inFront := false
	pastHeading := false
	var para []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if trimmed == "---" && !inFront {
			inFront = true
			continue
		}
		if trimmed == "---" && inFront {
			inFront = false
			continue
		}
		if inFront {
			continue
		}

		if !pastHeading {
			if strings.HasPrefix(trimmed, "# ") {
				pastHeading = true
			}
			continue
		}

		if trimmed == "" {
			if len(para) > 0 {
				break
			}
			continue
		}

		if strings.HasPrefix(trimmed, "![[") {
			continue
		}

		para = append(para, trimmed)
	}

	n.Summary = strings.Join(para, " ")
	return nil
}

func contentHash(content ContentInput) string {
	h := sha256.New()
	switch content.Kind {
	case "link":
		h.Write([]byte(content.Source))
	case "image", "pdf":
		h.Write(content.ImageData)
	default:
		h.Write([]byte(content.Text))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func findDuplicate(notes []Note, hash string) *Note {
	for i := range notes {
		if notes[i].SourceHash == hash {
			return &notes[i]
		}
	}
	return nil
}
