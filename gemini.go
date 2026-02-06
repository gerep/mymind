package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"google.golang.org/genai"
)

type AIResult struct {
	Title   string   `json:"title"`
	Tags    []string `json:"tags"`
	Summary string   `json:"summary"`
}

func analyze(apiKey string, content ContentInput) (AIResult, error) {
	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return AIResult{}, fmt.Errorf("creating Gemini client: %w", err)
	}

	prompt := buildPrompt(content)

	var parts []*genai.Part
	if content.Kind == "image" {
		parts = []*genai.Part{
			genai.NewPartFromBytes(content.ImageData, content.ImageMIME),
			genai.NewPartFromText(prompt),
		}
	} else {
		parts = []*genai.Part{
			genai.NewPartFromText(prompt),
		}
	}

	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	config := &genai.GenerateContentConfig{
		Temperature: genai.Ptr(float32(0.3)),
	}

	resp, err := client.Models.GenerateContent(ctx, "gemini-2.0-flash", contents, config)
	if err != nil {
		return AIResult{}, fmt.Errorf("generating content: %w", err)
	}

	text := resp.Text()
	result, err := parseAIResult(text)
	if err != nil {
		return AIResult{}, fmt.Errorf("parsing AI response: %w", err)
	}

	return result, nil
}

func buildPrompt(content ContentInput) string {
	var sb strings.Builder
	sb.WriteString(`Analyze the following content and return ONLY valid JSON (no markdown fences, no extra text) with this exact schema:
{"title": "short descriptive title", "tags": ["tag1", "tag2"], "summary": "2-3 sentence summary"}

Rules for tags:
- 3 to 8 tags
- Use kebab-case (e.g., "machine-learning", not "Machine Learning")
- Short, specific, no duplicates

`)

	switch content.Kind {
	case "link":
		fmt.Fprintf(&sb, "This is a web page. URL: %s\n\n", content.Source)
		sb.WriteString("Content:\n")
		sb.WriteString(content.Text)
	case "image":
		fmt.Fprintf(&sb, "This is an image from URL: %s\n", content.Source)
		sb.WriteString("Analyze the image and generate a descriptive title, relevant tags, and a summary of what the image shows.\n")
	default:
		sb.WriteString("This is a personal note.\n\n")
		sb.WriteString("Content:\n")
		sb.WriteString(content.Text)
	}

	return sb.String()
}

func parseAIResult(text string) (AIResult, error) {
	text = strings.TrimSpace(text)
	text = strings.TrimPrefix(text, "```json")
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSuffix(text, "```")
	text = strings.TrimSpace(text)

	var result AIResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return AIResult{}, fmt.Errorf("invalid JSON from AI: %w\nraw response: %s", err, text)
	}

	for i, tag := range result.Tags {
		tag = strings.ToLower(tag)
		tag = strings.TrimPrefix(tag, "#")
		tag = strings.ReplaceAll(tag, " ", "-")
		result.Tags[i] = tag
	}
	sort.Strings(result.Tags)

	return result, nil
}
