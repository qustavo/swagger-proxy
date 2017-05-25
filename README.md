# swagger-proxy [![Build Status](https://travis-ci.org/gchaincl/swagger-proxy.svg?branch=master)](https://travis-ci.org/gchaincl/swagger-proxy)
Swagger Proxy ensure HTTP Responses correctness based on swagger specs. 

# Usage
SwaggerProxy is designed to assist the development process, it can be used as a reverse proxy or as a middleware.

## Reverse Proxy
SwaggerProxy sits between the client/integration suite and the server as follows:
```
[Client] ---> [SwaggerProxy] ---> [Server]
```
SwaggerProxy will rely the request and analize the server response.

### Install
```bash
go get github.com/gchaincl/swagger-proxy/cmd/swagger-proxy`
```
### Run
```bash
$ swagger-proxy -h
Usage of swagger-proxy:
  -bind string
        Bind Address (default ":1234")
  -spec string
        Swagger Spec (default "swagger.yml")
  -target string
        Target (default "http://localhost:4321")
  -verbose
        Verbose
```

## Middleware
If your server is built in Golang, you can use it as a middleware:
```go
package main

import (
	"log"
	"net/http"

	proxy "github.com/gchaincl/swagger-proxy"
	"github.com/go-openapi/loads"
)

func main() {
	doc, err := loads.Spec("swagger.json")
	if err != nil {
		log.Fatal(err)
	}

	p, err := proxy.New(doc.Spec(), &proxy.LogReporter{}, proxy.WithVerbose(true))
	if err != nil {
		log.Fatal(err)
	}

	app := func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "POST" {
			w.WriteHeader(201)
		}
	}
	log.Printf("Server Running")
	http.ListenAndServe(":8989", p.Handler(http.HandlerFunc(app)))
}

```
