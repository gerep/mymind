package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

func isTweetURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := strings.ToLower(u.Hostname())
	return (strings.Contains(host, "twitter.com") || strings.Contains(host, "x.com")) &&
		strings.Contains(u.Path, "/status/")
}

func extractTweet(rawURL string) (Extracted, error) {
	endpoint := "https://publish.twitter.com/oembed?url=" + url.QueryEscape(rawURL) + "&omit_script=1&dnt=1"

	resp, err := httpClient.Get(endpoint)
	if err != nil {
		return Extracted{}, fmt.Errorf("twitter oembed request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return Extracted{}, fmt.Errorf("twitter oembed HTTP %d", resp.StatusCode)
	}

	var oembed struct {
		HTML       string `json:"html"`
		AuthorName string `json:"author_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&oembed); err != nil {
		return Extracted{}, fmt.Errorf("twitter oembed decode: %w", err)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(oembed.HTML))
	if err != nil {
		return Extracted{}, fmt.Errorf("twitter oembed parse html: %w", err)
	}

	text := collapseWhitespace(strings.TrimSpace(doc.Text()))

	title := fmt.Sprintf("Tweet by %s", oembed.AuthorName)

	return Extracted{
		URL:   rawURL,
		Title: title,
		Text:  text,
	}, nil
}
