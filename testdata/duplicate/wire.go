//go:build wireinject

package dupapp

import "github.com/google/wire"

func InitializeApp() *App {
	wire.Build(NewApp, NewGreeterA, NewGreeterB)
	return nil
}
