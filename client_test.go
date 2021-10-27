package httptestclient_test

import (
	"context"
	"errors"
	"fmt"
	"github.com/NearlyUnique/httptestclient/internal/self"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NearlyUnique/httptestclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_defaults(t *testing.T) {
	var actual struct {
		header http.Header
		url    string
		method string
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual.header = r.Header
		actual.url = r.URL.Path
		actual.method = r.Method
		w.Header().Set("a-header", "a-value")
		_, _ = w.Write([]byte(`any content`))
	}))
	defer s.Close()

	resp := httptestclient.New(t).DoSimple(s)

	assert.Equal(t, http.MethodGet, actual.method)
	assert.Equal(t, "/", actual.url)
	assert.Equal(t, "any content", resp.Body)

	assert.Equal(t, 3, len(actual.header))
	assert.Equal(t, "application/json", actual.header.Get("accept"))
	assert.Equal(t, httptestclient.UserAgent, actual.header.Get("user-agent"))
	// automatic in go http client
	assert.Equal(t, "gzip", actual.header.Get("Accept-Encoding"))

	assert.Equal(t, http.StatusOK, resp.Status)
	assert.Equal(t, "a-value", resp.Header.Get("a-header"))
}

func Test_overrides(t *testing.T) {
	var actual struct {
		header http.Header
		url    string
		method string
	}
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual.header = r.Header
		actual.url = r.URL.Path
		actual.method = r.Method
	}))
	defer s.Close()

	t.Run("http method can be overridden", func(t *testing.T) {
		for _, m := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodGet} {
			t.Run(fmt.Sprintf("set method to %v", m), func(t *testing.T) {
				_ = httptestclient.New(t).
					Method(m).
					DoSimple(s)

				assert.Equal(t, m, actual.method)
			})
		}
	})
	t.Run("URL can be overridden", func(t *testing.T) {
		testData := []struct {
			pattern  string
			args     []interface{}
			expected string
		}{
			{"/", nil, "/"},
			{"/a-path", nil, "/a-path"},
			{"path-without-leading/", nil, "/path-without-leading/"},
			{"/formatted/%s/path", []interface{}{"val"}, "/formatted/val/path"},
		}
		for _, td := range testData {
			t.Run(fmt.Sprintf("set url to %q", td.pattern), func(t *testing.T) {
				_ = httptestclient.New(t).
					URL(td.pattern, td.args...).
					DoSimple(s)

				assert.Equal(t, td.expected, actual.url)
			})
		}
	})
	t.Run("headers can be set", func(t *testing.T) {
		_ = httptestclient.New(t).
			Header("custom-header-1", "value1").
			Header("custom-header-2", "value2a", "value2b").
			Do(s)
		const defaultHeaderCount = 3 // see defaults test
		assert.Equal(t, defaultHeaderCount+2, len(actual.header))
		assert.Equal(t, "value1", actual.header.Get("custom-header-1"))
		assert.Equal(t, []string{"value2a", "value2b"}, actual.header["Custom-Header-2"])
	})
	t.Run("default httpclienttest headers can be removed and leave go client defaults", func(t *testing.T) {
		_ = httptestclient.New(t).
			ClearHeaders().
			Do(s)
		assert.Equal(t, 2, len(actual.header))
		assert.Equal(t, "gzip", actual.header.Get("Accept-Encoding"))
		assert.Equal(t, "Go-http-client/1.1", actual.header.Get("User-Agent"))
	})
}

func Test_http_status_codes(t *testing.T) {
	t.Run("if ExpectedStatusCode is not called then any 2xx passes", func(t *testing.T) {
		for _, statusCode := range []int{http.StatusOK, http.StatusCreated, http.StatusAccepted, http.StatusNoContent} {
			t.Run(fmt.Sprintf("respond with %v", statusCode), func(t *testing.T) {

				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(statusCode)
				}))
				defer s.Close()

				_ = httptestclient.New(t).DoSimple(s)
			})
		}
	})
	t.Run("if ExpectedStatusCode is not called then any non-2xx fails", func(t *testing.T) {
		for _, statusCode := range []int{http.StatusNotFound, http.StatusConflict, http.StatusInternalServerError} {
			t.Run(fmt.Sprintf("respond with %v", statusCode), func(t *testing.T) {
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(statusCode)
				}))
				defer s.Close()

				_ = httptestclient.New(self.NewFakeTester(func(format string, args ...interface{}) {
					assert.Equal(t, "expected success, got %d", format)
					require.Equal(t, 1, len(args))
					assert.Equal(t, statusCode, args[0].(int))
				})).DoSimple(s)
			})
		}
	})
	t.Run("when the server response with the specified status code the tests pass", func(t *testing.T) {
		for _, statusCode := range []int{http.StatusOK, http.StatusTeapot, http.StatusNotFound, http.StatusInternalServerError} {
			t.Run(fmt.Sprintf("respond with %v", statusCode), func(t *testing.T) {
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(statusCode)
				}))
				defer s.Close()

				_ = httptestclient.New(t).
					ExpectedStatusCode(statusCode).
					DoSimple(s)
			})
		}
	})
	t.Run("if a specific status code is given, any other code fails", func(t *testing.T) {
		for _, statusCode := range []int{http.StatusOK, http.StatusNotFound, http.StatusInternalServerError} {
			t.Run(fmt.Sprintf("expect %v but send 418 fails", statusCode), func(t *testing.T) {
				s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTeapot)
				}))
				defer s.Close()

				_ = httptestclient.
					New(self.NewFakeTester(func(format string, args ...interface{}) {
						assert.Equal(t, "expected %d, got %d", format)
						require.Equal(t, 2, len(args))
						assert.Equal(t, statusCode, args[0].(int))
						assert.Equal(t, http.StatusTeapot, args[1].(int))
					})).
					ExpectedStatusCode(statusCode).
					DoSimple(s)
			})
		}
	})
}

