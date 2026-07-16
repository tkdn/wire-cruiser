//go:build wireinject

package multiapp

import "github.com/google/wire"

func InitializeApp() *App {
	wire.Build(NewApp, NewGreeter)
	return nil
}

func InitializeGreeter() *Greeter {
	wire.Build(NewGreeter)
	return nil
}
