package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testResponse struct {
	status int
	header http.Header
	body   []byte
}

func (t *testResponse) Status() int         { return t.status }
func (t *testResponse) Header() http.Header { return t.header }
func (t *testResponse) Body() []byte        { return t.body }

type testReporter struct {
	success  []*http.Request
	errors   []error
	warnings []string
}

func (t *testReporter) Success(req *http.Request) {
	t.success = append(t.success, req)
}

func (t *testReporter) Error(req *http.Request, err error) {
	t.errors = append(t.errors, err)
}

func (t *testReporter) Warning(req *http.Request, msg string) {
	t.warnings = append(t.warnings, msg)
}

func (t *testReporter) Report() {
	panic("not implemented")
}

func openFixture(t *testing.T, name string) *spec.Swagger {
	doc, err := loads.Spec("./fixtures/" + name)
	require.NoError(t, err)
	return doc.Spec()
}

func TestValidation(t *testing.T) {
	spec := openFixture(t, "petstore.json")
	app, err := New(spec, nil)
	require.NoError(t, err)

	op := spec.Paths.Paths["/pet/findByStatus"].Get
	resp := &testResponse{
		header: http.Header{},
		body:   []byte("[]"),
	}
	resp.Header().Set("Content-Type", "application/json")
	for _, status := range []int{200, 400} {
		resp.status = status
		assert.NoError(t, app.Validate(resp, op))
	}

	resp.status = 999
	assert.Error(t, app.Validate(resp, op))
}

func TestHeaderValidation(t *testing.T) {
	swagger := openFixture(t, "petstore.json")
	proxy, err := New(swagger, nil)
	require.NoError(t, err)

	op := swagger.Paths.Paths["/user/login"].Get
	resp := &testResponse{
		status: 200,
		header: http.Header{},
	}

	t.Run("Valid Headers", func(t *testing.T) {
		resp.Header().Set("X-Rate-Limit", "32")
		resp.Header().Set("X-Expires-After", time.Now().Format(time.RFC3339))

		assert.NoError(t, proxy.ValidateHeaders(resp, op))
	})

	t.Run("Invalid Headers", func(t *testing.T) {
		resp.Header().Set("X-Rate-Limit", "NaN")
		resp.Header().Set("X-Expires-After", "Not a date-time")

		assert.Error(t, proxy.ValidateHeaders(resp, op))
	})
}

func TestProducesDefinition(t *testing.T) {
	swagger := openFixture(t, "petstore.json")
	app, err := New(swagger, nil)
	require.NoError(t, err)

	resp := &testResponse{header: http.Header{}}
	op := swagger.Paths.Paths["/pet"].Post
	assert.Error(t, app.ValidateMIME(resp, op),
		"Content-Type: application/json is missing")

	resp.Header().Set("Content-Type", "application/json")
	assert.NoError(t, app.ValidateMIME(resp, op),
		"Content-Type: application/json is present")

	t.Run("IsInheritedFromRoot", func(t *testing.T) {
		// MOVE `Produces` property to root
		swagger.Produces = []string{"application/json"}
		op.Produces = []string{}

		resp.Header().Del("Content-Type")
		assert.Error(t, app.ValidateMIME(resp, op),
			"Content-Type: application/json is missing")

		resp.Header().Set("Content-Type", "application/json")
		assert.NoError(t, app.ValidateMIME(resp, op),
			"Content-Type: application/json is present")
	})
}

func TestHTTPHandler(t *testing.T) {
	swagger := openFixture(t, "petstore.json")
	reporter := &testReporter{}
	app, err := New(swagger, reporter)
	require.NoError(t, err)

	fn := func(w http.ResponseWriter, req *http.Request) {
		if req.URL.String() == "/v2/store/inventory" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte("{}"))
		}
	}

	srv := httptest.NewServer(app.Handler(http.HandlerFunc(fn)))
	defer srv.Close()

	for _, url := range []string{
		"/v2/store/inventory",   // SUCCESS
		"/v2/pet/findByStatus",  // ERROR
		"/not_a_registered_url", // WARNING
	} {
		http.Get(srv.URL + url)
	}

	assert.Equal(t, 1, len(reporter.success))
	assert.Equal(t, 1, len(reporter.errors))
	assert.Equal(t, 1, len(reporter.warnings))
}

func TestPendingOperations(t *testing.T) {
	swagger := openFixture(t, "petstore.json")
	app, err := New(swagger, &testReporter{})
	require.NoError(t, err)

	// Dummy server using Handler as middleware
	srv := httptest.NewServer(app.Handler(
		http.HandlerFunc(
			func(w http.ResponseWriter, req *http.Request) {
			},
		),
	))
	defer srv.Close()

	pending := len(app.PendingOperations())

	// Count defines responses
	var opsCounter int
	WalkOps(swagger, func(path, meth string, op *spec.Operation) {
		opsCounter += 1
	})
	require.Equal(t, opsCounter, pending)

	t.Run("AreRemoved", func(t *testing.T) {
		http.Get(srv.URL + "/v2/pet/findByStatus")
		assert.Equal(t, pending-1, len(app.PendingOperations()))

		http.Post(srv.URL+"/v2/pet", "", nil)
		assert.Equal(t, pending-2, len(app.PendingOperations()))

		pending = len(app.PendingOperations())
	})

	t.Run("AreNotRemovedWhenPathNotFound", func(t *testing.T) {
		http.Get(srv.URL + "/not_and_endpoint")
		assert.Equal(t, pending, len(app.PendingOperations()))
	})

	t.Run("AreRemovedOnceWhenCalledNTimes", func(t *testing.T) {
		http.Get(srv.URL + "/v2/store/inventory")
		http.Get(srv.URL + "/v2/store/inventory")
		http.Get(srv.URL + "/v2/store/inventory")
		assert.Equal(t, pending-1, len(app.PendingOperations()))
	})
}
