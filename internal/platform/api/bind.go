package api

import (
	"encoding"
	"fmt"
	"net/http"
	"reflect"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
)

var (
	uuidType            = reflect.TypeFor[uuid.UUID]()
	timeType            = reflect.TypeFor[time.Time]()
	textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()
)

func invalidParams(fields map[string]string) error {
	return apperror.InvalidRequestFields("The request contains invalid parameters", fields)
}

func bindPath(r *http.Request, v reflect.Value, spec requestSpec, fields map[string]string) {
	for _, f := range spec.pathFields {
		raw := chi.URLParam(r, f.name)
		if raw == "" {
			// Only possible when the route pattern lacks the parameter.
			fields[f.name] = "is required"
			continue
		}
		if err := setScalar(v.FieldByIndex(f.index), raw); err != nil {
			fields[f.name] = err.Error()
		}
	}
}

func bindQuery(r *http.Request, v reflect.Value, spec requestSpec, fields map[string]string) {
	query := r.URL.Query()
	for _, f := range spec.queryFields {
		values, present := query[f.name]
		if !present {
			continue
		}
		target := v.FieldByIndex(f.index)
		if f.typ.Kind() == reflect.Slice && f.typ != uuidType {
			slice := reflect.MakeSlice(f.typ, 0, len(values))
			var failed bool
			for _, raw := range values {
				elem := reflect.New(f.typ.Elem()).Elem()
				if err := setScalar(elem, raw); err != nil {
					fields[f.name] = err.Error()
					failed = true
					break
				}
				slice = reflect.Append(slice, elem)
			}
			if !failed {
				target.Set(slice)
			}
			continue
		}
		if len(values) > 1 {
			fields[f.name] = "must not be repeated"
			continue
		}
		if err := setScalar(target, values[0]); err != nil {
			fields[f.name] = err.Error()
		}
	}
}

// supportedScalar reports whether t can be bound from a single string.
func supportedScalar(t reflect.Type) bool {
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	if t == uuidType || t == timeType {
		return true
	}
	if reflect.PointerTo(t).Implements(textUnmarshalerType) {
		return true
	}
	switch t.Kind() {
	case reflect.String, reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	}
	return false
}

func supportedSlice(t reflect.Type) bool {
	return t.Kind() == reflect.Slice && t != uuidType && supportedScalar(t.Elem())
}

// setScalar parses raw into v. Errors are client-facing messages.
func setScalar(v reflect.Value, raw string) error {
	if v.Kind() == reflect.Pointer {
		elem := reflect.New(v.Type().Elem())
		if err := setScalar(elem.Elem(), raw); err != nil {
			return err
		}
		v.Set(elem)
		return nil
	}

	switch v.Type() {
	case uuidType:
		id, err := uuid.Parse(raw)
		if err != nil {
			return fmt.Errorf("must be a valid UUID")
		}
		v.Set(reflect.ValueOf(id))
		return nil
	case timeType:
		ts, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return fmt.Errorf("must be an RFC 3339 timestamp")
		}
		v.Set(reflect.ValueOf(ts))
		return nil
	}

	if v.CanAddr() {
		if u, ok := v.Addr().Interface().(encoding.TextUnmarshaler); ok {
			if err := u.UnmarshalText([]byte(raw)); err != nil {
				return fmt.Errorf("is invalid")
			}
			return nil
		}
	}

	switch v.Kind() {
	case reflect.String:
		v.SetString(raw)
	case reflect.Bool:
		b, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("must be true or false")
		}
		v.SetBool(b)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		n, err := strconv.ParseInt(raw, 10, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("must be an integer")
		}
		v.SetInt(n)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		n, err := strconv.ParseUint(raw, 10, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("must be a non-negative integer")
		}
		v.SetUint(n)
	case reflect.Float32, reflect.Float64:
		f, err := strconv.ParseFloat(raw, v.Type().Bits())
		if err != nil {
			return fmt.Errorf("must be a number")
		}
		v.SetFloat(f)
	default:
		return fmt.Errorf("is not supported")
	}
	return nil
}
