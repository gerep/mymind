package main

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"google.golang.org/genai"
)

type AIResult struct {
	Title   string   `json:"title"`
	Tags    []string `json:"tags"`
	Summary string   `json:"summary"`
}

func analyze(apiKey, model, customPrompt string, content ContentInput) (AIResult, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return AIResult{}, fmt.Errorf("creating Gemini client: %w", err)
	}

	prompt := buildPrompt(content, customPrompt)

	var parts []*genai.Part
	switch content.Kind {
	case "image":
		parts = []*genai.Part{
			genai.NewPartFromBytes(content.ImageData, content.ImageMIME),
			genai.NewPartFromText(prompt),
		}
	case "pdf":
		parts = []*genai.Part{
			genai.NewPartFromBytes(content.ImageData, "application/pdf"),
			genai.NewPartFromText(prompt),
		}
	default:
		parts = []*genai.Part{
			genai.NewPartFromText(prompt),
		}
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	config := &genai.GenerateContentConfig{
		Temperature:      genai.Ptr(float32(0.3)),
		ResponseMIMEType: "application/json",
	}

	resp, err := client.Models.GenerateContent(ctx, model, contents, config)
	if err != nil {
		return AIResult{}, fmt.Errorf("generating content: %w", err)
	}

	text := resp.Text()
	result, err := parseAIResult(text)
	if err != nil {
		return AIResult{}, fmt.Errorf("parsing AI response: %w", err)
	}

	result = validateResult(result)

	return result, nil
}

func buildPrompt(content ContentInput, customPrompt string) string {
	var sb strings.Builder
	sb.WriteString(`Analyze the following content and return ONLY valid JSON with this exact schema:
{"title": "short descriptive title", "tags": ["tag1", "tag2"], "summary": "2-3 sentence summary"}

Rules for tags:
- 3 to 8 tags
- Use kebab-case (e.g., "machine-learning", not "Machine Learning")
- Short, specific, no duplicates
- Include common synonyms and alternative names (e.g., both "go" and "golang", both "js" and "javascript", both "k8s" and "kubernetes")

`)

	if customPrompt != "" {
		fmt.Fprintf(&sb, "Additional instructions: %s\n\n", customPrompt)
	}

	switch content.Kind {
	case "link":
		fmt.Fprintf(&sb, "This is a web page. URL: %s\n\n", content.Source)
		sb.WriteString("Content:\n")
		sb.WriteString(content.Text)
	case "image":
		fmt.Fprintf(&sb, "This is an image from: %s\n", content.Source)
		sb.WriteString("Analyze the image and generate a descriptive title, relevant tags, and a summary of what the image shows.\n")
	case "pdf":
		fmt.Fprintf(&sb, "This is a PDF document from: %s\n", content.Source)
		sb.WriteString("Analyze the PDF and generate a descriptive title, relevant tags, and a summary of the document's content.\n")
	default:
		sb.WriteString("This is a personal note.\n\n")
		sb.WriteString("Content:\n")
		sb.WriteString(content.Text)
	}

	return sb.String()
}

func parseAIResult(text string) (AIResult, error) {
	text = strings.TrimSpace(text)

	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start != -1 && end != -1 && end > start {
		text = text[start : end+1]
	}

	var result AIResult
	if err := json.Unmarshal([]byte(text), &result); err == nil {
		return result, nil
	}

	var results []AIResult
	if err := json.Unmarshal([]byte("["+text+"]"), &results); err != nil {
		return AIResult{}, fmt.Errorf("invalid JSON from AI: %w\nraw response: %s", err, text)
	}

	return mergeResults(results), nil
}

func mergeResults(results []AIResult) AIResult {
	if len(results) == 0 {
		return AIResult{}
	}
	if len(results) == 1 {
		return results[0]
	}

	merged := AIResult{
		Title: results[0].Title,
	}

	var summaries []string
	seen := make(map[string]bool)
	for _, r := range results {
		if r.Summary != "" {
			summaries = append(summaries, r.Summary)
		}
		for _, tag := range r.Tags {
			if !seen[tag] {
				seen[tag] = true
				merged.Tags = append(merged.Tags, tag)
			}
		}
	}
	merged.Summary = strings.Join(summaries, " ")

	return merged
}

var tagSanitizer = regexp.MustCompile(`[^a-z0-9-]+`)

func sanitizeTag(tag string) string {
	tag = strings.ToLower(tag)
	tag = strings.TrimPrefix(tag, "#")
	tag = tagSanitizer.ReplaceAllString(tag, "-")
	tag = strings.Trim(tag, "-")
	return tag
}

func validateResult(result AIResult) AIResult {
	if result.Title == "" {
		result.Title = "untitled"
	}
	if result.Summary == "" {
		result.Summary = "(no summary)"
	}

	seen := make(map[string]bool)
	var tags []string
	for _, tag := range result.Tags {
		tag = sanitizeTag(tag)
		if tag != "" && !seen[tag] {
			seen[tag] = true
			tags = append(tags, tag)
		}
	}
	sort.Strings(tags)
	result.Tags = tags

	return result
}
