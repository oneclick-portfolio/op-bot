package main

import (
	"net/http"
	"op-bot/internal/httpapi/response"
	"op-bot/internal/models"
)

type apiErrorPayload = models.APIErrorPayload

type apiErrorResponse = models.APIErrorResponse

// writeJSON is a wrapper for backwards compatibility.
func writeJSON(w http.ResponseWriter, status int, data any) {
	response.WriteJSON(w, status, data)
}

// defaultErrorCode is a wrapper for backwards compatibility.
func defaultErrorCode(status int) string {
	return response.DefaultErrorCode(status)
}

// writeError is a wrapper for backwards compatibility.
func writeError(w http.ResponseWriter, status int, message string, details ...string) {
	response.WriteError(w, status, message, details...)
}
