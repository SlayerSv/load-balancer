package apperrors

import (
	"encoding/json"
	"errors"
	"net/http"
)

var ErrAlreadyExists = errors.New("already exists")
var ErrInternal = errors.New("internal server error")
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
var ErrRateLimitExceeded = errors.New("rate limit exceeded")

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func Error(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	var code int
	errorResponse := ErrorResponse{
		Code:    code,
		Message: err.Error(),
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse)
}
