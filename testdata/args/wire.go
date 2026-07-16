//go:build wireinject

package argsapp

import "github.com/google/wire"

func InitializeApp(c *Config) *App {
	wire.Build(NewApp)
	return nil
}
