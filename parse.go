package main

import (
	"context"
	_ "embed"
	"io"
	"net/http"
	"op-bot/internal/ai/pdfparser"
)

//go:embed ai/pdf-parser-system.md
var pdfParserSystemPrompt string

//go:embed ai/pdf-parser-user.md
var pdfParserUserPrompt string

// Module-level parser for backwards compatibility
var pdfParser *pdfparser.Parser

// getSchemaFromWeb fetches the v5 schema from the web.
func getSchemaFromWeb() (string, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/openpatriot/op-bot/main/schema/v5.json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	schemaBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(schemaBytes), nil
}

// InitPDFParser initializes the PDF parser (called during LoadAppContext).
func initPDFParser() {
	v5SchemaJSON, err := getSchemaFromWeb()
	if err != nil {
		panic("failed to fetch v5 schema: " + err.Error())
	}
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
