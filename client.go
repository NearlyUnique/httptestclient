// Package httptestclient generates and executes standard http requests. It follows the basic builder pattern
// to build and/or execute the request against a httptest.Server
// Any errors that occur during the build or execute of the request will fail the test so that you can focus on your
// server not on the http client
package httptestclient

import (
	"bytes"
	"context"
	"fmt"
	"github.com/NearlyUnique/httptestclient/internal/self"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
)

// UserAgent default value
const UserAgent = "test-http-request"

// ContentTypeApplicationJson for http header Content-Type
const ContentTypeApplicationJson = "application/json"

// DefaultContentType when content is detected
var DefaultContentType = ContentTypeApplicationJson

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

// SimpleResponse simplified status response rather than using the http.Response directly
type SimpleResponse struct {
	Header http.Header
	Body   string
	Status int
}

// Client simplifies creating test client http request
type Client struct {
	t TestingT

	method         string
	url            string
	header         http.Header
	body           io.Reader
	context        context.Context
	expectedStatus int
	err            error
}

// New for testing, finish with Client.Do or Client.DoSimple
// `t` use `*testing.T`
// `s` server under test
// example:
//   resp2 := New(t).
//		Post("/some-request/%s/resource", id).
//		Header("special-header", "magic").
//		BodyString(`{"token":"opaque-string"}`).
//		Do(server)
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
func (c *Client) ExpectedStatusCode(status int) *Client {
	c.expectedStatus = status
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

// Post is short-hand for
//   testClient.Method("POST").URL(...)
func (c *Client) Post(url string, args ...interface{}) *Client {
	return c.Method(http.MethodPost).URL(url, args...)
}

// Put is short-hand for
//   testClient.Method("PUT").URL(...)
func (c *Client) Put(url string, args ...interface{}) *Client {
	return c.Method(http.MethodPut).URL(url, args...)
}

// Patch is short-hand for
//   testClient.Method("PATCH").URL(...)
func (c *Client) Patch(url string, args ...interface{}) *Client {
	return c.Method(http.MethodPatch).URL(url, args...)
}

// Get is short-hand for
//   testClient.Method("GET").URL(...)
func (c *Client) Get(url string, args ...interface{}) *Client {
	return c.Method(http.MethodGet).URL(url, args...)
}

// Delete is short-hand for
//   testClient.Method("DELETE").URL(...)
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

// BodyString is a literal string version of the body
// if the body has a value the content-type will be 'application/json' will be added
// unless you set an alternative or use ClearHeaders()
func (c *Client) BodyString(body string) *Client {
	return c.BodyBytes([]byte(body))
}

// Do the http request, http status must either match expected or be success
func (c *Client) Do(server *httptest.Server) *http.Response {
	if h, ok := c.t.(testingHooks); ok {
		h.Helper()
	}
	url := joinPath(server.URL, c.url)
	req, err := http.NewRequestWithContext(c.context, c.method, url, c.body)
	if c.hasError(err) {
		return nil
	}
	req.Header = c.header
	if c.body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", DefaultContentType)
	}

	resp, err := server.Client().Do(req)
	if c.hasError(err) {
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
		// if you get here, and you are self testing then your tst has failed to fail
		c.t.Errorf("ASSERTION NOT MET")
		c.t.FailNow()
	}

	return resp
}

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
	buf, err := ioutil.ReadAll(resp.Body)
	if c.hasError(err) {
		// test will have already failed for normal use, for self test the FakeTest will have detected the
		return SimpleResponse{}
	}
	return SimpleResponse{
		Header: resp.Header,
		Status: resp.StatusCode,
		Body:   string(buf),
	}
}

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
		c.t.Errorf("Expected no error, got %v", err)
		c.t.FailNow()
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
