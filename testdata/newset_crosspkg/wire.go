//go:build wireinject

package nscross

import (
	"github.com/google/wire"

	"example.com/nscross/service"
)

func InitializeApp() *App {
	wire.Build(NewApp, service.Set)
	return nil
}
