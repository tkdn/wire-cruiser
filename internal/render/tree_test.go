package render_test

import (
	"flag"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tkdn/wire-cruiser/internal/graph"
	"github.com/tkdn/wire-cruiser/internal/loader"
	"github.com/tkdn/wire-cruiser/internal/render"
)

var update = flag.Bool("update", false, "update golden files")

var goldenCases = []string{
	"basic",
	"newset_local",
	"newset_crosspkg",
	"bind",
	"struct_fieldsof",
	"value_interfacevalue",
	"args",
	"missing",
	"duplicate",
	"multi_injector",
}

func TestTree_Golden(t *testing.T) {
	for _, name := range goldenCases {
		t.Run(name, func(t *testing.T) {
			dir, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			res, err := loader.Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			var outs []string
			for _, li := range res.Injectors {
				inj, err := graph.Build(li)
				if err != nil {
					t.Fatal(err)
				}
				outs = append(outs, render.Tree(inj))
			}
			got := strings.Join(outs, "\n")

			golden := filepath.Join(dir, "want_tree.txt")
			if *update {
				if err := os.WriteFile(golden, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}
			if got != string(want) {
				t.Errorf("tree output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}
		})
	}
}

// TestTree_SharedSubtree は共有依存の 2 回目以降の出現が子の展開を
// 省略して (…) と表示されることを合成ノードで検証する。
func TestTree_SharedSubtree(t *testing.T) {
	leaf := &graph.Node{Kind: graph.KindFunc, Provider: "db.NewDB", Type: types.Typ[types.Int]}
	shared := &graph.Node{Kind: graph.KindFunc, Provider: "config.Load", Type: types.Typ[types.String], Deps: []*graph.Node{leaf}}
	a := &graph.Node{Kind: graph.KindFunc, Provider: "svc.NewA", Type: types.Typ[types.Bool], Deps: []*graph.Node{shared}}
	b := &graph.Node{Kind: graph.KindFunc, Provider: "svc.NewB", Type: types.Typ[types.Float64], Deps: []*graph.Node{shared}}
	root := &graph.Node{Kind: graph.KindFunc, Provider: "app.NewApp", Type: types.Typ[types.Complex128], Deps: []*graph.Node{a, b}}
	inj := &graph.Injector{
		Name: "InitializeApp", PkgName: "app",
		Result: types.Typ[types.Complex128],
		Roots:  []*graph.Node{root},
	}

	want := strings.Join([]string{
		"app.InitializeApp → complex128",
		"└─ app.NewApp → complex128",
		"   ├─ svc.NewA → bool",
		"   │  └─ config.Load → string",
		"   │     └─ db.NewDB → int",
		"   └─ svc.NewB → float64",
		"      └─ config.Load → string (…)",
		"",
	}, "\n")
	if got := render.Tree(inj); got != want {
		t.Errorf("tree output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}
