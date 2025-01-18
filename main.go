package vortex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
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
	httpClient  *http.Client
	baseURL     string
	retries     int
	headers     http.Header
	queryParams url.Values
	output      interface{}
	middleware  []Middleware
	hooks       []Hook
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
	}
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

func (c *Client) doRequest(method, endpoint string, body interface{}) (*Response, error) {
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequest(method, c.baseURL+endpoint, reqBody)
	if err != nil {
		return nil, err
	}

	if method == "GET" || method == "DELETE" {
		req.URL.RawQuery = c.queryParams.Encode()
	} else if method == "POST" || method == "PUT" || method == "PATCH" {
		req.Header.Set("Content-Type", "application/json")
	}

	c.addHeaders(req)

	var request Request
	var reqBodyBytes []byte
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp, err := c.httpClient.Do(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		// Apply hooks
		for _, hook := range c.hooks {
			hook(r, resp)
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

		if reqBody != nil {
			reqBodyBytes, _ = io.ReadAll(reqBody)
		}

		request = Request{
			Method:  method,
			URL:     req.URL.String(),
			Headers: req.Header,
			Body:    reqBodyBytes,
		}

		w.Header().Set("StatusCode", fmt.Sprintf("%d", resp.StatusCode))
		_, err = w.Write(respBody)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})

	for i := len(c.middleware) - 1; i >= 0; i-- {
		handler = c.middleware[i](req, handler)
	}

	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)

	return &Response{
		StatusCode: recorder.Code,
		Body:       recorder.Body.Bytes(),
		Output:     c.output,
		Request: &request,
	}, nil
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
	Method      string
	URL         string
	Headers     http.Header
	Body        []byte
	QueryParams url.Values
}

func (r *Request) GenerateCurlCommand() string {
	var curlCommand strings.Builder
	curlCommand.WriteString("curl -X ")
	curlCommand.WriteString(r.Method)
	curlCommand.WriteString(" \"")
	curlCommand.WriteString(r.URL)
	curlCommand.WriteString("\"")

	if len(r.QueryParams) > 0 {
		curlCommand.WriteString("?")
		curlCommand.WriteString(r.QueryParams.Encode())
	}

	for key, values := range r.Headers {
		for _, value := range values {
			curlCommand.WriteString(" -H \"")
			curlCommand.WriteString(key)
			curlCommand.WriteString(": ")
			curlCommand.WriteString(value)
			curlCommand.WriteString("\"")
		}
	}

	if (r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH") && len(r.Body) > 0 {
		curlCommand.WriteString(" -d '")
		curlCommand.WriteString(string(r.Body))
		curlCommand.WriteString("'")
	}

	return curlCommand.String()
}
