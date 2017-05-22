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

func openFixture(t *testing.T, name string) *spec.Swagger {
	doc, err := loads.Spec("./fixtures/" + name)
	require.NoError(t, err)
	return doc.Spec()
}

func TestValidation(t *testing.T) {
	spec := openFixture(t, "petstore.json")
	app, err := New(spec, nil)
	require.NoError(t, err)

	rr := spec.Paths.Paths["/pet/findByStatus"].Get.Responses
	for _, code := range []int{200, 400} {
		resp, ok := rr.StatusCodeResponses[code]
		require.True(t, ok)
		require.NoError(t, app.Validate(code, nil, []byte("[]"), &resp))
	}
}

func TestHeaderValidation(t *testing.T) {
	doc := openFixture(t, "petstore.json")
	spec := doc.Paths.Paths["/user/login"].Get.
		Responses.StatusCodeResponses[200].Headers
	header := http.Header{}

	t.Run("Valid Headers", func(t *testing.T) {
		header.Set("X-Rate-Limit", "32")
		header.Set("X-Expires-After", time.Now().Format(time.RFC3339))

		for key, val := range spec {
			err := validateHeaderValue(key, header.Get(key), &val)
			assert.NoError(t, err, "For header "+key)
		}
	})

	t.Run("Invalid Headers", func(t *testing.T) {
		header.Set("X-Rate-Limit", "NaN")
		header.Set("X-Expires-After", "Not a date-time")

		for key, val := range spec {
			err := validateHeaderValue(key, header.Get(key), &val)
			assert.Error(t, err, "For header "+key)
		}
	})

	t.Run("Missing Headers key", func(t *testing.T) {
		for key, val := range spec {
			err := validateHeaderValue(key, "", &val)
			assert.Error(t, err, "For header "+key)
			assert.Equal(t, key+" in headers is missing", err.Error())
		}
	})

}
