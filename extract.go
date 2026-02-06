package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

const maxExtractChars = 12000

var imageExtensions = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
	".svg":  "image/svg+xml",
}

type Extracted struct {
	URL   string
	Title string
	Text  string
}

type ImageDownload struct {
	Data      []byte
	MIMEType  string
	Extension string
}

func urlPathExt(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return strings.ToLower(path.Ext(u.Path))
}

func isImageURL(rawURL string) bool {
	if ext := urlPathExt(rawURL); ext != "" {
		if _, ok := imageExtensions[ext]; ok {
			return true
		}
	}

	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Head(rawURL)
	if err == nil {
		resp.Body.Close()
		ct := resp.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "image/") {
			return true
		}
	}

	resp2, err := client.Get(rawURL)
	if err != nil {
		return false
	}
	resp2.Body.Close()
	return strings.HasPrefix(resp2.Header.Get("Content-Type"), "image/")
}

func downloadImage(rawURL string) (ImageDownload, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return ImageDownload{}, err
	}
	req.Header.Set("User-Agent", "mymind/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return ImageDownload{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ImageDownload{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return ImageDownload{}, fmt.Errorf("reading image body: %w", err)
	}

	ct := resp.Header.Get("Content-Type")
	mime, ext := detectImageType(rawURL, ct)

	return ImageDownload{
		Data:      data,
		MIMEType:  mime,
		Extension: ext,
	}, nil
}

func detectImageType(rawURL, contentType string) (mime string, ext string) {
	urlExt := urlPathExt(rawURL)
	if m, ok := imageExtensions[urlExt]; ok {
		return m, urlExt
	}

	for e, m := range imageExtensions {
		if strings.Contains(contentType, m) {
			return m, e
		}
	}

	return "image/jpeg", ".jpg"
}

func extractFromURL(rawURL string) (Extracted, error) {
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", rawURL, nil)
	if err != nil {
		return Extracted{}, err
	}
	req.Header.Set("User-Agent", "mymind/0.1")

	resp, err := client.Do(req)
	if err != nil {
		return Extracted{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Extracted{}, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return Extracted{}, err
	}

	title := strings.TrimSpace(doc.Find("title").Text())

	doc.Find("script, style, nav, footer, aside").Remove()
	body := strings.TrimSpace(doc.Find("body").Text())
	body = collapseWhitespace(body)

	if len(body) > maxExtractChars {
		body = body[:maxExtractChars]
	}

	return Extracted{
		URL:   rawURL,
		Title: title,
		Text:  body,
	}, nil
}

func collapseWhitespace(s string) string {
	var b strings.Builder
	prevSpace := false
	for _, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			if !prevSpace {
				b.WriteRune(' ')
			}
			prevSpace = true
		} else {
			b.WriteRune(r)
			prevSpace = false
		}
	}
	return b.String()
}
