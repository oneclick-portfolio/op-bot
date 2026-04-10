package main

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

//go:embed ai/pdf-parser-system.md
var pdfParserSystemPrompt string

//go:embed ai/pdf-parser-user.md
var pdfParserUserPrompt string

//go:embed schema/v5.json
var v5SchemaJSON string

// ExtractJSON isolates the JSON payload if the LLM wrapped it in markdown.
func ExtractJSON(raw string) string {
	firstCurly := strings.Index(raw, "{")
	lastCurly := strings.LastIndex(raw, "}")

	if firstCurly != -1 && lastCurly != -1 && lastCurly > firstCurly {
		return raw[firstCurly : lastCurly+1]
	}
	return raw
}

// ParsePDFToJSON sends a resume PDF to Gemini and returns structured JSON
// matching the v5 Reactive Resume schema.
func ParsePDFToJSON(ctx context.Context, apiKey string, pdfBytes []byte) (map[string]any, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("google API key is not configured")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel("gemini-3.1-pro-preview")
	model.ResponseMIMEType = "application/json"

	// Build the full system instruction: system prompt + schema template.
	systemInstruction := pdfParserSystemPrompt + "\n\n## JSON Schema Template\n\n```json\n" + v5SchemaJSON + "\n```\n"

	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemInstruction)},
	}

	prompt := []genai.Part{
		genai.Text(pdfParserUserPrompt),
		genai.Blob{
			MIMEType: "application/pdf",
			Data:     pdfBytes,
		},
	}

	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("no content returned from AI")
	}

	textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		return nil, fmt.Errorf("unexpected response type from AI")
	}

	rawResult := string(textPart)
	cleanJSONStr := ExtractJSON(rawResult)

	var data map[string]any
	if err := json.Unmarshal([]byte(cleanJSONStr), &data); err != nil {
		return nil, fmt.Errorf("JSON parse failed: %w\nRaw response (first 500 chars): %.500s", err, cleanJSONStr)
	}

	return data, nil
}
