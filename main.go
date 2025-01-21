package vortex

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Middleware func(req *http.Request, next http.HandlerFunc) http.HandlerFunc

type Hook func(req *http.Request, resp *http.Response)

type Opt struct {
	BaseURL string
	Timeout time.Duration
	Retries int
}

type Client struct {
	httpClient    *http.Client
	baseURL       string
	retries       int
	headers       http.Header
	queryParams   url.Values
	output        interface{}
	middleware    []Middleware
	hooks         []Hook
	streamHandler func(*http.Response) error
	formFilePath  map[string]string
	formData      map[string]string
	insecure      bool
	formFile     map[string]multipart.File
}

func (c *Client) UseMiddleware(middleware ...Middleware) *Client {
	c.middleware = append(c.middleware, middleware...)
	return c
}

func (c *Client) UseHook(hooks ...Hook) *Client {
	c.hooks = append(c.hooks, hooks...)
	return c
}

func New(opt Opt) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: opt.Timeout,
		},
		baseURL:     opt.BaseURL,
		retries:     opt.Retries,
		headers:     http.Header{},
		queryParams: url.Values{},
		insecure:    false,
	}
}

func (c *Client) Insecure() *Client {
	c.insecure = true
	return c
}

func (c *Client) SetFormFilePath(key, filePath string) *Client {
	if c.formFilePath == nil {
		c.formFilePath = make(map[string]string)
	}
	c.formFilePath[key] = filePath
	return c
}

func (c *Client) SetFormFile(fieldName string, file multipart.File) *Client {
	if c.formFile == nil {
		c.formFile = make(map[string]multipart.File)
	}
	c.formFile[fieldName] = file
	return c
}

func (c *Client) SetFormData(params map[string]string) *Client {
	if c.formData == nil {
		c.formData = make(map[string]string)
	}
	for key, value := range params {
		c.formData[key] = value
	}
	return c
}

func (c *Client) SetHeader(key, value string) *Client {
	c.headers.Set(key, value)
	return c
}

func (c *Client) SetHeaders(headers map[string]string) *Client {
	for key, value := range headers {
		c.headers.Set(key, value)
	}
	return c
}

func (c *Client) SetQueryParam(key, value string) *Client {
	c.queryParams.Set(key, value)
	return c
}

func (c *Client) SetQueryParams(params map[string]interface{}) *Client {
	for key, value := range params {
		c.queryParams.Set(key, fmt.Sprintf("%v", value))
	}
	return c
}

func (c *Client) SetQueryParamFromInterface(params interface{}) *Client {
	jsonParams, _ := json.Marshal(params)
	var queryParams map[string]interface{}
	err := json.Unmarshal(jsonParams, &queryParams)
	if err != nil {
		log.Fatalf("Error unmarshalling query params: %v", err)
	}

	for key, value := range queryParams {
		c.queryParams.Set(key, fmt.Sprintf("%v", value))
	}
	return c
}

func (c *Client) SetOutput(output interface{}) *Client {
	c.output = output
	return c
}

