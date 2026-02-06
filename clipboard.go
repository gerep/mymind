package main

import (
	"fmt"
	"os/exec"
	"strings"
)

func readClipboardImage() ([]byte, string, error) {
	data, mime, err := tryWlPaste()
	if err == nil {
		return data, mime, nil
	}

	data, mime, err = tryXclip()
	if err == nil {
		return data, mime, nil
	}

	return nil, "", fmt.Errorf("could not read image from clipboard (tried wl-paste and xclip): %w", err)
}

func tryWlPaste() ([]byte, string, error) {
	mimeOut, err := exec.Command("wl-paste", "--list-types").Output()
	if err != nil {
		return nil, "", fmt.Errorf("wl-paste not available: %w", err)
	}

	mime := detectClipboardMIME(string(mimeOut))
	if mime == "" {
		return nil, "", fmt.Errorf("no image found in clipboard")
	}

	data, err := exec.Command("wl-paste", "--type", mime).Output()
	if err != nil {
		return nil, "", fmt.Errorf("wl-paste failed: %w", err)
	}

	if len(data) == 0 {
		return nil, "", fmt.Errorf("clipboard is empty")
	}

	return data, mime, nil
}

func tryXclip() ([]byte, string, error) {
	for _, mime := range []string{"image/png", "image/jpeg", "image/bmp"} {
		data, err := exec.Command("xclip", "-selection", "clipboard", "-target", mime, "-o").Output()
		if err == nil && len(data) > 0 {
			return data, mime, nil
		}
	}
	return nil, "", fmt.Errorf("xclip: no image in clipboard")
}

func detectClipboardMIME(types string) string {
	for _, mime := range []string{"image/png", "image/jpeg", "image/bmp", "image/webp", "image/gif"} {
		if strings.Contains(types, mime) {
			return mime
		}
	}
	return ""
}

func extFromMIME(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/bmp":
		return ".bmp"
	default:
		return ".png"
	}
}
