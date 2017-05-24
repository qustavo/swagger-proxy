package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/swag"
	"github.com/go-openapi/validate"
	"github.com/gorilla/mux"
)

type Proxy struct {
	// Opts
	target  string
	verbose bool

	router       *mux.Router
	routes       map[*mux.Route]*spec.Operation
	reverseProxy http.Handler

	reporter Reporter
	doc      interface{} // This is useful for validate (TODO: find a better way)
}

type ProxyOpt func(*Proxy)

func WithTarget(target string) ProxyOpt { return func(proxy *Proxy) { proxy.target = target } }
func WithVerbose(v bool) ProxyOpt       { return func(proxy *Proxy) { proxy.verbose = v } }

func New(s *spec.Swagger, reporter Reporter, opts ...ProxyOpt) (*Proxy, error) {
	// validate.NewSchemaValidator requires the spec as an interface{}
	// That's why we Unmarshal(Marshal()) the document
	data, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}

	var doc interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, err
	}

	proxy := &Proxy{
		target:   "http://localhost:8080",
		router:   mux.NewRouter(),
		routes:   make(map[*mux.Route]*spec.Operation),
		reporter: reporter,
		doc:      doc,
	}

	for _, opt := range opts {
		opt(proxy)
	}

	rpURL, err := url.Parse(proxy.target)
	if err != nil {
		return nil, err
	}
	proxy.reverseProxy = httputil.NewSingleHostReverseProxy(rpURL)

	proxy.router.NotFoundHandler = http.HandlerFunc(proxy.notFound)
	proxy.registerPaths(s.BasePath, s.Paths)

	return proxy, nil
}

func (proxy *Proxy) notFound(w http.ResponseWriter, req *http.Request) {
	http.Error(w, "Not Found", http.StatusNotFound)
}

func (proxy *Proxy) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	proxy.router.ServeHTTP(w, req)
}

func (proxy *Proxy) registerPaths(base string, paths *spec.Paths) {
	for path, item := range paths.Paths {
		// Register every spec operation under a newHandler
		for method, op := range getOperations(&item) {
			handler := proxy.newHandler()
			newPath := base + path
			if proxy.verbose {
				log.Printf("Register %s %s", method, newPath)
			}
			route := proxy.router.HandleFunc(newPath, handler).Methods(method)
			proxy.routes[route] = op
		}
	}
}

func (proxy *Proxy) Middleware(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, req *http.Request) {
		var match mux.RouteMatch
		proxy.router.Match(req, &match)
		if match.Handler == nil {
			proxy.notFound(w, req)
			return
		}

		wr := &WriterRecorder{ResponseWriter: w}
		next.ServeHTTP(wr, req)

		op := proxy.routes[match.Route]
		specResp, ok := op.Responses.StatusCodeResponses[wr.Status()]
		if !ok {
			msg := fmt.Sprintf("Server Status %d not defined by the spec", wr.Status())
			proxy.reporter.Warning(req, op, msg)
			return
		}

		if err := proxy.Validate(wr.Status(), wr.Header(), wr.Body(), &specResp); err != nil {
			proxy.reporter.Error(req, op, err)
		} else {
			proxy.reporter.Success(req, op)
		}
	}
	return http.HandlerFunc(fn)
}

func (proxy *Proxy) newHandler() http.HandlerFunc {
	return proxy.Middleware(proxy.reverseProxy).ServeHTTP
}

func (proxy *Proxy) Validate(status int, header http.Header, body []byte, resp *spec.Response) error {
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return err
	}

	for key, val := range resp.Headers {
		if err := validateHeaderValue(key, header.Get(key), &val); err != nil {
			return err
		}
	}

	// No schema to validate against
	if resp.Schema == nil {
		return nil
	}

	validator := validate.NewSchemaValidator(resp.Schema, proxy.doc, "", strfmt.NewFormats())
	result := validator.Validate(data)
	if result.HasErrors() {
		return result.AsError()
	}

	return nil
}

func validateHeaderValue(key, value string, spec *spec.Header) error {
	if value == "" {
		return fmt.Errorf("%s in headers is missing", key)
	}

	// TODO: Implement the rest of the format validators
	switch spec.Format {
	case "int32":
		_, err := swag.ConvertInt32(value)
		return err
	case "date-time":
		_, err := strfmt.ParseDateTime(value)
		return err
	}
	return nil
}

func getOperations(props *spec.PathItem) map[string]*spec.Operation {
	ops := make(map[string]*spec.Operation)

	if props.Delete != nil {
		ops["DELETE"] = props.Delete
	} else if props.Get != nil {
		ops["GET"] = props.Get
	} else if props.Head != nil {
		ops["HEAD"] = props.Head
	} else if props.Options != nil {
		ops["OPTIONS"] = props.Options
	} else if props.Patch != nil {
		ops["PATCH"] = props.Patch
	} else if props.Post != nil {
		ops["POST"] = props.Post
	} else if props.Put != nil {
		ops["PUT"] = props.Put
	}

	return ops
}
