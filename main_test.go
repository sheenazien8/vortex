package vortex

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
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

func TestSetQueryParamFromInterface(t *testing.T) {
	client := New(Opt{})
	params := struct {
		Key1 string `json:"key1"`
		Key2 int    `json:"key2"`
	}{
		Key1: "value1",
		Key2: 2,
	}
	client.SetQueryParamFromInterface(params)

	if client.queryParams.Get("key1") != "value1" {
		t.Errorf("expected query param key1 to be value1, got %s", client.queryParams.Get("key1"))
	}
	if client.queryParams.Get("key2") != "2" {
		t.Errorf("expected query param key2 to be 2, got %s", client.queryParams.Get("key2"))
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

func TestWriteFormData(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})

	// Create temporary files for testing
	tempFile1, err := os.CreateTemp("", "file1.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile1.Name())
	tempFile1.WriteString("file1 content")
	tempFile1.Seek(0, 0)

	tempFile2, err := os.CreateTemp("", "file2.txt")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tempFile2.Name())
	tempFile2.WriteString("file2 content")
	tempFile2.Seek(0, 0)

	client.SetFormFilePath("file1", tempFile1.Name())
	client.SetFormFile("file2", tempFile2)

	formData := map[string]string{
		"field1": "value1",
		"field2": "value2",
	}
	client.SetFormData(formData)

	bodyBuffer := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuffer)

	err = client.writeFormData(writer)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify the multipart content
	contentType := writer.FormDataContentType()
	if !strings.HasPrefix(contentType, "multipart/form-data") {
		t.Errorf("expected content type to start with multipart/form-data, got %s", contentType)
	}

	// Parse the multipart content
	reader := multipart.NewReader(bodyBuffer, writer.Boundary())
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("expected no error, got %v", err)
		}

		partName := part.FormName()
		if partName == "file1" {
			content, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if string(content) != "file1 content" {
				t.Errorf("expected file1 content to be 'file1 content', got %s", string(content))
			}
		} else if partName == "file2" {
			content, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if string(content) != "file2 content" {
				t.Errorf("expected file2 content to be 'file2 content', got %s", string(content))
			}
		} else if value, ok := formData[partName]; ok {
			content, err := io.ReadAll(part)
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if string(content) != value {
				t.Errorf("expected form field %s to be %s, got %s", partName, value, string(content))
			}
		} else {
			t.Errorf("unexpected form field %s", partName)
		}
	}
}

func TestSetFormFile(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})

	file1 := &os.File{}
	file2 := &os.File{}

	client.SetFormFile("file1", file1).
		SetFormFile("file2", file2)

	if len(client.formFile) != 2 {
		t.Fatalf("expected 2 files, got %d", len(client.formFile))
	}

	if client.formFile["file1"] != file1 {
		t.Errorf("expected file1 to be set correctly")
	}

	if client.formFile["file2"] != file2 {
		t.Errorf("expected file2 to be set correctly")
	}
}

func TestSetFormData(t *testing.T) {
	client := New(Opt{BaseURL: "http://example.com"})

	formData := map[string]string{
		"field1": "value1",
		"field2": "value2",
	}

	client.SetFormData(formData)

	if len(client.formData) != len(formData) {
		t.Fatalf("expected %d form data fields, got %d", len(formData), len(client.formData))
	}

	for key, value := range formData {
		if client.formData[key] != value {
			t.Errorf("expected form data for %s to be %s, got %s", key, value, client.formData[key])
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

func TestGenerateCurlCommandWithValidFormFile(t *testing.T) {
	fileName := "test.txt"
	mockFile, err := NewMockFile(&fileName)
	if err != nil {
		t.Fatalf("Failed to create mock file: %v", err)
	}
	defer mockFile.Close()
	mockFile.SetName("valid.txt")

	req := &Request{
		Method: "POST",
		URL:    "http://example.com/upload",
		Headers: http.Header{
			"Content-Type": []string{"multipart/form-data"},
		},
		FormFile: map[string]multipart.File{
			"file1": mockFile,
		},
	}

	curlCommand := req.GenerateCurlCommand()
	expectedCurlCommand := `curl -X POST "http://example.com/upload" -H "Content-Type: multipart/form-data" -F "file1=@valid.txt"`

	if curlCommand != expectedCurlCommand {
		t.Errorf("Expected curl command to be same with %s, but got: %s", expectedCurlCommand, curlCommand)
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

type MockFile struct {
	*os.File
	data       []byte
	currentPos int64
	readError  error
	writeError error
	closeError error
	name       string
}

func NewMockFile(name *string) (*MockFile, error) {
	realFile, err := os.CreateTemp("", *name)
	if err != nil {
		return nil, err
	}

	return &MockFile{
		File: realFile,
		data: []byte{},
	}, nil
}

func (m *MockFile) Read(p []byte) (n int, err error) {
	if m.readError != nil {
		return 0, m.readError
	}

	if m.currentPos >= int64(len(m.data)) {
		return 0, io.EOF
	}

	n = copy(p, m.data[m.currentPos:])
	m.currentPos += int64(n)
	return n, nil
}

func (m *MockFile) ReadAt(p []byte, a int64) (n int, err error) {
	if m.readError != nil {
		return 0, m.readError
	}

	if m.currentPos >= int64(len(m.data)) {
		return 0, io.EOF
	}

	n = copy(p, m.data[m.currentPos:])
	m.currentPos += int64(n)
	return n, nil
}

func (m *MockFile) Write(p []byte) (n int, err error) {
	if m.writeError != nil {
		return 0, m.writeError
	}

	m.data = append(m.data, p...)
	return len(p), nil
}

func (m *MockFile) Close() error {
	return m.closeError
}

func (m *MockFile) Seek(offset int64, whence int) (int64, error) {
	if m.readError != nil {
		return 0, m.readError
	}

	switch whence {
	case io.SeekStart:
		m.currentPos = offset
	case io.SeekCurrent:
		m.currentPos += offset
	case io.SeekEnd:
		m.currentPos = int64(len(m.data)) + offset
	}

	return m.currentPos, nil
}

func (m *MockFile) Name() string {
	return m.name
}

func (m *MockFile) SetName(name string) {
	m.name = name
}

func (m *MockFile) SetReadError(err error) {
	m.readError = err
}

func (m *MockFile) SetWriteError(err error) {
	m.writeError = err
}

func (m *MockFile) SetCloseError(err error) {
	m.closeError = err
}
