package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
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
		log.Println("SwaggerProxy", version, "listening on", bind, "->", proxy.Target())
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

func sameFiles(a, b string) bool {
	absA, err := filepath.Abs(a)
	if err != nil {
		return false
	}

	absB, err := filepath.Abs(b)
	if err != nil {
		return false
	}

	return absA == absB
}

func reload(proxy *proxy.Proxy, spec string) error {
	doc, err := loads.Spec(spec)
	if err != nil {
		return err
	}

	if err := proxy.SetSpec(doc.Spec()); err != nil {
		return err
	}
	return nil
}

func watchFor(px *proxy.Proxy, spec string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	if err := watcher.Add(filepath.Dir(spec)); err != nil {
		return err
	}
	defer watcher.Close()

	needReload := false
	for {
		select {
		case <-time.After(100 * time.Millisecond):
			if needReload {
				needReload = false
				log.Println("Reloading", spec)
				if err := reload(px, spec); err != nil {
					log.Println(err)
					continue
				}
				needReload = false
			}
		case ev := <-watcher.Events:
			if !sameFiles(ev.Name, spec) {
				continue
			}

			if ev.Op != fsnotify.Write && ev.Op != fsnotify.Chmod {
				continue
			}

			needReload = true
		case <-watcher.Errors:
			// TODO: handle this
		}
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

	go watchFor(proxy, *spec)

	if err := serve(proxy, *bind); err != nil {
		log.Println(err)
	}

	// Report PendingOperations
	fmt.Println("Pending Operations:")
	fmt.Println("------------------")
	for i, op := range proxy.PendingOperations() {
		fmt.Printf("%03d) id=%s\n", i+1, op.ID)
	}
}
