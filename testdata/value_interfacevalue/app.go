package valueapp

import "io"

type Config struct {
	Addr string
}

type App struct {
	C Config
	W io.Writer
}

func NewApp(c Config, w io.Writer) *App {
	return &App{C: c, W: w}
}
