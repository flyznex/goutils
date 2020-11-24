package httpext

import (
	"context"
	"encoding/json"
	"net/http"
)

type Errorer interface {
	Error() error
}

// Error response struct
// swagger:response errorResponse
type ErrorResponse struct {
	// in:body
	Error string `json:"err"`
}

// encode errors from business-logic
func EncodeError(_ context.Context, err error, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	switch err {
	default:
		w.WriteHeader(http.StatusInternalServerError)
	}
	json.NewEncoder(w).Encode(ErrorResponse{err.Error()})
}