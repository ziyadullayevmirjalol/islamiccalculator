// Package httpx holds the JSON response envelope shared by all handlers:
// {"data": ...} on success, {"error": {code, message, fields}} on failure.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/diyorbek/islamiccalculator/internal/pkg/apperr"
)

type dataEnvelope struct {
	Data any `json:"data"`
}

type errorEnvelope struct {
	Error *apperr.Error `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		slog.Error("encode response", "err", err)
	}
}

// Data writes a success envelope.
func Data(w http.ResponseWriter, status int, data any) {
	writeJSON(w, status, dataEnvelope{Data: data})
}

// Err writes an error envelope, logging the cause of internal errors
// while keeping the client message generic.
func Err(w http.ResponseWriter, err error) {
	e := apperr.From(err)
	if e.Code == apperr.CodeInternal {
		slog.Error("internal error", "err", err)
	}
	writeJSON(w, e.HTTPStatus(), errorEnvelope{Error: e})
}

// Decode parses a JSON request body into dst, returning a typed
// validation error on malformed input.
func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return apperr.Validation("request body too large", nil)
		}
		return apperr.Validation("invalid JSON body", nil)
	}
	return nil
}
