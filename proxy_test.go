package proxy

import (
	"testing"

	"github.com/go-openapi/loads"
	"github.com/go-openapi/spec"
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
		require.NoError(t, app.Validate(code, []byte("[]"), &resp))
	}
}
