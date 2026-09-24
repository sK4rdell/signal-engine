package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
)

// CodeUnsupportedMediaType is returned when a body is not application/json.
const CodeUnsupportedMediaType = "unsupported_media_type"

// decodeBody reads a bounded JSON body into req with strict semantics:
// a single JSON object, no unknown fields, no trailing content.
func decodeBody(w http.ResponseWriter, r *http.Request, req any, limit int64, optional bool) error {
	body := http.MaxBytesReader(w, r.Body, limit)
	raw, err := io.ReadAll(body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return apperror.RequestTooLarge()
		}
		return apperror.InvalidRequest("The request body could not be read").WithCause(err)
	}

	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		if optional {
			return nil
		}
		return apperror.InvalidRequest("A JSON request body is required")
	}

	if ct := r.Header.Get("Content-Type"); ct != "" {
		mediaType, _, err := mime.ParseMediaType(ct)
		if err != nil || mediaType != "application/json" {
			return apperror.New(http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
		}
	} else {
		return apperror.New(http.StatusUnsupportedMediaType, CodeUnsupportedMediaType, "Content-Type must be application/json")
	}

	if raw[0] != '{' {
		return apperror.InvalidRequest("The request body must be a JSON object")
	}

	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(req); err != nil {
		return jsonDecodeError(err)
	}
	if dec.More() {
		return apperror.InvalidRequest("The request body must contain a single JSON object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return apperror.InvalidRequest("The request body must contain a single JSON object")
	}
	return nil
}

func jsonDecodeError(err error) error {
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr), errors.Is(err, io.ErrUnexpectedEOF):
		return apperror.InvalidRequest("The request body is not valid JSON").WithCause(err)
	case errors.As(err, &typeErr):
		field := typeErr.Field
		if field == "" {
			return apperror.InvalidRequest("The request body must be a JSON object").WithCause(err)
		}
		return apperror.InvalidRequestFields("The request contains invalid fields", map[string]string{
			field: fmt.Sprintf("must be a %s", jsonTypeName(typeErr)),
		}).WithCause(err)
	}
	// encoding/json reports unknown fields only as a formatted string.
	if msg := err.Error(); strings.HasPrefix(msg, "json: unknown field ") {
		name := strings.Trim(strings.TrimPrefix(msg, "json: unknown field "), `"`)
		return apperror.InvalidRequestFields("The request contains unknown fields", map[string]string{
			name: "is not a known field",
		}).WithCause(err)
	}
	return apperror.InvalidRequest("The request body could not be decoded").WithCause(err)
}

func jsonTypeName(err *json.UnmarshalTypeError) string {
	switch err.Type.Kind().String() {
	case "string":
		return "string"
	case "bool":
		return "boolean"
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64":
		return "integer"
	case "float32", "float64":
		return "number"
	case "slice", "array":
		return "array"
	case "struct", "map":
		return "object"
	}
	return "valid " + err.Type.String()
}
