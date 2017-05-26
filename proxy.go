package proxy

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-openapi/errors"
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

	doc  interface{} // This is useful for validate (TODO: find a better way)
	spec *spec.Swagger
}

type ProxyOpt func(*Proxy)

func WithTarget(target string) ProxyOpt { return func(proxy *Proxy) { proxy.target = target } }
func WithVerbose(v bool) ProxyOpt       { return func(proxy *Proxy) { proxy.verbose = v } }

func New(s *spec.Swagger, reporter Reporter, opts ...ProxyOpt) (*Proxy, error) {
	proxy := &Proxy{
		target:   "http://localhost:8080",
		router:   mux.NewRouter(),
		routes:   make(map[*mux.Route]*spec.Operation),
		reporter: reporter,
	}

	if err := proxy.SetSpec(s); err != nil {
		return nil, err
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
	proxy.registerPaths()

	return proxy, nil
}

func (proxy *Proxy) SetSpec(spec *spec.Swagger) error {
	// validate.NewSchemaValidator requires the spec as an interface{}
	// That's why we Unmarshal(Marshal()) the document
	data, err := json.Marshal(spec)
	if err != nil {
		return err
	}

	var doc interface{}
	if err := json.Unmarshal(data, &doc); err != nil {
		return err
	}

	proxy.doc = doc
	proxy.spec = spec

	proxy.registerPaths()
	return nil
}

func (proxy *Proxy) Router() http.Handler {
	return proxy.router
}

func (proxy *Proxy) registerPaths() {
	base := proxy.spec.BasePath
	paths := proxy.spec.Paths

	router := mux.NewRouter()
	for path, item := range paths.Paths {
		// Register every spec operation under a newHandler
		for method, op := range getOperations(&item) {
			newPath := base + path
			if proxy.verbose {
				log.Printf("Register %s %s", method, newPath)
			}
			route := router.Handle(newPath, proxy.newHandler()).Methods(method)
			proxy.routes[route] = op
		}
	}
	*proxy.router = *router
}

func (proxy *Proxy) notFound(w http.ResponseWriter, req *http.Request) {
	proxy.reporter.Warning(req, "Route not defined on the Spec")
	proxy.reverseProxy.ServeHTTP(w, req)
}

func (proxy *Proxy) newHandler() http.Handler {
	return proxy.Handler(proxy.reverseProxy)
}
func (proxy *Proxy) Handler(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, req *http.Request) {
		wr := &WriterRecorder{ResponseWriter: w}
		next.ServeHTTP(wr, req)

		var match mux.RouteMatch
		proxy.router.Match(req, &match)
		op := proxy.routes[match.Route]

		if match.Handler == nil || op == nil {
			proxy.reporter.Warning(req, "Route not defined on the Spec")
			// Route hasn't been registered on the muxer
			return
		}

		if err := proxy.Validate(wr, op); err != nil {
			proxy.reporter.Error(req, err)
		} else {
			proxy.reporter.Success(req)
		}
	}
	return http.HandlerFunc(fn)
}

type validatorFunc func(Response, *spec.Operation) error

func (proxy *Proxy) Validate(resp Response, op *spec.Operation) error {
	if _, ok := op.Responses.StatusCodeResponses[resp.Status()]; !ok {
		return fmt.Errorf("Server Status %d not defined by the spec", resp.Status())
	}

	var validators = []validatorFunc{
		proxy.ValidateMIME,
		proxy.ValidateHeaders,
		proxy.ValidateBody,
	}

	var errs []error
	for _, v := range validators {
		if err := v(resp, op); err != nil {
			if cErr, ok := err.(*errors.CompositeError); ok {
				errs = append(errs, cErr.Errors...)
			} else {
				errs = append(errs, err)
			}
		}
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.CompositeValidationError(errs...)
}

func (proxy *Proxy) ValidateMIME(resp Response, op *spec.Operation) error {
	// Use Operation Spec or fallback to root
	produces := op.Produces
	if len(produces) == 0 {
		produces = proxy.spec.Produces
	}

	ct := resp.Header().Get("Content-Type")
	if len(produces) == 0 {
		return nil
	}

	for _, mime := range produces {
		if ct == mime {
			return nil
		}
	}

	return fmt.Errorf("Content-Type Error: Should produce %q, but got: '%s'", produces, ct)
}

func (proxy *Proxy) ValidateHeaders(resp Response, op *spec.Operation) error {
	var errs []error

	r := op.Responses.StatusCodeResponses[resp.Status()]
	for key, spec := range r.Headers {
		if err := validateHeaderValue(key, resp.Header().Get(key), &spec); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) == 0 {
		return nil
	}

	return errors.CompositeValidationError(errs...)
}

func (proxy *Proxy) ValidateBody(resp Response, op *spec.Operation) error {
	r := op.Responses.StatusCodeResponses[resp.Status()]
	if r.Schema == nil {
		return nil
	}

	var data interface{}
	if err := json.Unmarshal(resp.Body(), &data); err != nil {
		return err
	}

	v := validate.NewSchemaValidator(r.Schema, proxy.doc, "", strfmt.Default)
	if result := v.Validate(data); result.HasErrors() {
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
	ops := map[string]*spec.Operation{
		"DELETE":  props.Delete,
		"GET":     props.Get,
		"HEAD":    props.Head,
		"OPTIONS": props.Options,
		"PATCH":   props.Patch,
		"POST":    props.Post,
		"PUT":     props.Put,
	}

	// Keep those != nil
	for key, op := range ops {
		if op == nil {
			delete(ops, key)
		}
	}

	return ops
}
