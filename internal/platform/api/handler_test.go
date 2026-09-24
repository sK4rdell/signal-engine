package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/sK4rdell/signal-engine/internal/platform/api"
	"github.com/sK4rdell/signal-engine/internal/platform/apperror"
)

type createReq struct {
	Email   string   `json:"email" validate:"required,email,max=254"`
	Name    string   `json:"name" validate:"required,min=2,max=50"`
	Age     int      `json:"age" validate:"omitempty,gte=0,lte=150"`
	Role    string   `json:"role" validate:"omitempty,oneof=owner member"`
	Tags    []string `json:"tags" validate:"omitempty,max=3,dive,min=1"`
	Address *struct {
		City string `json:"city" validate:"required"`
	} `json:"address" validate:"omitempty"`
}

type createRes struct {
	ID    string `json:"id"`
	Email string `json:"email"`
}

type pathReq struct {
	ThingID uuid.UUID `path:"thingID" json:"-" validate:"required"`
	Page    int       `path:"page" json:"-"`
}

type queryReq struct {
	Q      string    `query:"q" json:"-"`
	Limit  int       `query:"limit" json:"-" validate:"omitempty,min=1,max=100"`
	Active bool      `query:"active" json:"-"`
	Ratio  float64   `query:"ratio" json:"-"`
	Count  uint8     `query:"count" json:"-"`
	Owner  uuid.UUID `query:"owner" json:"-"`
	Since  time.Time `query:"since" json:"-"`
	Cursor *string   `query:"cursor" json:"-"`
	Status []string  `query:"status" json:"-" validate:"dive,oneof=new won lost"`
	IDs    []int     `query:"id" json:"-"`
}

type mixedReq struct {
	ThingID uuid.UUID `path:"thingID" json:"-" validate:"required"`
	DryRun  bool      `query:"dry_run" json:"-"`
	Name    string    `json:"name" validate:"required"`
}

// echo types: bound fields carry json:"-" so tests copy them into plain
// response structs to inspect what was bound.
type pathEcho struct {
	ThingID uuid.UUID `json:"thing_id"`
	Page    int       `json:"page"`
}

type queryEcho struct {
	Q      string    `json:"q"`
	Limit  int       `json:"limit"`
	Active bool      `json:"active"`
	Ratio  float64   `json:"ratio"`
	Count  uint8     `json:"count"`
	Owner  uuid.UUID `json:"owner"`
	Since  time.Time `json:"since"`
	Cursor *string   `json:"cursor"`
	Status []string  `json:"status"`
	IDs    []int     `json:"ids"`
}

type mixedEcho struct {
	ThingID uuid.UUID `json:"thing_id"`
	DryRun  bool      `json:"dry_run"`
	Name    string    `json:"name"`
}

type bigReq struct {
	Blob string `json:"blob" validate:"required"`
}

type optionalReq struct {
	api.OptionalBody
	Note string `json:"note"`
}

type noBodyReq struct{}

type cookieRes struct {
	OK bool `json:"ok"`
}

func (cookieRes) ResponseCookies() []*http.Cookie {
	return []*http.Cookie{{Name: "session", Value: "abc", HttpOnly: true}}
}

type cookieOnly struct{}

func (cookieOnly) ResponseCookies() []*http.Cookie {
	return []*http.Cookie{{Name: "session", Value: "", MaxAge: -1}}
}

