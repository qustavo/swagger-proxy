package proxy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"

	"github.com/go-openapi/spec"
	"github.com/go-openapi/strfmt"
	"github.com/go-openapi/validate"
	"github.com/gorilla/mux"
)

type Proxy struct {
	// Opts
	target  string
	verbose bool

	router   *mux.Router
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
		reporter: reporter,
		doc:      doc,
	}

	for _, opt := range opts {
		opt(proxy)
	}

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
			handler := proxy.newHandler(op)
			newPath := base + path
			if proxy.verbose {
				log.Printf("Register %s %s", method, newPath)
			}
			proxy.router.HandleFunc(newPath, handler).Methods(method)
		}
	}
}

func (proxy *Proxy) newHandler(op *spec.Operation) http.HandlerFunc {
	fn := func(w http.ResponseWriter, req *http.Request) error {
		rpURL, err := url.Parse(proxy.target)
		if err != nil {
			return err
		}
		rp := httputil.NewSingleHostReverseProxy(rpURL)

		// We use ModifyResponse as a hook to inspect the response,
		// it never gets modified.
		rp.ModifyResponse = func(resp *http.Response) error {
			// Read from an io.ReadWriter without losing its content
			buf, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return err
			}
			defer resp.Body.Close()
			resp.Body = ioutil.NopCloser(bytes.NewBuffer(buf))

			// Find the associated Defined Response out of the response status
			specResp, ok := op.Responses.StatusCodeResponses[resp.StatusCode]
			if !ok {
				msg := fmt.Sprintf("Server Status %d not defined by the spec", resp.StatusCode)
				proxy.reporter.Warning(req, op, msg)
				return nil
			}

			if err := proxy.Validate(resp.StatusCode, buf, &specResp); err != nil {
				proxy.reporter.Error(req, op, err)
			} else {
				proxy.reporter.Success(req, op)
			}
			return nil
		}
		rp.ServeHTTP(w, req)

		return nil
	}

	return func(w http.ResponseWriter, req *http.Request) {
		if err := fn(w, req); err != nil {
			log.Printf("err = %+v\n", err)
		}
	}
}

func (proxy *Proxy) Validate(status int, body []byte, resp *spec.Response) error {
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return err
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

func copyHeaders(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
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
