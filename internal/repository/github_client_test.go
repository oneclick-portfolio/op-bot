package repository

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestRequestMapsAPIError(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusUnauthorized,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"message":"bad token"}`)),
		}, nil
	})}

	gh := NewGitHubClient(client)
	_, err := gh.Request("t", "/user", http.MethodGet, nil)
	if err == nil {
		t.Fatal("expected error")
	}
	apiErr, ok := err.(*APIError)
	if !ok {
		t.Fatalf("expected *APIError, got %T", err)
	}
	if apiErr.Status != http.StatusUnauthorized {
		t.Fatalf("status=%d, want=%d", apiErr.Status, http.StatusUnauthorized)
	}
}

func TestGetGitHubInstallationsFromTokenParsesList(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(req *http.Request) (*http.Response, error) {
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(`{"installations":[{"id":1}]}`)),
		}, nil
	})}

	gh := NewGitHubClient(client)
	list, err := gh.GetGitHubInstallationsFromToken("t")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("len=%d, want 1", len(list))
	}
}
