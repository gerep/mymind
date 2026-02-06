package main

import (
	"bufio"
	"flag"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

type ContentInput struct {
	Kind      string // "link", "note", "image", or "pdf"
	Source    string // original URL or file path
	Text      string // content to analyze (for link/note)
	ImageData []byte // raw bytes (for image/pdf)
	ImageMIME string // e.g. "image/jpeg" or "application/pdf"
	ImageExt  string // e.g. ".jpg" (for image)
}

func main() {
	_ = godotenv.Load()

	vault := flag.String("vault", "", "path to notes vault (or MYMIND_VAULT)")
	folder := flag.String("folder", "", "subfolder within vault for new notes (default: inbox or MYMIND_FOLDER)")
	model := flag.String("model", "", "Gemini model (default: gemini-2.0-flash or MYMIND_MODEL)")
	dryRun := flag.Bool("dry-run", false, "print markdown to stdout instead of writing a file")
	prompt := flag.String("prompt", "", "custom instructions for AI analysis (or MYMIND_PROMPT)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage:\n")
		fmt.Fprintf(os.Stderr, "  mymind <link>              Analyze a web page, tweet, or image URL\n")
		fmt.Fprintf(os.Stderr, "  mymind <note>              Analyze a text note\n")
		fmt.Fprintf(os.Stderr, "  mymind <file.pdf>          Analyze a PDF file\n")
		fmt.Fprintf(os.Stderr, "  mymind clipboard           Analyze image from clipboard\n")
		fmt.Fprintf(os.Stderr, "  echo \"text\" | mymind -     Read note from stdin\n")
		fmt.Fprintf(os.Stderr, "  mymind search <query>      Search saved notes (#tag for tag search)\n")
		fmt.Fprintf(os.Stderr, "  mymind list [-n N] [--all] List recent notes\n")
		fmt.Fprintf(os.Stderr, "  mymind open                Open vault in file manager\n")
		fmt.Fprintf(os.Stderr, "  mymind batch <file>        Process inputs from a file\n")
		fmt.Fprintf(os.Stderr, "  mymind recap [--period 7d] Summarize recent notes\n")
		fmt.Fprintf(os.Stderr, "  mymind link                Update related links between notes\n")
		fmt.Fprintf(os.Stderr, "  mymind scan [-force]       Generate tags for existing notes in vault\n")
		fmt.Fprintf(os.Stderr, "\nFlags:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nEnvironment:\n")
		fmt.Fprintf(os.Stderr, "  GEMINI_API_KEY       API key for Google Gemini (required)\n")
		fmt.Fprintf(os.Stderr, "  MYMIND_VAULT         Path to notes vault (required)\n")
		fmt.Fprintf(os.Stderr, "  MYMIND_FOLDER        Subfolder for new notes (default: inbox)\n")
		fmt.Fprintf(os.Stderr, "  MYMIND_MODEL         Gemini model (default: gemini-2.0-flash)\n")
		fmt.Fprintf(os.Stderr, "  MYMIND_PROMPT        Custom AI prompt instructions\n")
	}
	flag.Parse()

	if flag.NArg() < 1 {
		flag.Usage()
		os.Exit(1)
	}

	if *vault == "" {
		*vault = os.Getenv("MYMIND_VAULT")
	}
	if *vault == "" {
		fmt.Fprintln(os.Stderr, "error: vault path is required. Set MYMIND_VAULT or use --vault")
		os.Exit(1)
	}

	if *folder == "" {
		*folder = os.Getenv("MYMIND_FOLDER")
	}
	if *folder == "" {
		*folder = "inbox"
	}

	noteDir := filepath.Join(*vault, *folder)

	if *model == "" {
		*model = os.Getenv("MYMIND_MODEL")
	}
	if *model == "" {
		*model = "gemini-2.0-flash"
	}

	if *prompt == "" {
		*prompt = os.Getenv("MYMIND_PROMPT")
	}

	args := flag.Args()
	switch args[0] {
	case "search":
		runSearch(*vault, args[1:])
		return
	case "list":
		runList(*vault, args[1:])
		return
	case "open":
		runOpen(*vault)
		return
	case "batch":
		apiKey := requireAPIKey()
		runBatch(noteDir, apiKey, *model, *prompt, args[1:])
		return
	case "recap":
		apiKey := requireAPIKey()
		runRecap(*vault, apiKey, *model, args[1:])
		return
	case "link":
		runLink(*vault, args[1:])
		return
	case "scan":
		apiKey := requireAPIKey()
		runScan(*vault, apiKey, *model, *prompt, args[1:])
		return
	}

	apiKey := requireAPIKey()
	input := strings.Join(args, " ")

	content, err := resolveInput(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	hash := contentHash(content)
	notes, _ := loadNotes(*vault)
	if dup := findDuplicate(notes, hash); dup != nil {
		fmt.Fprintf(os.Stderr, "duplicate: already saved as %s\n", dup.Path)
		os.Exit(0)
	}

	fmt.Println("Analyzing with AI...")
	result, err := analyze(apiKey, *model, *prompt, content)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: AI analysis failed: %v\n", err)
		os.Exit(1)
	}

	if *dryRun {
		md := renderMarkdown(content, result, hash)
		fmt.Print(md)
		return
	}

	path, err := writeMarkdown(noteDir, content, result, hash)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: could not write markdown: %v\n", err)
		os.Exit(1)
	}

	appendRelatedToNewNote(path, *vault)
	fmt.Printf("Saved to %s\n", path)
}

