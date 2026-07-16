//go:build wireinject

package valueapp

import (
	"io"
	"os"

	"github.com/google/wire"
)

func InitializeApp() *App {
	wire.Build(NewApp, wire.Value(Config{Addr: "localhost"}), wire.InterfaceValue(new(io.Writer), os.Stdout))
	return nil
}
