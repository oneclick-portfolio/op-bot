package main

import (
	"context"
	_ "embed"
	"op-bot/internal/ai/pdfparser"
)

//go:embed ai/pdf-parser-system.md
var pdfParserSystemPrompt string

//go:embed ai/pdf-parser-user.md
var pdfParserUserPrompt string

//go:embed schema/v5.json
var v5SchemaJSON string

// Module-level parser for backwards compatibility
var pdfParser *pdfparser.Parser

// InitPDFParser initializes the PDF parser (called during LoadAppContext).
func initPDFParser() {
	pdfParser = pdfparser.NewParser(googleAPIKey, geminiModel, pdfParserSystemPrompt, pdfParserUserPrompt, v5SchemaJSON)
}

// ExtractJSON is a wrapper for backwards compatibility, including for tests.
func ExtractJSON(raw string) string {
	return pdfparser.ExtractJSON(raw)
}

// ParsePDFToJSON is a wrapper for backwards compatibility.
func ParsePDFToJSON(ctx context.Context, apiKey string, pdfBytes []byte) (map[string]any, error) {
	if pdfParser == nil {
		initPDFParser()
	}
	return pdfParser.ParseToJSON(ctx, pdfBytes)
}
