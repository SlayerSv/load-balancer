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

type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

var Log logger.Logger

func Error(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Set("Content-Type", "application/json")
	Log.Error("Error serving request", "request_id", r.Context().Value(pkg.RequestID), "error", err)
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
	default:
		code = http.StatusInternalServerError
		err = ErrInternal
	}
	errorResponse := ErrorResponse{
		Code:    code,
		Message: err.Error(),
	}
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse)
}