func (c *Client) doRequest(method, endpoint string, body interface{}) (response *Response, err error) {
	reqBody, jsonBody, writer, err := c.prepareRequestBody(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	c.setRequestHeaders(req, method, writer)

	var request Request
	handler := c.createHandler(method, req, jsonBody, &request)

	for i := len(c.middleware) - 1; i >= 0; i-- {
		handler = c.middleware[i](req, handler)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return &Response{
		StatusCode: recorder.Result().StatusCode,
		Body:       recorder.Body.Bytes(),
		Output:     c.output,
		Request:    &request,
	}, nil
}

func (c *Client) prepareRequestBody(body interface{}) (io.Reader, []byte, *multipart.Writer, error) {
	var reqBody io.Reader
	var jsonBody []byte
	var bodyBuffer *bytes.Buffer
	var writer *multipart.Writer
	var err error

	if len(c.formFilePath) > 0 || len(c.formData) > 0 || len(c.formFile) > 0 {
		bodyBuffer = &bytes.Buffer{}
		writer = multipart.NewWriter(bodyBuffer)
		err = c.writeFormData(writer)
		if err != nil {
			return nil, nil, nil, err
		}
		reqBody = bodyBuffer
	} else if body != nil {
		jsonBody, err = json.Marshal(body)
		if err != nil {
			return nil, nil, nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	return reqBody, jsonBody, writer, nil
}

func (c *Client) writeFormData(writer *multipart.Writer) error {
	for key, filePath := range c.formFilePath {
		file, err := os.Open(filePath)
		if err != nil {
			return err
		}
		defer file.Close()

		part, err := writer.CreateFormFile(key, filepath.Base(file.Name()))
		if err != nil {
			return err
		}

		_, err = io.Copy(part, file)
		if err != nil {
			return err
		}
	}

	for key, value := range c.formData {
		_ = writer.WriteField(key, value)
	}

	for fieldname, file := range c.formFile {
		fileHeader, ok := file.(*os.File)
		if !ok {
			return fmt.Errorf("file is not an *os.File")
		}
		defer fileHeader.Close()
		part, err := writer.CreateFormFile(fieldname, fileHeader.Name())
		if err != nil {
			return err
		}
		_, err = io.Copy(part, file)
		if err != nil {
			return err
		}
	}

	return writer.Close()
}

func (c *Client) setRequestHeaders(req *http.Request, method string, writer *multipart.Writer) {
	switch method {
	case "GET", "DELETE":
		req.URL.RawQuery = c.queryParams.Encode()
	case "POST", "PUT", "PATCH":
		if c.headers.Get("Content-Type") == "" && len(c.formFilePath) == 0 {
			req.Header.Set("Content-Type", "application/json")
		}
	}

	if len(c.formFilePath) > 0 || len(c.formData) > 0 || len(c.formFile) > 0 {
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}

	c.addHeaders(req)
}

func (c *Client) createHandler(method string, req *http.Request, jsonBody []byte, request *Request) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		httpClient := c.httpClient
		if c.insecure {
			println("insecure")
			httpClient.Transport = &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: c.insecure},
			}
		}
		resp, err := httpClient.Do(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		for _, hook := range c.hooks {
			hook(r, resp)
		}

		if c.streamHandler != nil {
			err := c.streamHandler(resp)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		respBody, _ := io.ReadAll(resp.Body)

		var output interface{}
		if c.output != nil {
			output = c.output
			err = json.Unmarshal(respBody, output)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		}

		*request = Request{
			Method:       method,
			URL:          req.URL.String(),
			Headers:      req.Header,
			Body:         jsonBody,
			FormFilePath: c.formFilePath,
			FormData:     c.formData,
			FormFile:     c.formFile,
			insecure:     c.insecure,
		}

		w.Header().Set("StatusCode", fmt.Sprintf("%d", resp.StatusCode))
		w.WriteHeader(resp.StatusCode)
		_, err = w.Write(respBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

func (c *Client) Get(endpoint string) (*Response, error) {
	return c.doRequest("GET", endpoint, nil)
}

func (c *Client) Delete(endpoint string) (*Response, error) {
	return c.doRequest("DELETE", endpoint, nil)
}

func (c *Client) Post(endpoint string, body interface{}) (*Response, error) {
	return c.doRequest("POST", endpoint, body)
}

func (c *Client) Put(endpoint string, body interface{}) (*Response, error) {
	return c.doRequest("PUT", endpoint, body)
}

func (c *Client) Patch(endpoint string, body interface{}) (*Response, error) {
	return c.doRequest("PATCH", endpoint, body)
}

func (c *Client) Stream(streamHandler func(*http.Response) error) *Client {
	c.streamHandler = streamHandler
	return c
}

func (c *Client) addHeaders(req *http.Request) {
	for key, values := range c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}
}

type Response struct {
	StatusCode int
	Body       []byte
	Output     interface{}
	Request    *Request
}

type Request struct {
	Method       string
	URL          string
	Headers      http.Header
	Body         []byte
	QueryParams  url.Values
	FormFilePath map[string]string
	FormData     map[string]string
	FormFile     map[string]multipart.File
	insecure     bool
}

type NamedFile interface {
    Name() string
    multipart.File
}

func (r *Request) GenerateCurlCommand() string {
	var curlCommand strings.Builder
	curlCommand.WriteString("curl")
	if r.insecure {
		curlCommand.WriteString(" -k")
	}
	curlCommand.WriteString(" -X " + r.Method)
	curlCommand.WriteString(" \"")

	if len(r.QueryParams) > 0 {
		curlCommand.WriteString(r.URL)
		curlCommand.WriteString("?")
		curlCommand.WriteString(r.QueryParams.Encode())
	} else {
		curlCommand.WriteString(r.URL)
	}
	curlCommand.WriteString("\"")

	for key, values := range r.Headers {
		for _, value := range values {
			if key == "Content-Type" && strings.Contains(value, "boundary") {
				value = strings.Split(value, ";")[0]
			}
			curlCommand.WriteString(" -H \"")
			curlCommand.WriteString(key)
			curlCommand.WriteString(": ")
			curlCommand.WriteString(value)
			curlCommand.WriteString("\"")
		}
	}

	if (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") && len(r.Body) > 0 || len(r.FormFilePath) > 0 || len(r.FormData) > 0 || len(r.FormFile) > 0 {
		contentType := r.Headers.Get("Content-Type")
		if strings.Contains(contentType, "multipart/form-data") {
			for key, filePath := range r.FormFilePath {
				curlCommand.WriteString(" -F \"")
				curlCommand.WriteString(key)
				curlCommand.WriteString("=@")
				curlCommand.WriteString(filePath)
				curlCommand.WriteString("\"")
			}

			for key, value := range r.FormData {
				curlCommand.WriteString(" -F \"")
				curlCommand.WriteString(key)
				curlCommand.WriteString("=")
				curlCommand.WriteString(value)
				curlCommand.WriteString("\"")
			}

			for fieldname, file := range r.FormFile {
				namedFile, ok := file.(NamedFile)
				if !ok {
					return ""
				}
				curlCommand.WriteString(" -F \"")
				curlCommand.WriteString(fieldname)
				curlCommand.WriteString("=@")
				curlCommand.WriteString(namedFile.Name())
				curlCommand.WriteString("\"")
			}
		} else {
			curlCommand.WriteString(" --data-raw '")
			curlCommand.WriteString(string(r.Body))
			curlCommand.WriteString("'")
		}
	}

	return curlCommand.String()
}
