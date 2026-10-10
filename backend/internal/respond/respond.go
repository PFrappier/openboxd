// Package respond writes the JSON responses of the API.
package respond

import (
	"encoding/json"
	"net/http"
)

// JSON writes v as the JSON body of a response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// Error writes msg as {"error": msg}, the body every error response has.
func Error(w http.ResponseWriter, status int, msg string) {
	JSON(w, status, map[string]string{"error": msg})
}