func Test_sending_a_payload(t *testing.T) {
	testData := []struct {
		method     string
		methodFunc func(*httptestclient.Client, string, ...interface{}) *httptestclient.Client
	}{
		{method: "POST", methodFunc: (*httptestclient.Client).Post},
		{method: "PUT", methodFunc: (*httptestclient.Client).Put},
		{method: "PATCH", methodFunc: (*httptestclient.Client).Patch},
	}
	for _, td := range testData {
		t.Run(fmt.Sprintf("send using %v", td.method), func(t *testing.T) {
			var actual struct {
				payload     string
				method      string
				contentType string
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actual.contentType = r.Header.Get("content-type")
				actual.method = r.Method
				buf, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				actual.payload = string(buf)
				defer func() { _ = r.Body.Close() }()
			}))
			defer s.Close()

			testClient := httptestclient.New(t).BodyString("any content")

			td.methodFunc(testClient, "/any").DoSimple(s)

			assert.Equal(t, "any content", actual.payload)
			assert.Equal(t, "application/json", actual.contentType)
			assert.Equal(t, td.method, actual.method)
		})
	}
}
func Test_sending_an_empty_payload_struct_fails_the_test(t *testing.T) {
	test := self.NewFakeTester(func(format string, args ...interface{}) {
		assert.Equal(t, "payload to send is nil", format)
	})
	req := httptestclient.New(test).
		Method(http.MethodPost).
		BodyJSON(nil).
		BuildRequest()
	assert.Nil(t, req)
}
func Test_sending_a_payload_struct(t *testing.T) {
	testData := []struct {
		method     string
		methodFunc func(*httptestclient.Client, string, ...interface{}) *httptestclient.Client
	}{
		{method: "POST", methodFunc: (*httptestclient.Client).Post},
		{method: "PUT", methodFunc: (*httptestclient.Client).Put},
		{method: "PATCH", methodFunc: (*httptestclient.Client).Patch},
	}
	for _, td := range testData {
		t.Run(fmt.Sprintf("send using %v", td.method), func(t *testing.T) {
			var actual struct {
				payload     string
				method      string
				contentType string
			}
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				actual.contentType = r.Header.Get("content-type")
				actual.method = r.Method
				buf, err := ioutil.ReadAll(r.Body)
				require.NoError(t, err)
				actual.payload = string(buf)
				defer func() { _ = r.Body.Close() }()
			}))
			defer s.Close()

			payload := struct {
				Name string `json:"name"`
				Age  int    `json:"age"`
			}{
				Name: "anyone",
				Age:  21,
			}
			testClient := httptestclient.New(t).BodyJSON(payload)

			td.methodFunc(testClient, "/any").DoSimple(s)

			assert.JSONEq(t, `{"age":21,"name":"anyone"}`, actual.payload)
			assert.Equal(t, "application/json", actual.contentType)
			assert.Equal(t, td.method, actual.method)
		})
	}
}
func Test_a_context_can_be_added_to_the_request(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer s.Close()

	// the context will be canceled causing an error when the request is made
	cancel()

	_ = httptestclient.
		New(self.NewFakeTester(func(format string, args ...interface{}) {
			assert.Equal(t, "Expected no error, got %v", format)
			err := errors.Unwrap(args[0].(error))
			assert.Equal(t, "context canceled", err.Error())
		})).
		Context(ctx).
		DoSimple(s)
}
func Test_raw_request_can_be_built(t *testing.T) {
	var actual struct {
		header http.Header
		url    string
		method string
	}
	productionHandler := func(w http.ResponseWriter, r *http.Request) {
		actual.header = r.Header
		actual.url = r.URL.Path
		actual.method = r.Method
		w.Header().Set("custom-response-header", "resp-1")
		_, _ = w.Write([]byte(`any content`))
	}

	rr := httptest.NewRecorder()

	req := httptestclient.
		New(t).
		Get("/path").
		Header("custom-request-header", "req-1").
		BuildRequest()

	productionHandler(rr, req)

	assert.Equal(t, http.MethodGet, actual.method)
	assert.Equal(t, "/path", actual.url)
	assert.Equal(t, "any content", rr.Body.String())

	assert.Equal(t, 3, len(actual.header))
	assert.Equal(t, "application/json", actual.header.Get("accept"))
	assert.Equal(t, "req-1", actual.header.Get("custom-request-header"))
	assert.Equal(t, httptestclient.UserAgent, actual.header.Get("user-agent"))

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "resp-1", rr.Header().Get("custom-response-header"))
}
