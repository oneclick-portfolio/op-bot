package response

import (
	"encoding/json"
	"net/http"
	"op-bot/internal/models"
)

// WriteJSON writes a JSON response with the given status code and data.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// DefaultErrorCode returns a default error code for an HTTP status code.
func DefaultErrorCode(status int) string {
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

// WriteError writes an error response with the given status code and message.
func WriteError(w http.ResponseWriter, status int, message string, details ...string) {
	WriteJSON(w, status, models.APIErrorResponse{
		Error: models.APIErrorPayload{
			Code:    DefaultErrorCode(status),
			Message: message,
			Details: details,
		},
	})
}
