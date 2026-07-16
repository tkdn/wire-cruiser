//go:build wireinject

package nslocal

import "github.com/google/wire"

func InitializeApp() *App {
	wire.Build(Set)
	return nil
}
