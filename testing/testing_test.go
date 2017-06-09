package testing

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-openapi/errors"
	"github.com/stretchr/testify/assert"
)

func foo(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
}

type testReporter struct {
	N    int
	Args [][]interface{}
}

func newTestReporter() *testReporter {
	return &testReporter{
		N:    0,
		Args: make([][]interface{}, 0),
	}
}

func (tr *testReporter) Error(args ...interface{}) {
	tr.N++
	tr.Args = append(tr.Args, args)
}

func TestValidation(t *testing.T) {
	noErrValidator := ValidatorFunc(func(_ *http.Request, _ *http.Response) error {
		return nil
	})

	simpleErr := fmt.Errorf("simple_error")
	simpleErrValidator := ValidatorFunc(func(_ *http.Request, _ *http.Response) error {
		return simpleErr
	})

	compositeErrs := []error{
		fmt.Errorf("composite_error_1"),
		fmt.Errorf("composite_error_2"),
		fmt.Errorf("composite_error_3"),
	}

	compositeErrValidator := ValidatorFunc(func(_ *http.Request, _ *http.Response) error {
		return errors.CompositeValidationError(compositeErrs...)
	})

	srv := httptest.NewServer(http.HandlerFunc(foo))

	t.Run("No errors", func(t *testing.T) {
		tr := newTestReporter()
		c := NewClient(tr, noErrValidator)
		c.Get(srv.URL)
		assert.Equal(t, 0, tr.N)
	})

	t.Run("Simple error", func(t *testing.T) {
		tr := newTestReporter()
		c := NewClient(tr, simpleErrValidator)
		c.Get(srv.URL)
		assert.Equal(t, 1, tr.N)
		assert.Equal(t, simpleErr.Error(), tr.Args[0][0].(error).Error())
	})

	t.Run("Composite error", func(t *testing.T) {
		tr := newTestReporter()
		c := NewClient(tr, compositeErrValidator)
		c.Get(srv.URL)
		assert.Equal(t, 3, tr.N)
		for i, err := range compositeErrs {
			assert.Equal(t, err.Error(), tr.Args[i][0].(error).Error())
		}

	})
}
