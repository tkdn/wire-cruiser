//go:build wireinject

package structapp

import "github.com/google/wire"

func InitializeApp() *App {
	wire.Build(wire.Struct(new(App), "*"), wire.FieldsOf(new(*Config), "DB"), LoadConfig)
	return nil
}
