# Overview

Simplified creating http client calls for testing http servers or handlers. Removes the need for error checks whilst building clients so your tests can focus on what is important.

# Basic usage

```go
func Test_Example(t *testing.T) {
    myTestDatabase:=map[string]string{"a_key":"the_value"}
    s := httptest.NewServer(ProductionHandler(myTestDatabase))

    resp := httptestclient.New(t).
        Get("/any/a_key").
        Header("custom","my-value").
        DoSimple(s)

    // default is to allow resp.Status == 2xx so no need to assert

    if resp.Body != "the_value"{
        t.Errorf("expected the_value got %s", resp.Body)
    }
}
```

create a new httptestclient, set the `.Method`, `.Url`, `.Header`s, `.ExpectedStatusCode` as required, then call `.Do` (to get the `http.Response`) or `.DoSimple` to receive just the headers, status and body.

If any part of the construction or execution of the request fails the test will fail but you don't need to specify this. 

# Avoid this

```go
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
```

Every