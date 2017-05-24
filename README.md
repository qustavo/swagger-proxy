# swagger-proxy
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
import(
  "github.com/go-openapi/loads"
  proxy "github.com/gchaincl/swagger-proxy"

)

func main() {
  // Load your spec
  doc, _ := loads.Spec("swagger.yml")
  
  // Construct the proxy instance
	p, _ := proxy.New(doc.Spec(), &proxy.LogReporter{})
  
  middleware := p.Handler
  // Now you can decorate your handler with middleware
}
```

# TODO
- [x] Handle shutdowns properly
- [x] Header verification (partially)
- [ ] Content-Type negotiation
- [ ] More testing
- [ ] Parameterize warning behavior
- [ ] Parameterize NotFound behavior
- [ ] Execute any integration test suite
- [ ] Analise request input
- [ ] Move this list to issues