func newRouter() http.Handler {
	r := api.NewRouter(testRouterConfig(nil))

	r.Post("/things", api.Handle(http.StatusCreated, func(ctx context.Context, req createReq) (createRes, error) {
		return createRes{ID: "1", Email: req.Email}, nil
	}))
	r.Get("/things/{thingID}/pages/{page}", api.Handle(http.StatusOK, func(ctx context.Context, req pathReq) (pathEcho, error) {
		return pathEcho(req), nil
	}))
	r.Get("/search", api.Handle(http.StatusOK, func(ctx context.Context, req queryReq) (queryEcho, error) {
		return queryEcho(req), nil
	}))
	r.Patch("/things/{thingID}", api.Handle(http.StatusOK, func(ctx context.Context, req mixedReq) (mixedEcho, error) {
		return mixedEcho(req), nil
	}))
	r.Post("/optional", api.Handle(http.StatusOK, func(ctx context.Context, req optionalReq) (optionalReq, error) {
		return req, nil
	}))
	r.Post("/no-body", api.Handle(http.StatusNoContent, func(ctx context.Context, req noBodyReq) (api.NoContent, error) {
		return api.NoContent{}, nil
	}))
	r.Post("/app-error", api.Handle(http.StatusOK, func(ctx context.Context, req noBodyReq) (api.NoContent, error) {
		return api.NoContent{}, apperror.NotFound("thing_not_found", "Thing not found").WithCause(errors.New("no rows"))
	}))
	r.Post("/wrapped-app-error", api.Handle(http.StatusOK, func(ctx context.Context, req noBodyReq) (api.NoContent, error) {
		return api.NoContent{}, errors.Join(errors.New("context"), apperror.Conflict("thing_exists", "Thing exists"))
	}))
	r.Post("/unexpected", api.Handle(http.StatusOK, func(ctx context.Context, req noBodyReq) (api.NoContent, error) {
		return api.NoContent{}, errors.New("pq: connection refused to 10.0.0.1")
	}))
	r.Post("/cancelled", api.Handle(http.StatusOK, func(ctx context.Context, req noBodyReq) (api.NoContent, error) {
		return api.NoContent{}, context.Canceled
	}))
	r.Post("/login", api.Handle(http.StatusOK, func(ctx context.Context, req noBodyReq) (cookieRes, error) {
		return cookieRes{OK: true}, nil
	}))
	r.Post("/logout", api.Handle(http.StatusNoContent, func(ctx context.Context, req noBodyReq) (cookieOnly, error) {
		return cookieOnly{}, nil
	}))
	r.Post("/request-id", api.Handle(http.StatusOK, func(ctx context.Context, req noBodyReq) (map[string]string, error) {
		return map[string]string{"request_id": api.RequestIDFromContext(ctx)}, nil
	}))
	r.Post("/unencodable", api.Handle(http.StatusOK, func(ctx context.Context, req noBodyReq) (map[string]any, error) {
		return map[string]any{"ch": make(chan int)}, nil
	}))
	return r
}

func do(t *testing.T, h http.Handler, method, target string, body string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func assertError(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) api.ErrorResponse {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", rec.Code, status, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	body := decodeError(t, rec)
	if body.Error.Code != code {
		t.Errorf("code = %q, want %q (body %s)", body.Error.Code, code, rec.Body.String())
	}
	return body
}

// --- JSON body -------------------------------------------------------------

func TestHandle_ValidJSON(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/things", `{"email":"a@example.com","name":"Ann","tags":["x"]}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body %s", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var res createRes
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.ID != "1" || res.Email != "a@example.com" {
		t.Errorf("response = %+v", res)
	}
}

func TestHandle_MalformedJSON(t *testing.T) {
	for _, body := range []string{`{"email":`, `{"email" "x"}`, `{`, `{"email":"a@b.c",}`} {
		rec := do(t, newRouter(), http.MethodPost, "/things", body)
		assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	}
}

func TestHandle_RejectsUnknownJSONField(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/things", `{"emial":"a@example.com","name":"Ann"}`)
	body := assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	if body.Error.Fields["emial"] == "" {
		t.Errorf("unknown field should be named, got %v", body.Error.Fields)
	}
}

func TestHandle_RejectsMultipleJSONDocuments(t *testing.T) {
	for _, body := range []string{
		`{"email":"a@example.com","name":"Ann"}{"email":"b@example.com","name":"Bob"}`,
		`{"email":"a@example.com","name":"Ann"} trailing`,
		`{"email":"a@example.com","name":"Ann"} 42`,
	} {
		rec := do(t, newRouter(), http.MethodPost, "/things", body)
		assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	}
}

func TestHandle_RejectsNonObjectBody(t *testing.T) {
	for _, body := range []string{`[]`, `"str"`, `null`, `42`} {
		rec := do(t, newRouter(), http.MethodPost, "/things", body)
		assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	}
}

func TestHandle_RejectsWrongFieldType(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/things", `{"email":"a@example.com","name":"Ann","age":"old"}`)
	body := assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	if body.Error.Fields["age"] != "must be a integer" && body.Error.Fields["age"] != "must be an integer" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestHandle_RequiredBodyMissing(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/things", "")
	assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)

	req := httptest.NewRequest(http.MethodPost, "/things", strings.NewReader("   \n"))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	newRouter().ServeHTTP(rec, req)
	assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
}

func TestHandle_OptionalBody(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/optional", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("empty optional body: status %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, newRouter(), http.MethodPost, "/optional", `{"note":"hi"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"note":"hi"`) {
		t.Fatalf("optional body present: status %d %s", rec.Code, rec.Body.String())
	}
	rec = do(t, newRouter(), http.MethodPost, "/optional", `{"nope":1}`)
	assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
}

