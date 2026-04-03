package main

import (
	"encoding/json"
	"net/http"
)

type apiErrorPayload struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details,omitempty"`
}

type apiErrorResponse struct {
	Error apiErrorPayload `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func defaultErrorCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "BAD_REQUEST"
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusNotFound:
		return "NOT_FOUND"
	case http.StatusInternalServerError:
		return "INTERNAL_ERROR"
	default:
		if status >= 500 {
			return "INTERNAL_ERROR"
		}
		return "REQUEST_FAILED"
	}
}

func writeError(w http.ResponseWriter, status int, message string, details ...string) {
	writeJSON(w, status, apiErrorResponse{
		Error: apiErrorPayload{
			Code:    defaultErrorCode(status),
			Message: message,
			Details: details,
		},
	})
}
