package main

import (
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"

	proxy "github.com/gchaincl/swagger-proxy"
	"github.com/go-openapi/loads"
)

const version = "v0.0.1"

func serve(proxy *proxy.Proxy, bind string) error {
	s := http.Server{
		Addr:    bind,
		Handler: proxy.Router(),
	}

	errC := make(chan error)
	go func() {
		log.Println("SwaggerProxy", version, "listening on", bind)
		errC <- s.ListenAndServe()
	}()

	sigC := make(chan os.Signal, 1)
	signal.Notify(sigC, os.Interrupt)

	select {
	case err := <-errC:
		return err
	case s := <-sigC:
		log.Printf("%s", s)
		return nil
	}
}

func main() {
	bind := flag.String("bind", ":1234", "Bind Address")
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

	if err := serve(proxy, *bind); err != nil {
		log.Println(err)
	}
}
