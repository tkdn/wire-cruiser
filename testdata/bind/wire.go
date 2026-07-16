//go:build wireinject

package bindapp

import "github.com/google/wire"

func InitializeApp() *App {
	wire.Build(NewApp, NewFooImpl, wire.Bind(new(Fooer), new(*FooImpl)))
	return nil
}
