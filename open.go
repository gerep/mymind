package main

import (
	"fmt"
	"os"
	"os/exec"
)

func runOpen(outDir string) {
	fmt.Printf("Opening %s...\n", outDir)
	if err := exec.Command("xdg-open", outDir).Start(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
	}
}
