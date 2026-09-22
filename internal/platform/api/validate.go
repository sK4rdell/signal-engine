package api

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"

	"github.com/go-playground/validator/v10"

	"betemplate/internal/platform/apperror"
)

var (
	validateOnce sync.Once
	validate     *validator.Validate
)

// Validator returns the shared validator. Field names in errors come from
// json, path or query tags so messages match what clients sent.
func Validator() *validator.Validate {
	validateOnce.Do(func() {
		validate = validator.New(validator.WithRequiredStructEnabled())
		validate.RegisterTagNameFunc(func(f reflect.StructField) string {
			for _, tag := range []string{"json", "path", "query"} {
				name := strings.SplitN(f.Tag.Get(tag), ",", 2)[0]
				if name != "" && name != "-" {
					return name
				}
			}
			return f.Name
		})
	})
	return validate
}

// validateRequest runs structural validation and converts failures to a
// 422 with one message per field.
func validateRequest(req any) error {
	err := Validator().Struct(req)
	if err == nil {
		return nil
	}
	var verrs validator.ValidationErrors
	if !asValidationErrors(err, &verrs) {
		return apperror.Internal(fmt.Errorf("validate request: %w", err))
	}
	fields := make(map[string]string, len(verrs))
	for _, fe := range verrs {
		name := fieldPath(fe)
		if _, exists := fields[name]; !exists {
			fields[name] = fieldMessage(fe)
		}
	}
	return apperror.Validation(fields)
}

func asValidationErrors(err error, target *validator.ValidationErrors) bool {
	return errors.As(err, target)
}

// fieldPath strips the root struct name from the namespace: "Req.address.city"
// becomes "address.city".
func fieldPath(fe validator.FieldError) string {
	ns := fe.Namespace()
	if i := strings.Index(ns, "."); i >= 0 {
		return ns[i+1:]
	}
	return fe.Field()
}

func fieldMessage(fe validator.FieldError) string {
	kind := fe.Kind()
	if kind == reflect.Pointer {
		kind = fe.Type().Elem().Kind()
	}
	unit := func(n string) string {
		switch kind {
		case reflect.String:
			return n + " characters"
		case reflect.Slice, reflect.Array, reflect.Map:
			return n + " items"
		}
		return n
	}

	switch fe.Tag() {
	case "required", "required_if", "required_unless", "required_with", "required_without":
		return "is required"
	case "email":
		return "must be a valid email address"
	case "uuid", "uuid4":
		return "must be a valid UUID"
	case "url", "http_url":
		return "must be a valid URL"
	case "min":
		if kind == reflect.String || kind == reflect.Slice || kind == reflect.Array || kind == reflect.Map {
			return "must contain at least " + unit(fe.Param())
		}
		return "must be at least " + fe.Param()
	case "max":
		if kind == reflect.String || kind == reflect.Slice || kind == reflect.Array || kind == reflect.Map {
			return "must contain at most " + unit(fe.Param())
		}
		return "must be at most " + fe.Param()
	case "len":
		return "must contain exactly " + unit(fe.Param())
	case "gte":
		return "must be at least " + fe.Param()
	case "lte":
		return "must be at most " + fe.Param()
	case "gt":
		return "must be greater than " + fe.Param()
	case "lt":
		return "must be less than " + fe.Param()
	case "oneof":
		return "must be one of: " + strings.Join(strings.Fields(fe.Param()), ", ")
	case "eqfield":
		return "must match " + strings.ToLower(fe.Param())
	case "nefield":
		return "must differ from " + strings.ToLower(fe.Param())
	case "alphanum":
		return "must contain only letters and digits"
	case "boolean":
		return "must be true or false"
	}
	return "is invalid"
}