func requireAPIKey() string {
	key := os.Getenv("GEMINI_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "error: GEMINI_API_KEY environment variable is not set")
		os.Exit(1)
	}
	return key
}

func resolveInput(input string) (ContentInput, error) {
	if input == "clipboard" {
		fmt.Println("Reading image from clipboard...")
		data, mime, err := readClipboardImage()
		if err != nil {
			return ContentInput{}, err
		}
		return ContentInput{
			Kind:      "image",
			Source:    "clipboard",
			ImageData: data,
			ImageMIME: mime,
			ImageExt:  extFromMIME(mime),
		}, nil
	}

	if input == "-" {
		fmt.Println("Reading from stdin...")
		text := readStdin()
		if text == "" {
			return ContentInput{}, fmt.Errorf("no input received from stdin")
		}
		return ContentInput{Kind: "note", Text: text}, nil
	}

	if isPDF(input) {
		fmt.Printf("Reading PDF %s...\n", input)
		return readPDF(input)
	}

	if isURL(input) {
		if isTweetURL(input) {
			fmt.Printf("Extracting tweet %s...\n", input)
			extracted, err := extractTweet(input)
			if err != nil {
				fmt.Fprintf(os.Stderr, "warning: tweet extraction failed, treating as link: %v\n", err)
			} else {
				return ContentInput{
					Kind:   "link",
					Source: extracted.URL,
					Text:   fmt.Sprintf("Title: %s\n\n%s", extracted.Title, extracted.Text),
				}, nil
			}
		}

		if isImageURL(input) {
			fmt.Printf("Downloading image %s...\n", input)
			imgData, err := downloadImage(input)
			if err != nil {
				return ContentInput{}, fmt.Errorf("could not download image: %w", err)
			}
			return ContentInput{
				Kind:      "image",
				Source:    input,
				ImageData: imgData.Data,
				ImageMIME: imgData.MIMEType,
				ImageExt:  imgData.Extension,
			}, nil
		}

		fmt.Printf("Fetching %s...\n", input)
		extracted, err := extractFromURL(input)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not fetch URL: %v\n", err)
			return ContentInput{Kind: "link", Source: input, Text: input}, nil
		}
		return ContentInput{
			Kind:   "link",
			Source: extracted.URL,
			Text:   fmt.Sprintf("Title: %s\n\n%s", extracted.Title, extracted.Text),
		}, nil
	}

	return ContentInput{Kind: "note", Text: input}, nil
}

func processInput(vault, apiKey, model, customPrompt, input string) error {
	content, err := resolveInput(input)
	if err != nil {
		return err
	}

	hash := contentHash(content)
	notes, _ := loadNotes(vault)
	if dup := findDuplicate(notes, hash); dup != nil {
		fmt.Fprintf(os.Stderr, "  skipping duplicate: %s\n", dup.Path)
		return nil
	}

	result, err := analyze(apiKey, model, customPrompt, content)
	if err != nil {
		return fmt.Errorf("AI analysis failed: %w", err)
	}

	path, err := writeMarkdown(vault, content, result, hash)
	if err != nil {
		return fmt.Errorf("could not write markdown: %w", err)
	}

	appendRelatedToNewNote(path, vault)
	fmt.Printf("  Saved to %s\n", path)
	return nil
}

func isURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	return u.Scheme == "http" || u.Scheme == "https"
}

func readStdin() string {
	var sb strings.Builder
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		if sb.Len() > 0 {
			sb.WriteRune('\n')
		}
		sb.WriteString(scanner.Text())
	}
	return strings.TrimSpace(sb.String())
}
