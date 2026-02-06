package main

import (
	"fmt"
	"net/url"
	"os"
	"strings"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: mymind <link or note>")
		os.Exit(1)
	}

	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "error: GEMINI_API_KEY environment variable is not set")
		os.Exit(1)
	}

	outDir := os.Getenv("MYMIND_OUTPUT_DIR")
	if outDir == "" {
		outDir = "./mymind-notes"
	}

	input := strings.Join(os.Args[1:], " ")

	var content ContentInput
	if isURL(input) {
		if isImageURL(input) {
			fmt.Printf("Downloading image %s...\n", input)
			imgData, err := downloadImage(input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: could not download image: %v\n", err)
				os.Exit(1)
			}
			content = ContentInput{
				Kind:      "image",
				Source:    input,
				ImageData: imgData.Data,
				ImageMIME: imgData.MIMEType,
				ImageExt:  imgData.Extension,
			}
		} else {
			fmt.Printf("Fetching %s...\n", input)
			extracted, err := extractFromURL(input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: could not fetch URL: %v\n", err)
				content = ContentInput{
					Kind:   "link",
					Source: input,
					Text:   input,
				}
			} else {
				content = ContentInput{
					Kind:   "link",
					Source: extracted.URL,
					Text:   fmt.Sprintf("Title: %s\n\n%s", extracted.Title, extracted.Text),
				}
			}
		}
	} else {
		content = ContentInput{
			Kind: "note",
			Text: input,
		}
	}

	fmt.Println("Analyzing with AI...")
	result, err := analyze(apiKey, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: AI analysis failed: %v\n", err)
		os.Exit(1)
	}

	path, err := writeMarkdown(outDir, content, result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not write markdown: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Saved to %s\n", path)
}

func isURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

type ContentInput struct {
	Kind      string // "link", "note", or "image"
	Source    string // original URL, empty for notes
	Text      string // content to analyze (for link/note)
	ImageData []byte // raw image bytes (for image)
	ImageMIME string // e.g. "image/jpeg" (for image)
	ImageExt  string // e.g. ".jpg" (for image)
}
