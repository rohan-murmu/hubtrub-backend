package handler

import (
	"encoding/json"
	"log"
	"net/http"
)

// ErrorResponse represents a standard error response
type ErrorResponse struct {
	Error string `json:"error"`
}

// SuccessResponse represents a standard success response
type SuccessResponse struct {
	Data    interface{} `json:"data,omitempty"`
	Message string      `json:"message,omitempty"`
}

// RespondError sends a JSON error response with the specified status code
func RespondError(w http.ResponseWriter, statusCode int, errorMessage string) {
	log.Printf("Error Response [%d]: %s", statusCode, errorMessage)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(ErrorResponse{
		Error: errorMessage,
	})
}

// RespondSuccess sends a JSON success response with data
func RespondSuccess(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// RespondWithMessage sends a JSON response with a message
func RespondWithMessage(w http.ResponseWriter, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": message,
		"data":    data,
	})
}
