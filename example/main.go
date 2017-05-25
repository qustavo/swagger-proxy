package main

import (
	"log"
	"net/http"

	proxy "github.com/gchaincl/swagger-proxy"
	"github.com/go-openapi/loads"
)

func main() {
	doc, err := loads.Spec("swagger.json")
	if err != nil {
		log.Fatal(err)
	}

	p, err := proxy.New(doc.Spec(), &proxy.LogReporter{}, proxy.WithVerbose(true))
	if err != nil {
		log.Fatal(err)
	}

	app := func(w http.ResponseWriter, req *http.Request) {
		if req.Method == "POST" {
			w.WriteHeader(201)
		}
	}
	log.Printf("Server Running")
	http.ListenAndServe(":8989", p.Handler(http.HandlerFunc(app)))
}
