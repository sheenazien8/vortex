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
- [x] Generate curl command
- [x] Hook 
- [x] Stream Request
- [ ] Form Data Support
- [ ] Form Upload Support


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

## Generate Curl Command
```go
curlCommand := resp.Request.GenerateCurlCommand()
println("Generated Curl Command:", curlCommand)

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

## Stream Request
```go
func streamRequest(resp *http.Response) {
	log.Printf("Stream: Response status code: %d", resp.StatusCode)
}

apiClient := vortex.New(vortex.Opt{
    BaseURL: "https://lakasir.test",
})

_, err := apiClient.
		Stream(streamRequest).
		Post("/test")
```

## Contributing

We welcome contributions to the Vortex project! If you would like to contribute, please follow these guidelines:

1. **Fork the repository**: Click the "Fork" button at the top right of this repository to create a copy of the repository in your GitHub account.

2. **Clone your fork**: Clone your forked repository to your local machine.
    ```sh
    git clone https://github.com/sheenazien8/vortex.git
    cd vortex
    ```

3. **Create a new branch**: Create a new branch for your feature or bugfix.
    ```sh
    git checkout -b my-feature-branch
    ```

4. **Make your changes**: Make your changes to the codebase. Ensure that your code follows the project's coding standards and includes appropriate tests.

5. **Commit your changes**: Commit your changes with a descriptive commit message.
    ```sh
    git add .
    git commit -m "Add feature X"
    ```

6. **Push to your fork**: Push your changes to your forked repository.
    ```sh
    git push origin my-feature-branch
    ```

7. **Create a pull request**: Go to the original repository on GitHub and create a pull request from your forked repository. Provide a clear description of your changes and any related issues.

8. **Review process**: Your pull request will be reviewed by the maintainers. Please be responsive to any feedback and make necessary changes.

Thank you for contributing to Vortex!

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for more details.
