package argsapp

type Config struct{}

type App struct {
	C *Config
}

func NewApp(c *Config) *App {
	return &App{C: c}
}
