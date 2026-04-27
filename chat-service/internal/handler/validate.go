package handler

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/fyodor/messenger/pkg/response"
)

// validateUUID checks that s is a non-empty, valid UUID.
// Writes a 400 response and returns false if invalid.
func validateUUID(w http.ResponseWriter, s, fieldName string) bool {
	if s == "" {
		response.Err(w, http.StatusBadRequest, fieldName+" is required")
		return false
	}
	if _, err := uuid.Parse(s); err != nil {
		response.Err(w, http.StatusBadRequest, "invalid "+fieldName)
		return false
	}
	return true
}
