package main

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	_ "golang.org/x/image/webp"

	"golang.org/x/image/draw"
)

// renderImageToText loads an image file and renders it as colored half-block
// characters. Each character cell represents 1×2 pixels using the "▀" glyph
// with foreground (top pixel) and background (bottom pixel) colors.
func renderImageToText(path string, maxWidth int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	src, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}

	bounds := src.Bounds()
	srcW := bounds.Dx()
	srcH := bounds.Dy()

	if maxWidth <= 0 {
		maxWidth = 80
	}

	// Scale to fit terminal width, maintaining aspect ratio.
	// Terminal characters are roughly twice as tall as wide, so we
	// halve the vertical resolution (each char = 2 pixel rows).
	dstW := srcW
	dstH := srcH
	if dstW > maxWidth {
		dstH = dstH * maxWidth / dstW
		dstW = maxWidth
	}

	// Ensure height is even for half-block pairing
	if dstH%2 != 0 {
		dstH++
	}

	// Resize
	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var sb strings.Builder
	for y := 0; y < dstH; y += 2 {
		for x := 0; x < dstW; x++ {
			tr, tg, tb, _ := dst.At(x, y).RGBA()
			br, bg, bb, _ := dst.At(x, y+1).RGBA()

			// RGBA returns 16-bit values; shift to 8-bit
			fmt.Fprintf(&sb, "\033[38;2;%d;%d;%dm\033[48;2;%d;%d;%dm▀",
				tr>>8, tg>>8, tb>>8,
				br>>8, bg>>8, bb>>8,
			)
		}
		sb.WriteString("\033[0m\n")
	}

	return sb.String(), nil
}
