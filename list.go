package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func runList(outDir string, args []string) {
	fs := flag.NewFlagSet("list", flag.ExitOnError)
	n := fs.Int("n", 10, "number of notes to show")
	all := fs.Bool("all", false, "show all notes")
	fs.Parse(args)

	notes, err := loadNotes(outDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	if len(notes) == 0 {
		fmt.Fprintf(os.Stdout, "no notes found in %s\n", outDir)
		return
	}

	total := len(notes)
	if !*all && *n < len(notes) {
		notes = notes[:*n]
	}

	for _, note := range notes {
		tags := ""
		if len(note.Tags) > 0 {
			tags = fmt.Sprintf("  [%s]", strings.Join(note.Tags, ", "))
		}
		fmt.Fprintf(os.Stdout, "%s  [%s]  %s%s\n",
			note.Created.Format("2006-01-02"),
			note.Kind,
			note.Title,
			tags,
		)
	}

	fmt.Fprintf(os.Stdout, "\nShowing %d of %d notes\n", len(notes), total)
}
