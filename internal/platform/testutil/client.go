package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"betemplate/internal/platform/api"
)

// Client sends requests through the router and keeps cookies between
// requests like a browser would.
type Client struct {
	app *App

	// Origin is sent as the Origin header. Set it to "" to omit the header
	// or to another value to simulate a cross-site request.
	Origin string
	// RemoteAddr is the peer address, used by rate limiting and logging.
	RemoteAddr string
	// Headers are added to every request.
	Headers http.Header
	// SessionID is set by AuthenticatedClient.
	SessionID uuid.UUID

	cookies map[string]*http.Cookie
}

// SetCookie stores a cookie for subsequent requests.
func (c *Client) SetCookie(cookie *http.Cookie) {
	if cookie.MaxAge < 0 {
		delete(c.cookies, cookie.Name)
		return
	}
	c.cookies[cookie.Name] = cookie
}

// Cookie returns a stored cookie by name.
func (c *Client) Cookie(name string) (*http.Cookie, bool) {
	cookie, ok := c.cookies[name]
	return cookie, ok
}

// Get sends a GET request.
func (c *Client) Get(path string) *Response { return c.Do(http.MethodGet, path, nil) }

// Delete sends a DELETE request.
func (c *Client) Delete(path string) *Response { return c.Do(http.MethodDelete, path, nil) }

// PostJSON sends a POST with a JSON body. A nil body sends no body.
func (c *Client) PostJSON(path string, body any) *Response {
	return c.Do(http.MethodPost, path, body)
}

// PatchJSON sends a PATCH with a JSON body.
func (c *Client) PatchJSON(path string, body any) *Response {
	return c.Do(http.MethodPatch, path, body)
}

// Do sends a request. body may be nil, a string or []byte (sent verbatim
// as application/json), or any value encoded as JSON.
func (c *Client) Do(method, path string, body any) *Response {
	c.app.t.Helper()

	var reader io.Reader
	hasBody := false
	switch b := body.(type) {
	case nil:
	case string:
		reader, hasBody = bytes.NewBufferString(b), true
	case []byte:
		reader, hasBody = bytes.NewBuffer(b), true
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			c.app.t.Fatalf("testutil: encode request body: %v", err)
		}
		reader, hasBody = bytes.NewBuffer(raw), true
	}

	req := httptest.NewRequestWithContext(context.Background(), method, path, reader)
	req.RemoteAddr = c.RemoteAddr
	if hasBody {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.Origin != "" {
		req.Header.Set("Origin", c.Origin)
	}
	for k, vs := range c.Headers {
		for _, v := range vs {
			req.Header.Add(k, v)
		}
	}
	for _, cookie := range c.cookies {
		req.AddCookie(cookie)
	}

	rec := httptest.NewRecorder()
	c.app.Router.ServeHTTP(rec, req)
	result := rec.Result()
	for _, cookie := range result.Cookies() {
		c.SetCookie(cookie)
	}
	return &Response{
		t:          c.app.t,
		StatusCode: rec.Code,
		Header:     rec.Header(),
		Body:       rec.Body.Bytes(),
		Cookies:    result.Cookies(),
	}
}

// Response is a recorded HTTP response with assertion helpers.
type Response struct {
	t          testing.TB
	StatusCode int
	Header     http.Header
	Body       []byte
	Cookies    []*http.Cookie
}

// AssertStatus fails the test when the status differs.
func (r *Response) AssertStatus(t testing.TB, want int) *Response {
	t.Helper()
	if r.StatusCode != want {
		t.Fatalf("status = %d, want %d\nbody: %s", r.StatusCode, want, r.Body)
	}
	return r
}

// DecodeJSON decodes the body into v.
func (r *Response) DecodeJSON(t testing.TB, v any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		t.Fatalf("decode response body: %v\nbody: %s", err, r.Body)
	}
}

// Error decodes the standard error body.
func (r *Response) Error(t testing.TB) api.ErrorBody {
	t.Helper()
	var body api.ErrorResponse
	r.DecodeJSON(t, &body)
	return body.Error
}

// AssertError asserts the status and the public error code.
func (r *Response) AssertError(t testing.TB, status int, code string) api.ErrorBody {
	t.Helper()
	r.AssertStatus(t, status)
	e := r.Error(t)
	if e.Code != code {
		t.Fatalf("error code = %q, want %q\nbody: %s", e.Code, code, r.Body)
	}
	return e
}

// Cookie returns a response cookie by name.
func (r *Response) Cookie(name string) *http.Cookie {
	for _, c := range r.Cookies {
		if c.Name == name {
			return c
		}
	}
	return nil
}
