package main

import (
	"fmt"
	"os"
	"strings"
)

const maxPDFSize = 20 * 1024 * 1024

func isPDF(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".pdf") {
		return false
	}
	_, err := os.Stat(path)
	return err == nil
}

func readPDF(path string) (ContentInput, error) {
	info, err := os.Stat(path)
	if err != nil {
		return ContentInput{}, fmt.Errorf("cannot access PDF file: %w", err)
	}

	if info.Size() > maxPDFSize {
		return ContentInput{}, fmt.Errorf("PDF file too large: %d bytes (max %d)", info.Size(), maxPDFSize)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return ContentInput{}, fmt.Errorf("cannot read PDF file: %w", err)
	}

	return ContentInput{
		Kind:      "pdf",
		Source:    path,
		ImageData: data,
		ImageMIME: "application/pdf",
	}, nil
}
