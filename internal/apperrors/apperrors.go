package apperrors

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/SlayerSv/load-balancer/internal/logger"
	"github.com/SlayerSv/load-balancer/pkg"
)

var ErrAlreadyExists = errors.New("already exists")
var ErrBadRequest = errors.New("bad request")
var ErrInternal = errors.New("internal server error")
var ErrNotFound = errors.New("not found")
var ErrUnauthorized = errors.New("unauthorized")
var ErrRateLimitExceeded = errors.New("rate limit exceeded")
var ErrServiceUnavailable = errors.New("service unavailable")

// ErrorResponse is the message that will be sent on errors during request processing.
type ErrorResponse struct {
	// Code is the http status code (404, 500, 401, etc)
	Code int `json:"code"`
	// Message is the text message describing error
	Message string `json:"message"`
}

var Log logger.Logger

// Error writes error message to client in json format
func Error(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	var code int
	switch {
	case errors.Is(err, ErrAlreadyExists):
		code = http.StatusConflict
	case errors.Is(err, ErrUnauthorized):
		code = http.StatusUnauthorized
	case errors.Is(err, ErrNotFound):
		code = http.StatusNotFound
	case errors.Is(err, ErrRateLimitExceeded):
		code = http.StatusTooManyRequests
	case errors.Is(err, ErrBadRequest):
		code = http.StatusBadRequest
	case errors.Is(err, ErrServiceUnavailable):
		code = http.StatusServiceUnavailable
	default:
		code = http.StatusInternalServerError
	}
	if code == http.StatusInternalServerError {
		Log.Error("Error serving request", "request_id", r.Context().Value(pkg.RequestID), "error", err)
		err = ErrInternal
	} else {
		Log.Info("Sending error response", "request_id", r.Context().Value(pkg.RequestID), "code", code, "message", err.Error())
	}
	errorResponse := ErrorResponse{
		Code:    code,
		Message: err.Error(),
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse)
}
