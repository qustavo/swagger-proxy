package integration

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	proxy "github.com/gchaincl/swagger-proxy"
	"github.com/go-openapi/loads"
	"github.com/stretchr/testify/require"
)

func TestMiddleware(t *testing.T) {
	doc, err := loads.Spec("../fixtures/petstore.json")
	require.NoError(t, err)

	p, err := proxy.New(doc.Spec(), &proxy.LogReporter{})
	require.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.Write([]byte("{}"))
	})

	srv := httptest.NewServer(p.Handler(handler))
	defer srv.Close()

	for path, item := range doc.Spec().Paths.Paths {
		url := srv.URL + doc.Spec().BasePath + path
		url = strings.Replace(url, "{", "", -1)
		url = strings.Replace(url, "}", "", -1)
		if item.Get != nil {
			http.Get(url)
		}
	}
}
