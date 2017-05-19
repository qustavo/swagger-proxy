package main

import (
	"flag"
	"log"
	"net/http"

	proxy "github.com/gchaincl/swagger-proxy"
	"github.com/go-openapi/loads"
)

func main() {
	spec := flag.String("spec", "swagger.yml", "Swagger Spec")
	target := flag.String("target", "http://localhost:4321", "Target")
	verbose := flag.Bool("verbose", false, "Verbose")
	flag.Parse()

	doc, err := loads.Spec(*spec)
	if err != nil {
		log.Fatal(err)
	}

	proxy, err := proxy.New(doc.Spec(), &proxy.LogReporter{},
		proxy.WithTarget(*target),
		proxy.WithVerbose(*verbose),
	)
	if err != nil {
		log.Fatal(err)
	}

	http.ListenAndServe(":1234", proxy)
}
