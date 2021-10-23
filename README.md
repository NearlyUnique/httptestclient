# Overview

Simplified creating http client calls for testing http servers or handlers. Removes the need for error checks whilst building clients so your tests can focus on what is important.

# Basic usage
```go
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
```

create a new `httptestclient`, set the `.Method`, `.Url`, `.Header`s, `.ExpectedStatusCode` as required, then call `.Do` (to get the `http.Response`) or `.DoSimple` to receive just the headers, status and body.

If any part of the construction or execution of the request fails the test will fail, but you don't need to specify this. 

# Avoid this

```go
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
```

There is so much error handling to make sure the test is valid that is not easy to see what is actually being tested