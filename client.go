// Package httptestclient generates and executes standard http requests. It follows the basic builder pattern
// to build and/or execute the request against a httptest.Server
// Any errors that occur during the build or execute of the request will fail the test so that you can focus on your
// server not on the http client
package httptestclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"

	"github.com/NearlyUnique/httptestclient/internal/self"
)

// UserAgent default value
const UserAgent = "test-http-request"

// ContentTypeApplicationJson for http header Content-Type
const ContentTypeApplicationJson = "application/json"

var (
	// DefaultContentType when content is detected
	DefaultContentType = ContentTypeApplicationJson
)

// TestingT allows testing of this test client use *testing.T
type TestingT interface {
	// Errorf as per testing.T
	Errorf(format string, args ...interface{})
	// FailNow as per testing.T
	FailNow()
}

// testingHooks testing clients do not need to expose this
type testingHooks interface {
	Helper()
	Cleanup(func())
	Failed() bool
}

// ErrNilBodyJSON sentinel error
var ErrNilBodyJSON = errors.New("BodyJson requires non nil value")

// SimpleResponse simplified status response rather than using the http.Response directly
type SimpleResponse struct {
	Header http.Header
	Body   string
	Status int

	t TestingT
}

// BodyJSON uses json.Unmarshal to map the Body to the struct
func (r SimpleResponse) BodyJSON(payload interface{}) {
	err := json.Unmarshal([]byte(r.Body), payload)
	if err != nil {
		r.t.Errorf("unmarshal payload failed: %v", err)
		r.t.FailNow()
	}
}

// Client simplifies creating test client http request
type Client struct {
	t TestingT

	method             string
	url                string
	header             http.Header
	body               io.Reader
	form               url.Values
	context            context.Context
	expectedStatus     int
	err                error
	expectRedirectPath string
}

// New for testing, finish with Client.Do or Client.DoSimple
// `t` use `*testing.T`
// `s` server under test
// example:
//
//	  resp2 := New(t).
//			Post("/some-request/%s/resource", id).
//			Header("special-header", "magic").
//			BodyString(`{"token":"opaque-string"}`).
//			Do(server)
func New(t TestingT) *Client {
	if h, ok := t.(testingHooks); ok {
		h.Helper()
	}

	h := make(http.Header)
	h.Set("Accept", "application/json")
	h.Set("User-Agent", UserAgent)

	return &Client{
		t:       t,
		context: context.Background(),
		method:  http.MethodGet,
		url:     "/",
		header:  h,
	}
}

// Context when sending the request, defaults to `context.Context()` if not set
func (c *Client) Context(ctx context.Context) *Client {
	c.context = ctx
	return c
}

// ExpectedStatusCode for the test to pass. By default, any 2xx will pass otherwise explicitly state the success status
// do not use this to expect redirects, see ExpectRedirectTo
func (c *Client) ExpectedStatusCode(status int) *Client {
	if status >= 300 && status < 400 {
		c.failNow("misuse of ExpectedStatusCode(%d), use ExpectRedirectTo instead", status)
		return c
	}
	c.expectedStatus = status
	return c
}

// ExpectRedirectTo a specific url
func (c *Client) ExpectRedirectTo(path string) *Client {
	c.expectRedirectPath = path
	return c
}

// Method to use in request, the default is GET
func (c *Client) Method(method string) *Client {
	c.method = method
	return c
}

// URL adds a url using standard formatting as per fmt.Sprintf, default '/'
func (c *Client) URL(url string, args ...interface{}) *Client {
	c.url = fmt.Sprintf(url, args...)
	return c
}

// Post is shorthand for
//
//	testClient.Method("POST").URL(...)
func (c *Client) Post(url string, args ...interface{}) *Client {
	return c.Method(http.MethodPost).URL(url, args...)
}

// Put is shorthand for
//
//	testClient.Method("PUT").URL(...)
func (c *Client) Put(url string, args ...interface{}) *Client {
	return c.Method(http.MethodPut).URL(url, args...)
}

// Patch is shorthand for
//
//	testClient.Method("PATCH").URL(...)
func (c *Client) Patch(url string, args ...interface{}) *Client {
	return c.Method(http.MethodPatch).URL(url, args...)
}

// Get is shorthand for
//
//	testClient.Method("GET").URL(...)
func (c *Client) Get(url string, args ...interface{}) *Client {
	return c.Method(http.MethodGet).URL(url, args...)
}

// Delete is shorthand for
//
//	testClient.Method("DELETE").URL(...)
func (c *Client) Delete(url string, args ...interface{}) *Client {
	return c.Method(http.MethodDelete).URL(url, args...)
}

// Header for request, can be called multiple times and is additive unless using the same header name, then it overwrites
func (c *Client) Header(name, value string, moreValues ...string) *Client {
	c.header.Set(name, value)
	for _, v := range moreValues {
		c.header.Add(name, v)
	}
	return c
}

