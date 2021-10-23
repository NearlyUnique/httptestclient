package httptestclient_test

import (
	"github.com/NearlyUnique/httptestclient"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type database map[string]string

func ProductionHandler(conn database) http.Handler {
	// other dependency setup here
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.Split(r.URL.Path, "/")
		if len(p) == 0 {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		val, ok := conn[p[len(p)-1]]
		if !ok {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		_, _ = w.Write([]byte(val))
	})
}

func Test_Example(t *testing.T) {
	myTestDatabase := map[string]string{"a_key": "the_value"}
	s := httptest.NewServer(ProductionHandler(myTestDatabase))

	resp := httptestclient.New(t).
		Get("/any/a_key").
		Header("custom", "my-value").
		DoSimple(s)

	// default is to allow resp.Status == 2xx so no need to assert

	if resp.Body != "the_value" {
		t.Errorf("expected the_value got %s", resp.Body)
	}
}

func Test_Example_ToAvoid(t *testing.T) {
	myTestDatabase := map[string]string{"a_key": "the_value"}
	s := httptest.NewServer(ProductionHandler(myTestDatabase))

	req, err := http.NewRequest(http.MethodGet, s.URL+"/any/a_key", nil)
	if err != nil {
		t.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("custom", "a_key")
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Errorf("failed to execute request: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Errorf("expected 2xx OK got %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed to read response body: %v", err)
	}
	if string(buf) != "the_value" {
		t.Errorf("expected the_value got %s", string(buf))
	}
}