func TestHandle_NoBodyEndpointIgnoresBody(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/no-body", "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status %d body %q", rec.Code, rec.Body.String())
	}
}

func TestHandle_RequiresJSONContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/things", strings.NewReader(`{"email":"a@example.com","name":"Ann"}`))
	rec := httptest.NewRecorder()
	newRouter().ServeHTTP(rec, req)
	assertError(t, rec, http.StatusUnsupportedMediaType, api.CodeUnsupportedMediaType)

	req = httptest.NewRequest(http.MethodPost, "/things", strings.NewReader(`{"email":"a@example.com","name":"Ann"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec = httptest.NewRecorder()
	newRouter().ServeHTTP(rec, req)
	assertError(t, rec, http.StatusUnsupportedMediaType, api.CodeUnsupportedMediaType)

	req = httptest.NewRequest(http.MethodPost, "/things", strings.NewReader(`{"email":"a@example.com","name":"Ann"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	rec = httptest.NewRecorder()
	newRouter().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("charset parameter should be accepted, got %d", rec.Code)
	}
}

func TestHandle_OversizedBody(t *testing.T) {
	cfg := testRouterConfig(nil)
	cfg.HTTP.MaxBodyBytes = 64
	r := api.NewRouter(cfg)
	r.Post("/things", api.Handle(http.StatusCreated, func(ctx context.Context, req createReq) (createRes, error) {
		return createRes{}, nil
	}))
	r.Post("/big", api.Handle(http.StatusCreated, func(ctx context.Context, req bigReq) (createRes, error) {
		return createRes{ID: "big"}, nil
	}, api.WithBodyLimit(1<<20)))

	big := `{"blob":"` + strings.Repeat("x", 100) + `"}`
	rec := do(t, r, http.MethodPost, "/things", big)
	assertError(t, rec, http.StatusRequestEntityTooLarge, apperror.CodeRequestTooLarge)

	small := `{"email":"a@example.com","name":"Ann"}`
	if rec := do(t, r, http.MethodPost, "/things", small); rec.Code != http.StatusCreated {
		t.Fatalf("small body: status %d", rec.Code)
	}
	if rec := do(t, r, http.MethodPost, "/big", big); rec.Code != http.StatusCreated {
		t.Fatalf("WithBodyLimit should raise the limit, got %d %s", rec.Code, rec.Body.String())
	}
}

func TestHandle_FallbackBodyLimitWithoutRouter(t *testing.T) {
	h := api.Handle(http.StatusOK, func(ctx context.Context, req createReq) (createRes, error) { return createRes{}, nil })
	big := `{"email":"a@example.com","name":"` + strings.Repeat("x", 2<<20) + `"}`
	rec := do(t, h, http.MethodPost, "/", big)
	assertError(t, rec, http.StatusRequestEntityTooLarge, apperror.CodeRequestTooLarge)
}

// --- validation ------------------------------------------------------------

func TestHandle_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		body    string
		field   string
		wantMsg string
	}{
		{"required", `{"name":"Ann"}`, "email", "is required"},
		{"email", `{"email":"nope","name":"Ann"}`, "email", "must be a valid email address"},
		{"min length", `{"email":"a@example.com","name":"A"}`, "name", "must contain at least 2 characters"},
		{"max length", `{"email":"a@example.com","name":"` + strings.Repeat("n", 51) + `"}`, "name", "must contain at most 50 characters"},
		{"range", `{"email":"a@example.com","name":"Ann","age":200}`, "age", "must be at most 150"},
		{"enum", `{"email":"a@example.com","name":"Ann","role":"admin"}`, "role", "must be one of: owner, member"},
		{"slice max", `{"email":"a@example.com","name":"Ann","tags":["a","b","c","d"]}`, "tags", "must contain at most 3 items"},
		{"slice element", `{"email":"a@example.com","name":"Ann","tags":[""]}`, "tags[0]", "must contain at least 1 characters"},
		{"nested", `{"email":"a@example.com","name":"Ann","address":{}}`, "address.city", "is required"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := do(t, newRouter(), http.MethodPost, "/things", tc.body)
			body := assertError(t, rec, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
			if got := body.Error.Fields[tc.field]; got != tc.wantMsg {
				t.Errorf("fields[%s] = %q, want %q (all: %v)", tc.field, got, tc.wantMsg, body.Error.Fields)
			}
		})
	}
}

