package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

const resumeSchemaURL = "https://rxresu.me/schema.json"

const resumeSchemaFetchTimeout = 5 * time.Second

type validationResult struct {
	Valid  bool     `json:"valid"`
	Errors []string `json:"errors"`
}

func getResumeSchema() (*jsonschema.Schema, error) {
	resumeSchemaOnce.Do(func() {
		schemaCompiler := jsonschema.NewCompiler()

		// Mirror the official flow: fetch schema JSON, then compile validator from it.
		schemaData, err := fetchRemoteResumeSchema()
		if err != nil {
			if isNetworkError(err) {
				log.Printf("resume.schema remote_fetch_failed url=%s err=%v; using local fallback", resumeSchemaURL, err)
				schemaData, err = loadLocalResumeSchema()
				if err != nil {
					resumeSchemaErr = fmt.Errorf("schema fetch failed and local fallback failed: %w", err)
					log.Printf("resume.schema local_fallback_failed err=%v", resumeSchemaErr)
					return
				}
				log.Printf("resume.schema local_fallback_loaded bytes=%d", len(schemaData))
			} else {
				resumeSchemaErr = fmt.Errorf("unable to load resume schema: %w", err)
				log.Printf("resume.schema load_failed err=%v", resumeSchemaErr)
				return
			}
		} else {
			log.Printf("resume.schema remote_loaded url=%s bytes=%d", resumeSchemaURL, len(schemaData))
		}

		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schemaData))
		if err != nil {
			resumeSchemaErr = fmt.Errorf("unable to parse resume schema: %w", err)
			return
		}

		// Use the official URL as resource key so relative $refs resolve correctly.
		if err := schemaCompiler.AddResource(resumeSchemaURL, doc); err != nil {
			resumeSchemaErr = fmt.Errorf("unable to register resume schema: %w", err)
			return
		}

		resumeSchema, resumeSchemaErr = schemaCompiler.Compile(resumeSchemaURL)
		if resumeSchemaErr != nil {
			resumeSchemaErr = fmt.Errorf("unable to compile resume schema from %s: %w", resumeSchemaURL, resumeSchemaErr)
			log.Printf("resume.schema compile_failed err=%v", resumeSchemaErr)
			return
		}

		log.Printf("resume.schema compile_success")
	})
	return resumeSchema, resumeSchemaErr
}

func fetchRemoteResumeSchema() ([]byte, error) {
	client := &http.Client{Timeout: resumeSchemaFetchTimeout}
	resp, err := client.Get(resumeSchemaURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to load resume schema: HTTP %d", resp.StatusCode)
	}

	schemaData, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read resume schema: %w", err)
	}

	return schemaData, nil
}

func loadLocalResumeSchema() ([]byte, error) {
	candidates := []string{
		filepath.Join(backendDir, "schema", "v5.json"),
		filepath.Join(backendDir, "..", "schema", "v5.json"),
		filepath.Join("schema", "v5.json"),
	}

	var lastErr error
	for _, candidate := range candidates {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data, nil
		}
		lastErr = err
	}

	if lastErr == nil {
		lastErr = errors.New("local schema not found")
	}

	return nil, fmt.Errorf("unable to read local schema file schema/v5.json from known paths: %w", lastErr)
}

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr)
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
