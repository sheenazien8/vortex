# Vortex
Simple HTTP and REST client library for Go, Inspired by Guzzle HTTP client from PHP.

## Installation
```sh
go get github.com/sheenazien8/vortex
```

## Features
- [x] Simple request
- [x] POST, GET, DELETE, PUT, PATCH, DELETE
- [x] Middleware
- [x] Hook 


## Usage
```go
package main

import (
	"github.com/sheenazien8/vortex"
	"log"
)

func main() {
	var response struct {
		Data struct {
			Email string `json:"email"`
			Token string `json:"token"`
		} `json:"data"`

		Message string `json:"message"`
		Status  bool   `json:"status"`
	}

	apiClient := vortex.New(vortex.Opt{
		BaseURL: "https://lakasir.test",
	})
	resp, err := apiClient.
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetOutput(&response).
		Post("/api/auth/login", map[string]interface{}{
			"email":    "warunghikmah@lakasir.com",
			"password": "password",
		})
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != 200 {
		log.Fatal("status code is not 200 ", resp.StatusCode, string(resp.Body))
	}

	data := response.Data
	var token = data.Token

	meResponse, err := apiClient.
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("Authorization", "Bearer "+token).
		SetOutput(&response).
		Get("/api/auth/me")
	if err != nil {
		panic(err)
	}

	if meResponse.StatusCode != 200 {
		log.Fatal("status code is not 200 ", meResponse.StatusCode, string(meResponse.Body))
	}

	println(response.Data.Email)
}
```

## Middleware
```go
func LoggingMiddleware(req *http.Request, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("Request: %s %s", r.Method, r.URL.String())
		next.ServeHTTP(w, r)
		log.Printf("Response: %s", w.Header().Get("StatusCode"))
	}
}

apiClient := vortex.New(vortex.Opt{
    BaseURL: "https://lakasir.test",
})

resp, err := apiClient.
    UseMiddleware(LoggingMiddleware)
```

## Hook
```go

func ExampleHook(req *http.Request, resp *http.Response) {
	log.Printf("Hook: Response status code: %d", resp.StatusCode)
}

apiClient := vortex.New(vortex.Opt{
    BaseURL: "https://lakasir.test",
})

resp, err := apiClient.
		UseHook(ExampleHook)
```
