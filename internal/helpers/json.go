// Package helpers provides utility functions for handling HTTP requests,
// responses, and JSON encoding/decoding.
package helpers

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// ErrorResponse represents a standardized error response structure.
type ErrorResponse struct {
	Error   string            `json:"error,omitempty"`
	Message string            `json:"message,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// ReadJson reads and decodes a JSON request body into the provided destination.
func ReadJson(r *http.Request, dst any, logger *slog.Logger) error {
	defer func() {
		if err := r.Body.Close(); err != nil {
			logger.Warn("failed to close request body", "error", err)
		}
	}()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

// WriteError writes a structured error response with the given HTTP status.
func WriteError(w http.ResponseWriter, status int, errR ErrorResponse) {
	WriteJson(w, status, errR)
}

// WriteJson writes a JSON response with the given HTTP status code.
func WriteJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

// Error is a helper to send a simple error message response.
func Error(w http.ResponseWriter, status int, msg string) {
	WriteError(w, status, ErrorResponse{
		Error: msg,
	})
}

// BadRequestError sends a standardized 400 Bad Request response.
func BadRequestError(w http.ResponseWriter) {
	Error(w, http.StatusBadRequest, "BAD_REQUEST")
}

// NotFoundError sends a standardized 404 Not Found response.
func NotFoundError(w http.ResponseWriter) {
	Error(w, http.StatusNotFound, "NOT_FOUND")
}

// InternalServerError sends a standardized 500 Internal Server Error response.
func InternalServerError(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
}
