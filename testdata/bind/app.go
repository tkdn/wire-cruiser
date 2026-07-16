package bindapp

type Fooer interface {
	Foo()
}

type FooImpl struct{}

func (*FooImpl) Foo() {}

func NewFooImpl() *FooImpl {
	return &FooImpl{}
}

type App struct {
	F Fooer
}

func NewApp(f Fooer) *App {
	return &App{F: f}
}
