package api

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
)

// HandlerFunc is a feature endpoint: typed input in, typed output or an
// application error out. It never touches http.ResponseWriter.
type HandlerFunc[Req any, Res any] func(ctx context.Context, req Req) (Res, error)

// NoContent is the response type for endpoints that answer 204.
type NoContent struct{}

// OptionalBody marks a request struct whose JSON body may be absent. Embed
// it in the request type; the zero struct is passed to the handler when no
// body is sent. Without it a request type with JSON fields requires a body.
type OptionalBody struct{}

// CookieResponse is implemented by response types that need to set cookies
// (login and logout). Handle applies the cookies before writing the body.
type CookieResponse interface {
	ResponseCookies() []*http.Cookie
}

// Option customises Handle.
type Option func(*options)

type options struct {
	bodyLimit int64
}

// WithBodyLimit raises (or lowers) the request body limit for one endpoint.
func WithBodyLimit(maxBytes int64) Option {
	return func(o *options) { o.bodyLimit = maxBytes }
}

// Handle adapts a typed handler to net/http:
//
//	bind path parameters -> bind query parameters -> decode JSON body
//	-> validate -> call handler -> write response or error
//
// The request type must be a struct. Fields tagged `path:"name"` and
// `query:"name"` bind from the route and the query string and must also carry
// `json:"-"`; every other exported field is a JSON body field. Binding
// failures answer 400 invalid_request, validation failures 422
// validation_failed, and application errors whatever they declare. Anything
// else is a 500 whose cause is logged, never returned.
//
// The response is written as JSON with status, except for status 204 which
// writes no body. Response types may implement CookieResponse.
func Handle[Req any, Res any](status int, fn HandlerFunc[Req, Res], opts ...Option) http.HandlerFunc {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	spec := analyzeRequest(reflect.TypeFor[Req]())

	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		var req Req
		if err := bindRequest(w, r, &req, spec, o.bodyLimit); err != nil {
			WriteError(ctx, w, err)
			return
		}
		if err := validateRequest(&req); err != nil {
			WriteError(ctx, w, err)
			return
		}

		res, err := fn(ctx, req)
		if err != nil {
			WriteError(ctx, w, err)
			return
		}

		if cr, ok := any(res).(CookieResponse); ok {
			for _, c := range cr.ResponseCookies() {
				http.SetCookie(w, c)
			}
		}
		if status == http.StatusNoContent {
			WriteNoContent(w)
			return
		}
		WriteJSON(ctx, w, status, res)
	}
}

// requestSpec is the reflection analysis of a request type, computed once
// when the route is registered.
type requestSpec struct {
	pathFields   []boundField
	queryFields  []boundField
	hasBody      bool
	bodyOptional bool
}

type boundField struct {
	name  string
	index []int
	typ   reflect.Type
}

var optionalBodyType = reflect.TypeFor[OptionalBody]()

func analyzeRequest(t reflect.Type) requestSpec {
	if t.Kind() != reflect.Struct {
		panic(fmt.Sprintf("api: request type %s must be a struct", t))
	}
	var spec requestSpec
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous && f.Type == optionalBodyType {
			spec.bodyOptional = true
			continue
		}
		if !f.IsExported() {
			continue
		}
		pathName, hasPath := f.Tag.Lookup("path")
		queryName, hasQuery := f.Tag.Lookup("query")
		jsonTag := f.Tag.Get("json")

		switch {
		case hasPath && hasQuery:
			panic(fmt.Sprintf("api: %s.%s cannot bind from both path and query", t, f.Name))
		case hasPath:
			if jsonTag != "-" {
				panic(fmt.Sprintf("api: %s.%s binds from path and must carry json:\"-\"", t, f.Name))
			}
			if !supportedScalar(f.Type) {
				panic(fmt.Sprintf("api: %s.%s has unsupported path type %s", t, f.Name, f.Type))
			}
			spec.pathFields = append(spec.pathFields, boundField{name: pathName, index: f.Index, typ: f.Type})
		case hasQuery:
			if jsonTag != "-" {
				panic(fmt.Sprintf("api: %s.%s binds from query and must carry json:\"-\"", t, f.Name))
			}
			if !supportedScalar(f.Type) && !supportedSlice(f.Type) {
				panic(fmt.Sprintf("api: %s.%s has unsupported query type %s", t, f.Name, f.Type))
			}
			spec.queryFields = append(spec.queryFields, boundField{name: queryName, index: f.Index, typ: f.Type})
		case jsonTag == "-":
			// Neither bound nor decoded; the handler may fill it itself.
		default:
			spec.hasBody = true
		}
	}
	if spec.bodyOptional && !spec.hasBody {
		panic(fmt.Sprintf("api: %s embeds OptionalBody but has no JSON fields", t))
	}
	return spec
}

func bindRequest(w http.ResponseWriter, r *http.Request, req any, spec requestSpec, bodyLimit int64) error {
	v := reflect.ValueOf(req).Elem()

	fields := map[string]string{}
	bindPath(r, v, spec, fields)
	bindQuery(r, v, spec, fields)
	if len(fields) > 0 {
		return invalidParams(fields)
	}

	if !spec.hasBody {
		return nil
	}
	if bodyLimit <= 0 {
		bodyLimit = defaultBodyLimit(r.Context())
	}
	return decodeBody(w, r, req, bodyLimit, spec.bodyOptional)
}
