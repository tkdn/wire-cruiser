package dupapp

type Greeter struct{}

func NewGreeterA() *Greeter {
	return &Greeter{}
}

func NewGreeterB() *Greeter {
	return &Greeter{}
}

type App struct {
	G *Greeter
}

func NewApp(g *Greeter) *App {
	return &App{G: g}
}