// FormData for posting x-www-form-urlencoded forms
// args is expected to be pairs of key:values
func (c *Client) FormData(args ...string) *Client {
	if len(args)%2 != 0 {
		c.failNow("Incorrect number of parameters %d items, missed pair", len(args))
	}
	if c.form == nil {
		c.form = url.Values{}
	}
	for i := 0; i < len(args); i += 2 {
		c.form.Add(args[i], args[i+1])
	}
	// re-encode fom as body
	c.body = strings.NewReader(c.form.Encode())
	return c
}

// ClearHeaders removes default http headers, Accept, Content-Type, User-Agent. Must be called before adding other headers
func (c *Client) ClearHeaders() *Client {
	c.header = make(http.Header)
	return c
}

// BodyBytes to send
func (c *Client) BodyBytes(body []byte) *Client {
	c.body = bytes.NewReader(body)
	return c
}

// BodyJSON convert struct to son using json.Marshal
func (c *Client) BodyJSON(payload interface{}) *Client {
	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}
	if payload == nil {
		c.failNow("payload to send is nil")
		c.err = ErrNilBodyJSON
		return c
	}
	buf, err := json.Marshal(payload)
	if c.hasError(err) {
		return c
	}
	return c.BodyBytes(buf)
}

// BodyString is a literal string version of the body
// if the body has a value the content-type will be 'application/json' will be added
// unless you set an alternative or use ClearHeaders()
func (c *Client) BodyString(body string) *Client {
	return c.BodyBytes([]byte(body))
}

// BuildRequest a raw unsent request
func (c *Client) BuildRequest() *http.Request {
	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}
	return c.buildRequest("")
}

func (c *Client) buildRequest(baseURL string) *http.Request {
	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}
	if c.err != nil {
		return nil
	}
	urlPath := joinPath(baseURL, c.url)
	if len(c.form) > 0 && c.method == "" {
		c.Method(http.MethodPost)
	}
	req, err := http.NewRequestWithContext(c.context, c.method, urlPath, c.body)
	if c.hasError(err) {
		return nil
	}
	req.Header = c.header
	if len(c.form) > 0 {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else if c.body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", DefaultContentType)
	}
	return req
}

// Do the http request, http status must either match expected or be success
func (c *Client) Do(server *httptest.Server) *http.Response {

	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}

	req := c.buildRequest(server.URL)
	if req == nil {
		return nil
	}
	client := server.Client()
	wasRedirected := false
	if client.CheckRedirect == nil {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			fmt.Println("Redirected to:", req.URL)
			if req.URL.Path != c.expectRedirectPath {
				c.failNow("expected to redirect path '%s', actual path '%s'", c.expectRedirectPath, req.URL.Path)
				return fmt.Errorf("expected to redirect path '%s', actual path '%s'", c.expectRedirectPath, req.URL.Path)
			}
			wasRedirected = true
			return nil
		}
	}
	resp, err := client.Do(req)
	if c.hasError(err) {
		return nil
	}
	if c.expectRedirectPath != "" && !wasRedirected {
		c.failNow("expected to redirect path '%s' but no redirection happened", c.expectRedirectPath)
		return nil
	}
	if c.expectedStatus == 0 && resp.StatusCode >= 400 {
		c.failNow("expected success, got %d", resp.StatusCode)
		return nil
	} else if c.expectedStatus > 0 && c.expectedStatus != resp.StatusCode {
		c.failNow("expected %d, got %d", c.expectedStatus, resp.StatusCode)
		return nil
	}

	if _, ok := c.t.(*self.FakeTester); ok {
		// if you get here, and you are self testing then your test has failed to fail
		c.failNow("ASSERTION NOT MET")
	}

	return resp
}

// DoSimple performs as Do but reads any response payload to a string
func (c *Client) DoSimple(server *httptest.Server) SimpleResponse {
	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}
	resp := c.Do(server)
	if resp == nil {
		// test will have already failed for normal use, for self test the FakeTest will have detected the
		return SimpleResponse{}
	}
	defer func() { _ = resp.Body.Close() }()
	buf, err := io.ReadAll(resp.Body)
	if c.hasError(err) {
		// test will have already failed for normal use, for self test the FakeTest will have detected the
		return SimpleResponse{}
	}
	return SimpleResponse{
		Header: resp.Header,
		Status: resp.StatusCode,
		Body:   string(buf),
		t:      c.t,
	}
}

// joinPath for http paths
func joinPath(root, path string) string {
	if !strings.HasPrefix(path, "/") {
		return root + "/" + path
	}
	return root + path
}

// hasError returns true when there is error
func (c *Client) hasError(err error) bool {
	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}
	if err != nil {
		c.err = err
		c.failNow("Expected no error, got %v", err)
		return true
	}
	return false
}

// failNow forces an error
func (c *Client) failNow(format string, args ...interface{}) {
	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}
	c.err = fmt.Errorf(format, args...)
	c.t.Errorf(format, args...)
	c.t.FailNow()
}
