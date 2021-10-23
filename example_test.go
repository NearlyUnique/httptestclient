package httptestclient_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/NearlyUnique/httptestclient"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type database map[string]string

// ProductionHandler takes the pPOST body, URL and header to return json
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
		buf, err := ioutil.ReadAll(r.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var c Customer
		err = json.Unmarshal(buf, &c)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		payload := struct {
			Value string `json:"value"`
		}{
			Value: fmt.Sprintf("%s %s %s", val, c.Name, r.Header.Get("custom")),
		}
		buf, err = json.Marshal(payload)
		if err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(buf)
	})
}

// Customer for testing
type Customer struct {
	Name string `json:"name"`
}

func Test_Example(t *testing.T) {
	myTestDatabase := map[string]string{"database-key": "Hello"}
	s := httptest.NewServer(ProductionHandler(myTestDatabase))

	resp := httptestclient.New(t).
		Post("/any/%s", "database-key").
		BodyJSON(&Customer{Name: "Bob"}).
		Header("custom", "😊").
		DoSimple(s)

	// default is to allow resp.Status == 2xx so no need to assert
	payload := map[string]string{}
	resp.BodyJSON(&payload)
	if payload["value"] != "Hello Bob 😊" {
		t.Errorf("expected json with name='Hello Bob 😊' got %s", resp.Body)
	}
}

func Test_Example_ToAvoid(t *testing.T) {
	myTestDatabase := map[string]string{"database-key": "Hello"}
	s := httptest.NewServer(ProductionHandler(myTestDatabase))

	buf, err := json.Marshal(Customer{Name: "Bob"})
	if err != nil {
		t.Errorf("failed to marshal request: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, s.URL+"/any/database-key", bytes.NewReader(buf))
	if err != nil {
		t.Errorf("failed to create request: %v", err)
	}
	req.Header.Set("custom", "😊")
	resp, err := s.Client().Do(req)
	if err != nil {
		t.Errorf("failed to execute request: %v", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		t.Errorf("expected 2xx OK got %d", resp.StatusCode)
	}
	defer resp.Body.Close()
	buf, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		t.Errorf("failed to read response body: %v", err)
	}
	payload := map[string]string{}
	err = json.Unmarshal(buf, &payload)
	if err != nil {
		t.Errorf("failed to unmarshal response body: %v", err)
	}
	if payload["value"] != "Hello Bob 😊" {
		t.Errorf("expected json with name='Hello Bob 😊' got %s", string(buf))
	}
}
