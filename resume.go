package main

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

type validationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

func getResumeSchema() (*jsonschema.Schema, error) {
	resumeSchemaOnce.Do(func() {
		schemaCompiler = jsonschema.NewCompiler()
		schemaPath := filepath.Join(backendDir, "tests", "rxresume.schema.json")

		var schemaData []byte
		schemaData, resumeSchemaErr = os.ReadFile(schemaPath)
		if resumeSchemaErr != nil {
			resp, err := http.Get("https://rxresu.me/schema.json")
			if err != nil {
				resumeSchemaErr = fmt.Errorf("unable to load resume schema: %w", err)
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				resumeSchemaErr = fmt.Errorf("unable to load resume schema: HTTP %d", resp.StatusCode)
				return
			}
			schemaData, resumeSchemaErr = io.ReadAll(resp.Body)
			if resumeSchemaErr != nil {
				return
			}
		}

		if err := schemaCompiler.AddResource("schema.json", bytes.NewReader(schemaData)); err != nil {
			resumeSchemaErr = err
			return
		}
		resumeSchema, resumeSchemaErr = schemaCompiler.Compile("schema.json")
	})
	return resumeSchema, resumeSchemaErr
}

func formatSchemaErrors(err error) []string {
	if err == nil {
		return nil
	}
	var errors []string
	lines := strings.Split(err.Error(), "\n")
	for i, line := range lines {
		if i >= 20 {
			break
		}
		line = strings.TrimSpace(line)
		if line != "" {
			errors = append(errors, line)
		}
	}
	return errors
}

func validateResumeData(data any) validationResult {
	if data == nil {
		return validationResult{Valid: false, Errors: []string{"/ must be a JSON object"}}
	}
	obj, ok := data.(map[string]any)
	if !ok {
		return validationResult{Valid: false, Errors: []string{"/ must be a JSON object"}}
	}
	schema, err := getResumeSchema()
	if err != nil {
		return validationResult{Valid: false, Errors: []string{err.Error()}}
	}
	if err := schema.Validate(obj); err != nil {
		return validationResult{Valid: false, Errors: formatSchemaErrors(err)}
	}
	return validationResult{Valid: true, Errors: []string{}}
}
