package helpers

import (
	"encoding/json"
	"log"
	"net/http"
)

type ErrorResponse struct {
	Error   string            `json:"error,omitempty"`
	Message string            `json:"message,omitempty"`
	Fields  map[string]string `json:"fields,omitempty"`
}

func ReadJson(r *http.Request, dst any) error {
	defer func() {
		if err := r.Body.Close(); err != nil {
			log.Println("warning: failed to close request body:", err)
		}
	}()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func WriteError(w http.ResponseWriter, status int, errR ErrorResponse) {
	WriteJson(w, status, errR)
}

func WriteJson(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		http.Error(w, "failed to encode response", http.StatusInternalServerError)
	}
}

func Error(w http.ResponseWriter, status int, msg string) {
	WriteError(w, status, ErrorResponse{
		Error: msg,
	})
}

func BadRequestError(w http.ResponseWriter) {
	Error(w, http.StatusBadRequest, "BAD_REQUEST")
}
func NotFoundError(w http.ResponseWriter) {
	Error(w, http.StatusNotFound, "NOT_FOUND")
}

func InternalServerError(w http.ResponseWriter) {
	Error(w, http.StatusInternalServerError, "INTERNAL_SERVER_ERROR")
}
