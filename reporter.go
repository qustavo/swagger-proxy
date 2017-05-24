package proxy

import (
	"fmt"
	"net/http"

	"github.com/fatih/color"
	"github.com/go-openapi/errors"
)

type Reporter interface {
	Success(req *http.Request)
	Error(req *http.Request, err error)
	Warning(req *http.Request, msg string)
	Report()
}

type LogReporter struct {
}

func (r *LogReporter) Success(req *http.Request) {
	fmt.Fprintf(color.Output, "%s %s %s\n",
		color.GreenString("✔"), req.Method, req.URL,
	)
}

func (r *LogReporter) Error(req *http.Request, err error) {
	fmt.Fprintf(color.Output, "%s %s %s\n",
		color.RedString("✗"), req.Method, req.URL,
	)
	if cErr, ok := err.(*errors.CompositeError); ok {
		for i, err := range cErr.Errors {
			fmt.Printf("  %d) %s\n", i+1, err)
		}

	} else {
		fmt.Printf("=> %s\n", err)
	}
}

func (r *LogReporter) Warning(req *http.Request, msg string) {
	fmt.Fprintf(color.Output, "%s %s %s\n",
		color.YellowString("!"), req.Method, req.URL,
	)
	fmt.Printf("  WARNING: %s\n", msg)
}

func (r *LogReporter) Report() {}
