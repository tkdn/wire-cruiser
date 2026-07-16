package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tkdn/wire-cruiser/internal/graph"
	"github.com/tkdn/wire-cruiser/internal/loader"
	"github.com/tkdn/wire-cruiser/internal/render"
)

// Stdout and Stderr distinguish the two writer roles so that wire can
// tell them apart when assembling App.
type (
	Stdout io.Writer
	Stderr io.Writer
)

func provideStdout() Stdout { return os.Stdout }
func provideStderr() Stderr { return os.Stderr }

// App is the CLI application. The graph goes to Out, warnings and
// errors go to Err.
type App struct {
	Out Stdout
	Err Stderr
}

func NewApp(out Stdout, errw Stderr) *App {
	return &App{Out: out, Err: errw}
}

// Run executes the CLI and returns the exit code:
// 0 = ok, 1 = incomplete graph (with warnings), 2 = usage or load error.
func (a *App) Run(args []string) int {
	fs := flag.NewFlagSet("wire-cruiser", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	fs.Usage = func() {
		fmt.Fprintln(a.Err, "usage: wire-cruiser <path>")
		fmt.Fprintln(a.Err, "  <path>  wire.go file or a package directory containing injectors")
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fs.Usage()
		return 2
	}

	res, err := loader.Load(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(a.Err, "wire-cruiser: %v\n", err)
		return 2
	}
	if len(res.Injectors) == 0 {
		fmt.Fprintf(a.Err, "wire-cruiser: no injectors found in %s\n", fs.Arg(0))
		return 2
	}

	code := 0
	var outs []string
	for _, li := range res.Injectors {
		inj, err := graph.Build(li)
		if err != nil {
			fmt.Fprintf(a.Err, "wire-cruiser: %v\n", err)
			return 2
		}
		for _, d := range inj.Diags {
			if d.Pos.IsValid() {
				fmt.Fprintf(a.Err, "warning: %s: %s\n", d.Pos, d.Msg)
			} else {
				fmt.Fprintf(a.Err, "warning: %s\n", d.Msg)
			}
			code = 1
		}
		outs = append(outs, render.Tree(inj))
	}
	fmt.Fprint(a.Out, strings.Join(outs, "\n"))
	return code
}
