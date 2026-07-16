# wire-cruiser

A CLI tool that analyzes [google/wire](https://github.com/google/wire) injectors (wire.go files with `//go:build wireinject`) and visualizes the dependency graph from the entry point in your terminal.

When code generation fails, wire reports the offending type but offers no way to see the graph as a whole. wire-cruiser **keeps rendering the graph even for broken configurations** — missing or duplicated providers included — which makes it useful for investigating dependency issues.

## Installation

```console
$ go install github.com/tkdn/wire-cruiser/cmd/wire-cruiser@latest
```

## Usage

```console
$ wire-cruiser <path>
```

`<path>` is either a wire.go file path or a package directory containing injectors.

The CLI assembly of this repository itself is wired with wire (dogfooding). Visualizing itself produces:

```console
$ wire-cruiser ./cmd/wire-cruiser
main.initializeApp → *main.App
└─ main.NewApp → *main.App
   ├─ main.provideStdout → main.Stdout
   └─ main.provideStderr → main.Stderr
```

### Reading the output

| Notation | Meaning |
|------|------|
| `pkg.Func → T` | provider function `pkg.Func` provides type `T` |
| `... → Iface (bind: *Impl)` | the interface is bound to an implementation via wire.Bind |
| `wire.Struct(T)` / `wire.FieldsOf(S.F)` / `wire.Value` / `wire.InterfaceValue` | providers declared with the wire DSL |
| `(arg) T` | a value passed in as an injector argument (leaf of the graph) |
| `[MISSING] T` | no provider found for the type |
| `... (…)` | a shared dependency seen before (children are collapsed after the first occurrence) |

### Exit codes

| code | meaning |
|------|------|
| 0 | success |
| 1 | incomplete graph (missing providers, duplicate providers for the same type, etc. — warnings go to stderr while the graph is still printed) |
| 2 | invalid arguments or package load failure |

## Supported features

- Function providers and `wire.NewSet` (ProviderSet variables in other packages are expanded recursively)
- `wire.Bind` / `wire.Struct` / `wire.FieldsOf` / `wire.Value` / `wire.InterfaceValue`
- Multiple injectors in the same file / package
- Generics are not supported (wire itself never supported them)

## Development

```console
$ go test ./...                                   # run all tests
$ go test ./internal/render/ -update              # regenerate golden files
$ go tool wire ./cmd/wire-cruiser                 # regenerate wire_gen.go
```

Each directory under `testdata/` is an independent Go module holding a resolution case per DSL feature together with its golden file (`want_tree.txt`).
