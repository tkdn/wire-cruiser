//go:build wireinject

package missingapp

import "github.com/google/wire"

func InitializeApp() *App {
	wire.Build(NewApp)
	return nil
}
