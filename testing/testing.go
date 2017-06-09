package testing

import (
	"net/http"

	"github.com/go-openapi/errors"
)

type Validator interface {
	Validate(*http.Request, *http.Response) error
}

type ValidatorFunc func(*http.Request, *http.Response) error

func (fn ValidatorFunc) Validate(req *http.Request, res *http.Response) error {
	return fn(req, res)
}

type Reporter interface {
	Error(args ...interface{})
}

type ReporterFunc func(args ...interface{})

func (fn ReporterFunc) Error(args ...interface{}) {
	fn(args...)
}

type swaggerTransport struct {
	realTransport http.RoundTripper
	validator     Validator
	t             Reporter
}

func (st *swaggerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, respErr := st.realTransport.RoundTrip(req)

	if err := st.validator.Validate(req, resp); err != nil {
		if cErr, ok := err.(*errors.CompositeError); ok {
			for _, err := range cErr.Errors {
				st.t.Error(err)
			}
		} else {
			st.t.Error(err)
		}
	}

	return resp, respErr
}

func NewSwaggerTransport(t Reporter, validator Validator) http.RoundTripper {
	return &swaggerTransport{
		realTransport: http.DefaultTransport,
		validator:     validator,
		t:             t,
	}
}

func NewClient(t Reporter, validator Validator) *http.Client {
	return &http.Client{
		Transport: NewSwaggerTransport(t, validator),
	}
}
