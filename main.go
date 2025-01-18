package vortex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Opt struct {
	BaseURL string
	Timeout time.Duration
	Retries int
}

type Client struct {
	httpClient *http.Client
	baseURL    string
	retries    int
	headers    http.Header
	queryParams url.Values
	output     interface{}
}

func New(opt Opt) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: opt.Timeout,
		},
		baseURL:    opt.BaseURL,
		retries:    opt.Retries,
		headers:    http.Header{},
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

func (c *Client) Get(endpoint string) (*Response, error) {
	req, err := http.NewRequest("GET", c.baseURL+endpoint, nil)
	if err != nil {
		return nil, err
	}

	req.URL.RawQuery = c.queryParams.Encode()

	for key, values := range c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	var output interface{}
	if c.output != nil {
		output = c.output
		err = json.Unmarshal(body, output)
		if err != nil {
			return nil, err
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       body,
		Output:     output,
		Request: &Request{
			Method:     "GET",
			URL:        req.URL.String(),
			Headers:    req.Header,
			Body:       nil,
			QueryParams: c.queryParams,
		},
	}, nil
}

func (c *Client) Post(endpoint string, body interface{}) (*Response, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", c.baseURL+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	// Add headers
	for key, values := range c.headers {
		for _, value := range values {
			req.Header.Add(key, value)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// Unmarshal the response body into the output interface
	var output interface{}
	if c.output != nil {
		output = c.output
		err = json.Unmarshal(respBody, output)
		if err != nil {
			return nil, err
		}
	}

	return &Response{
		StatusCode: resp.StatusCode,
		Body:       respBody,
		Output:     output,
		Request: &Request{
			Method:     "POST",
			URL:        req.URL.String(),
			Headers:    req.Header,
			Body:       jsonBody,
			QueryParams: c.queryParams,
		},
	}, nil
}

type Response struct {
	StatusCode int
	Body       []byte
	Output     interface{}
	Request    *Request
}

type Request struct {
	Method     string
	URL        string
	Headers    http.Header
	Body       []byte
	QueryParams url.Values
}

func (r *Request) GenerateCurlCommand() string {
	var curlCommand strings.Builder
	curlCommand.WriteString("curl -X ")
	curlCommand.WriteString(r.Method)
	curlCommand.WriteString(" \"")
	curlCommand.WriteString(r.URL)
	curlCommand.WriteString("\"")

	// Add query parameters
	if len(r.QueryParams) > 0 {
		curlCommand.WriteString("?")
		curlCommand.WriteString(r.QueryParams.Encode())
	}

	// Add headers
	for key, values := range r.Headers {
		for _, value := range values {
			curlCommand.WriteString(" -H \"")
			curlCommand.WriteString(key)
			curlCommand.WriteString(": ")
			curlCommand.WriteString(value)
			curlCommand.WriteString("\"")
		}
	}

	// Add body for POST requests
	if r.Method == "POST" && len(r.Body) > 0 {
		curlCommand.WriteString(" -d '")
		curlCommand.WriteString(string(r.Body))
		curlCommand.WriteString("'")
	}

	return curlCommand.String()
}
