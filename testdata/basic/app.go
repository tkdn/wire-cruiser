package basic

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
