package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func runBatch(outDir, apiKey, model, customPrompt string, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "Usage: mymind batch <file>")
		return
	}

	filePath := args[0]
	f, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return
	}
	defer f.Close()

	var total, errors int
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		total++
		fmt.Printf("Processing: %s...\n", line)
		if err := processInput(outDir, apiKey, model, customPrompt, line); err != nil {
			fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			errors++
		}
	}

	fmt.Printf("Processed %d inputs (%d errors)\n", total, errors)
}
