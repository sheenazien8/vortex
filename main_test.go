package vortex

import (
	"bufio"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	opt := Opt{
		BaseURL: "http://example.com",
		Timeout: 5 * time.Second,
		Retries: 3,
	}
	client := New(opt)

	if client.baseURL != opt.BaseURL {
		t.Errorf("expected baseURL %s, got %s", opt.BaseURL, client.baseURL)
	}
	if client.httpClient.Timeout != opt.Timeout {
		t.Errorf("expected timeout %v, got %v", opt.Timeout, client.httpClient.Timeout)
	}
	if client.retries != opt.Retries {
		t.Errorf("expected retries %d, got %d", opt.Retries, client.retries)
	}
}

func TestSetHeader(t *testing.T) {
	client := New(Opt{})
	client.SetHeader("Content-Type", "application/json")

	if client.headers.Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type header to be application/json, got %s", client.headers.Get("Content-Type"))
	}
}

func TestSetHeaders(t *testing.T) {
	client := New(Opt{})
	headers := map[string]string{
		"Content-Type": "application/json",
		"Accept":       "application/json",
	}
	client.SetHeaders(headers)

	for key, value := range headers {
		if client.headers.Get(key) != value {
			t.Errorf("expected %s header to be %s, got %s", key, value, client.headers.Get(key))
		}
	}
}

func TestSetQueryParam(t *testing.T) {
	client := New(Opt{})
	client.SetQueryParam("key", "value")

	if client.queryParams.Get("key") != "value" {
		t.Errorf("expected query param key to be value, got %s", client.queryParams.Get("key"))
	}
}

func TestSetQueryParams(t *testing.T) {
	client := New(Opt{})
	params := map[string]interface{}{
		"key1": "value1",
		"key2": 2,
	}
	client.SetQueryParams(params)

	for key, value := range params {
		switch value.(type) {
		case string:
			if client.queryParams.Get(key) != value.(string) {
				t.Errorf("expected query param %s to be %v, got %s", key, value, client.queryParams.Get(key))
			}
		case int:
			if client.queryParams.Get(key) != fmt.Sprintf("%d", value.(int)) {
				t.Errorf("expected query param %s to be %v, got %s", key, value, client.queryParams.Get(key))
			}
		}
	}
}

func TestGet(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})
	client.SetQueryParam("key", "value")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "value" {
			t.Errorf("expected query param key to be value, got %s", r.URL.Query().Get("key"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	client.baseURL = server.URL
	resp, err := client.Get("/test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", resp.StatusCode)
	}
}

func TestPost(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected method POST, got %s", r.Method)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	client.baseURL = server.URL
	resp, err := client.Post("/test", map[string]string{"key": "value"})
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", resp.StatusCode)
	}
}

func TestMiddleware(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})
	client.UseMiddleware(func(req *http.Request, next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("X-Test", "middleware")
			next(w, r)
		}
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Test") != "middleware" {
			t.Errorf("expected X-Test header to be middleware, got %s", r.Header.Get("X-Test"))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	client.baseURL = server.URL
	resp, err := client.Get("/test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", resp.StatusCode)
	}
}

func TestSetFormFilePath(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})

	files := map[string]string{
		"file1": "/path/to/file1.txt",
		"file2": "/path/to/file2.txt",
	}

	client.SetFormFilePath("file1", files["file1"]).
		SetFormFilePath("file2", files["file2"])

	if len(client.formFilePath) != len(files) {
		t.Fatalf("expected %d files, got %d", len(files), len(client.formFilePath))
	}

	for key, filePath := range files {
		if client.formFilePath[key] != filePath {
			t.Errorf("expected file path for %s to be %s, got %s", key, filePath, client.formFilePath[key])
		}
	}
}

func TestHook(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})
	client.UseHook(func(req *http.Request, resp *http.Response) {
		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code 200, got %d", resp.StatusCode)
		}
	})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"message": "success"}`))
	}))
	defer server.Close()

	client.baseURL = server.URL
	resp, err := client.Get("/test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status code 200, got %d", resp.StatusCode)
	}
}

func TestStream(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("expected http.ResponseWriter to be an http.Flusher")
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		for i := 0; i < 5; i++ {
			fmt.Fprintf(w, "data: %d\n\n", i)
			flusher.Flush()
			time.Sleep(100 * time.Millisecond)
		}
	}))
	defer server.Close()

	client.baseURL = server.URL
	_, err := client.Stream(func(resp *http.Response) error {
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			t.Log(scanner.Text())
		}
		return scanner.Err()
	}).
		Get("/test")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestClientIsecure(t *testing.T) {
	client := New(Opt{})
	client.Insecure()

	if !client.insecure {
		t.Errorf("Expected client.insecure to be true, got %v", client.insecure)
	}
}

func TestGenerateCurlCommand(t *testing.T) {
	req := &Request{
		Method:  "POST",
		URL:     "http://example.com/api",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"key":"value"}`),
		QueryParams: url.Values{
			"param1": []string{"value1"},
			"param2": []string{"value2"},
		},
	}

	expectedCurlCommand := `curl -X POST "http://example.com/api?param1=value1&param2=value2" -H "Content-Type: application/json" --data-raw '{"key":"value"}'`

	curlCommand := req.GenerateCurlCommand()

	if curlCommand != expectedCurlCommand {
		t.Errorf("Expected curl command: %s, but got: %s", expectedCurlCommand, curlCommand)
	}

	reqMultipart := &Request{
		Method: "POST",
		URL:    "http://example.com/upload",
		Headers: http.Header{
			"Content-Type": []string{"multipart/form-data"},
		},
		FormFilePath: map[string]string{
			"file1": "/path/to/file1.txt",
			"file2": "/path/to/file2.txt",
		},
		FormData: map[string]string{
			"field1": "value1",
		},
	}

	expectedCurlCommandMultipart := `curl -X POST "http://example.com/upload" -H "Content-Type: multipart/form-data" -F "file1=@/path/to/file1.txt" -F "file2=@/path/to/file2.txt" -F "field1=value1"`

	curlCommandMultipart := reqMultipart.GenerateCurlCommand()

	if curlCommandMultipart != expectedCurlCommandMultipart {
		t.Errorf("Expected curl command: %s, but got: %s", expectedCurlCommandMultipart, curlCommandMultipart)
	}
}

func TestGenerateCurlCommandWithInsecureFlag(t *testing.T) {
	req := &Request{
		Method:  "POST",
		URL:     "https://example.com/api",
		Headers: http.Header{"Content-Type": []string{"application/json"}},
		Body:    []byte(`{"key":"value"}`),
		QueryParams: url.Values{
			"param1": []string{"value1"},
			"param2": []string{"value2"},
		},
		insecure: true,
	}

	expectedCurlCommand := `curl -k -X POST "https://example.com/api?param1=value1&param2=value2" -H "Content-Type: application/json" --data-raw '{"key":"value"}'`

	curlCommand := req.GenerateCurlCommand()

	if curlCommand != expectedCurlCommand {
		t.Errorf("Expected curl command: %s, but got: %s", expectedCurlCommand, curlCommand)
	}
}
