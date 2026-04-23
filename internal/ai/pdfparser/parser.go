package pdfparser

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

// Parser provides PDF parsing functionality using Gemini AI.
type Parser struct {
	APIKey       string
	ModelName    string
	SystemPrompt string
	UserPrompt   string
	SchemaJSON   string
}

// NewParser creates a new PDF parser.
func NewParser(apiKey, modelName, systemPrompt, userPrompt, schemaJSON string) *Parser {
	if modelName == "" {
		modelName = "gemini-3.1-flash-lite-preview"
	}
	return &Parser{
		APIKey:       apiKey,
		ModelName:    modelName,
		SystemPrompt: systemPrompt,
		UserPrompt:   userPrompt,
		SchemaJSON:   schemaJSON,
	}
}

// ExtractJSON isolates the JSON payload if the LLM wrapped it in markdown.
func ExtractJSON(raw string) string {
	firstCurly := strings.Index(raw, "{")
	lastCurly := strings.LastIndex(raw, "}")

	if firstCurly != -1 && lastCurly != -1 && lastCurly > firstCurly {
		return raw[firstCurly : lastCurly+1]
	}
	return raw
}

// ParseToJSON sends a resume PDF to Gemini and returns structured JSON
// matching the v5 Reactive Resume schema.
func (p *Parser) ParseToJSON(ctx context.Context, pdfBytes []byte) (map[string]any, error) {
	if p.APIKey == "" {
		return nil, fmt.Errorf("google API key is not configured")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(p.APIKey))
	if err != nil {
		return nil, fmt.Errorf("failed to create genai client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(p.ModelName)
	model.ResponseMIMEType = "application/json"

	// Build the full system instruction: system prompt + schema template.
	systemInstruction := p.SystemPrompt + "\n\n## JSON Schema Template\n\n```json\n" + p.SchemaJSON + "\n```\n"

	model.SystemInstruction = &genai.Content{
		Parts: []genai.Part{genai.Text(systemInstruction)},
	}

	prompt := []genai.Part{
		genai.Text(p.UserPrompt),
		genai.Blob{
			MIMEType: "application/pdf",
			Data:     pdfBytes,
		},
	}

	slog.InfoContext(ctx, "ai.request.start", "model", p.ModelName, "pdf_bytes", len(pdfBytes))

	resp, err := model.GenerateContent(ctx, prompt...)
	if err != nil {
		slog.ErrorContext(ctx, "ai.request.failed", "model", p.ModelName, "error", err)
		return nil, fmt.Errorf("AI generation failed: %w", err)
	}

	slog.InfoContext(ctx, "ai.response.received", "candidate_count", len(resp.Candidates))

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		slog.WarnContext(ctx, "ai.response.empty",
			"candidate_count", len(resp.Candidates),
			"finish_reason", finishReasonStr(resp),
		)
		return nil, fmt.Errorf("no content returned from AI")
	}

	slog.DebugContext(ctx, "ai.response.candidate",
		"finish_reason", resp.Candidates[0].FinishReason,
		"part_count", len(resp.Candidates[0].Content.Parts),
	)

	textPart, ok := resp.Candidates[0].Content.Parts[0].(genai.Text)
	if !ok {
		slog.ErrorContext(ctx, "ai.response.unexpected_type",
			"part_type", fmt.Sprintf("%T", resp.Candidates[0].Content.Parts[0]),
		)
		return nil, fmt.Errorf("unexpected response type from AI")
	}

	rawResult := string(textPart)
	slog.InfoContext(ctx, "ai.response.text",
		"raw_length", len(rawResult),
		"raw_preview", truncate(rawResult, 300),
	)

	cleanJSONStr := ExtractJSON(rawResult)
	if cleanJSONStr == rawResult && (!strings.HasPrefix(strings.TrimSpace(rawResult), "{")) {
		slog.WarnContext(ctx, "ai.response.json_extraction_failed",
			"raw_preview", truncate(rawResult, 300),
		)
	} else {
		slog.DebugContext(ctx, "ai.response.json_extracted", "json_length", len(cleanJSONStr))
	}

	var data map[string]any
	if err := json.Unmarshal([]byte(cleanJSONStr), &data); err != nil {
		slog.ErrorContext(ctx, "ai.response.json_parse_failed",
			"error", err,
			"json_preview", truncate(cleanJSONStr, 500),
		)
		return nil, fmt.Errorf("JSON parse failed: %w\nRaw response (first 500 chars): %.500s", err, cleanJSONStr)
	}

	slog.InfoContext(ctx, "ai.response.json_parsed_ok", "top_level_keys", len(data))
	return data, nil
}

// finishReasonStr safely extracts the finish reason from a GenerateContentResponse.
func finishReasonStr(resp *genai.GenerateContentResponse) string {
	if len(resp.Candidates) == 0 {
		return "no_candidates"
	}
	return fmt.Sprintf("%v", resp.Candidates[0].FinishReason)
}

// truncate returns at most n characters of s.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
