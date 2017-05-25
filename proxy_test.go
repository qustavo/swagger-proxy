package proxy

import (
	"net/http"
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
