//go:build wireinject

package basic

import "github.com/google/wire"

func InitializeApp() *App {
	wire.Build(NewApp, NewGreeter)
	return nil
}