func TestHandle_ValidationReportsAllFields(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/things", `{"email":"bad","name":"A","role":"x"}`)
	body := assertError(t, rec, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
	if len(body.Error.Fields) != 3 {
		t.Errorf("fields = %v, want 3 entries", body.Error.Fields)
	}
	if strings.Contains(rec.Body.String(), "Field validation") {
		t.Error("validator internals leaked")
	}
}

// --- path binding ----------------------------------------------------------

func TestHandle_PathBinding(t *testing.T) {
	id := uuid.New()
	rec := do(t, newRouter(), http.MethodGet, "/things/"+id.String()+"/pages/7", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var res pathEcho
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.ThingID != id || res.Page != 7 {
		t.Errorf("bound = %+v", res)
	}
}

func TestHandle_PathBindingRejectsMalformedValues(t *testing.T) {
	rec := do(t, newRouter(), http.MethodGet, "/things/not-a-uuid/pages/7", "")
	body := assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	if body.Error.Fields["thingID"] != "must be a valid UUID" {
		t.Errorf("fields = %v", body.Error.Fields)
	}

	rec = do(t, newRouter(), http.MethodGet, "/things/"+uuid.NewString()+"/pages/seven", "")
	body = assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	if body.Error.Fields["page"] != "must be an integer" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestHandle_PathParameterMissingFromRoute(t *testing.T) {
	r := api.NewRouter(testRouterConfig(nil))
	r.Get("/wrong", api.Handle(http.StatusOK, func(ctx context.Context, req pathReq) (pathReq, error) { return req, nil }))
	rec := do(t, r, http.MethodGet, "/wrong", "")
	body := assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
	if body.Error.Fields["thingID"] != "is required" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

// --- query binding ---------------------------------------------------------

func TestHandle_QueryBinding(t *testing.T) {
	owner := uuid.New()
	target := "/search?q=hello&limit=20&active=true&ratio=0.5&count=200&owner=" + owner.String() +
		"&since=2026-01-02T03:04:05Z&cursor=abc&status=new&status=won&id=1&id=2"
	rec := do(t, newRouter(), http.MethodGet, target, "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var res queryEcho
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.Q != "hello" || res.Limit != 20 || !res.Active || res.Ratio != 0.5 || res.Count != 200 || res.Owner != owner {
		t.Errorf("scalars = %+v", res)
	}
	if !res.Since.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("Since = %v", res.Since)
	}
	if res.Cursor == nil || *res.Cursor != "abc" {
		t.Errorf("Cursor = %v", res.Cursor)
	}
	if len(res.Status) != 2 || res.Status[1] != "won" || len(res.IDs) != 2 || res.IDs[1] != 2 {
		t.Errorf("slices = %v %v", res.Status, res.IDs)
	}
}

func TestHandle_QueryBindingAbsentValuesStayZero(t *testing.T) {
	rec := do(t, newRouter(), http.MethodGet, "/search", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"cursor":null`) {
		t.Errorf("absent pointer should stay nil: %s", rec.Body.String())
	}
}

func TestHandle_QueryBindingRejectsMalformedValues(t *testing.T) {
	tests := []struct {
		query string
		field string
		msg   string
	}{
		{"limit=ten", "limit", "must be an integer"},
		{"active=yes", "active", "must be true or false"},
		{"ratio=x", "ratio", "must be a number"},
		{"count=300", "count", "must be a non-negative integer"},
		{"count=-1", "count", "must be a non-negative integer"},
		{"owner=abc", "owner", "must be a valid UUID"},
		{"since=yesterday", "since", "must be an RFC 3339 timestamp"},
		{"id=1&id=x", "id", "must be an integer"},
		{"limit=1&limit=2", "limit", "must not be repeated"},
	}
	for _, tc := range tests {
		t.Run(tc.query, func(t *testing.T) {
			rec := do(t, newRouter(), http.MethodGet, "/search?"+tc.query, "")
			body := assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
			if body.Error.Fields[tc.field] != tc.msg {
				t.Errorf("fields = %v, want %s=%q", body.Error.Fields, tc.field, tc.msg)
			}
		})
	}
}

func TestHandle_QueryValuesAreValidated(t *testing.T) {
	rec := do(t, newRouter(), http.MethodGet, "/search?limit=500", "")
	body := assertError(t, rec, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
	if body.Error.Fields["limit"] != "must be at most 100" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
	rec = do(t, newRouter(), http.MethodGet, "/search?status=bogus", "")
	body = assertError(t, rec, http.StatusUnprocessableEntity, apperror.CodeValidationFailed)
	if body.Error.Fields["status[0]"] == "" {
		t.Errorf("fields = %v", body.Error.Fields)
	}
}

func TestHandle_MixedBinding(t *testing.T) {
	id := uuid.New()
	rec := do(t, newRouter(), http.MethodPatch, "/things/"+id.String()+"?dry_run=true", `{"name":"new"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d %s", rec.Code, rec.Body.String())
	}
	var res mixedEcho
	if err := json.Unmarshal(rec.Body.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if res.ThingID != id || !res.DryRun || res.Name != "new" {
		t.Errorf("bound = %+v", res)
	}

	// Path and query fields are not accepted from the body.
	rec = do(t, newRouter(), http.MethodPatch, "/things/"+id.String(), `{"name":"new","ThingID":"x"}`)
	assertError(t, rec, http.StatusBadRequest, apperror.CodeInvalidRequest)
}

// --- handler execution and responses --------------------------------------

func TestHandle_ApplicationError(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/app-error", "")
	body := assertError(t, rec, http.StatusNotFound, "thing_not_found")
	if body.Error.Message != "Thing not found" {
		t.Errorf("message = %q", body.Error.Message)
	}
	if strings.Contains(rec.Body.String(), "no rows") {
		t.Error("cause leaked")
	}
}

func TestHandle_WrappedApplicationError(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/wrapped-app-error", "")
	assertError(t, rec, http.StatusConflict, "thing_exists")
}

func TestHandle_UnexpectedErrorDoesNotLeak(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/unexpected", "")
	body := assertError(t, rec, http.StatusInternalServerError, apperror.CodeInternal)
	if strings.Contains(rec.Body.String(), "connection refused") || strings.Contains(rec.Body.String(), "10.0.0.1") {
		t.Error("internal error leaked")
	}
	if body.Error.Message != "An unexpected error occurred" {
		t.Errorf("message = %q", body.Error.Message)
	}
}

func TestHandle_CancelledContext(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/cancelled", "")
	if rec.Code != 499 {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestHandle_NoContentWritesNoBody(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/no-body", "")
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q", rec.Body.String())
	}
}

func TestHandle_ResponseCookies(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/login", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "session" || cookies[0].Value != "abc" || !cookies[0].HttpOnly {
		t.Errorf("cookies = %v", cookies)
	}
	if !strings.Contains(rec.Body.String(), `"ok":true`) {
		t.Errorf("body = %s", rec.Body.String())
	}

	rec = do(t, newRouter(), http.MethodPost, "/logout", "")
	if rec.Code != http.StatusNoContent || rec.Body.Len() != 0 {
		t.Fatalf("status = %d body %q", rec.Code, rec.Body.String())
	}
	cookies = rec.Result().Cookies()
	if len(cookies) != 1 || cookies[0].MaxAge != -1 {
		t.Errorf("cookies = %v", cookies)
	}
}

func TestHandle_RequestIDAvailableToHandlerAndResponse(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/request-id", "", "X-Request-ID", "trace-42")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"request_id":"trace-42"`) {
		t.Errorf("handler did not see request id: %s", rec.Body.String())
	}
	if rec.Header().Get("X-Request-ID") != "trace-42" {
		t.Errorf("response header = %q", rec.Header().Get("X-Request-ID"))
	}

	rec = do(t, newRouter(), http.MethodPost, "/things", `{`)
	if rec.Header().Get("X-Request-ID") == "" {
		t.Error("error responses must carry the request ID too")
	}
}

func TestHandle_UnencodableResponseIsInternalError(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/unencodable", "")
	assertError(t, rec, http.StatusInternalServerError, apperror.CodeInternal)
}

func TestHandle_ResponseIsJSONWithSnakeCaseFromTags(t *testing.T) {
	rec := do(t, newRouter(), http.MethodPost, "/things", `{"email":"a@example.com","name":"Ann"}`)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["email"]; !ok {
		t.Errorf("expected snake_case tag names, got %s", rec.Body.String())
	}
	if !bytes.HasSuffix(rec.Body.Bytes(), []byte("\n")) {
		t.Error("expected trailing newline from encoder")
	}
}

// --- registration-time checks ---------------------------------------------

func TestHandle_PanicsOnInvalidRequestTypes(t *testing.T) {
	assertPanics := func(name string, fn func()) {
		t.Helper()
		defer func() {
			if recover() == nil {
				t.Errorf("%s: expected panic at registration", name)
			}
		}()
		fn()
	}
	assertPanics("non-struct", func() {
		api.Handle(http.StatusOK, func(ctx context.Context, req string) (string, error) { return "", nil })
	})
	assertPanics("path without json:-", func() {
		type bad struct {
			ID uuid.UUID `path:"id"`
		}
		api.Handle(http.StatusOK, func(ctx context.Context, req bad) (bad, error) { return req, nil })
	})
	assertPanics("unsupported query type", func() {
		type bad struct {
			M map[string]string `query:"m" json:"-"`
		}
		api.Handle(http.StatusOK, func(ctx context.Context, req bad) (bad, error) { return req, nil })
	})
	assertPanics("optional body without fields", func() {
		type bad struct {
			api.OptionalBody
		}
		api.Handle(http.StatusOK, func(ctx context.Context, req bad) (bad, error) { return req, nil })
	})
}
