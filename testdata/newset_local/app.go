package nslocal

import "github.com/google/wire"

type Greeter struct{}

func NewGreeter() *Greeter {
	return &Greeter{}
}

type App struct {
	G *Greeter
}

func NewApp(g *Greeter) *App {
	return &App{G: g}
}

var Set = wire.NewSet(NewApp, NewGreeter)
