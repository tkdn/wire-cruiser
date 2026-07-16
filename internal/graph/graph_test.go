package graph_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/tkdn/wire-cruiser/internal/graph"
	"github.com/tkdn/wire-cruiser/internal/loader"
)

type wantNode struct {
	kind     graph.NodeKind
	provider string
	typ      string
	bind     string
	deps     []wantNode
}

func loadCase(t *testing.T, name string) *loader.Result {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	res, err := loader.Load(abs)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Injectors) == 0 {
		t.Fatal("no injectors found")
	}
	return res
}

func buildInjector(t *testing.T, name string) *graph.Injector {
	t.Helper()
	res := loadCase(t, name)
	inj, err := graph.Build(res.Injectors[0])
	if err != nil {
		t.Fatal(err)
	}
	return inj
}

func checkNodes(t *testing.T, path string, got []*graph.Node, want []wantNode) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got %d nodes, want %d", path, len(got), len(want))
	}
	for i, w := range want {
		g := got[i]
		p := fmt.Sprintf("%s/%s", path, w.typ)
		if g.Kind != w.kind {
			t.Errorf("%s: kind = %v, want %v", p, g.Kind, w.kind)
		}
		if g.Provider != w.provider {
			t.Errorf("%s: provider = %q, want %q", p, g.Provider, w.provider)
		}
		if got := graph.TypeString(g.Type); got != w.typ {
			t.Errorf("%s: type = %q, want %q", p, got, w.typ)
		}
		bind := ""
		if g.Bind != nil {
			bind = graph.TypeString(g.Bind)
		}
		if bind != w.bind {
			t.Errorf("%s: bind = %q, want %q", p, bind, w.bind)
		}
		checkNodes(t, p, g.Deps, w.deps)
	}
}

func checkNoDiags(t *testing.T, inj *graph.Injector) {
	t.Helper()
	for _, d := range inj.Diags {
		t.Errorf("unexpected diagnostic: %s", d.Msg)
	}
}

func TestBuild_Basic(t *testing.T) {
	inj := buildInjector(t, "basic")
	if inj.Name != "InitializeApp" {
		t.Errorf("name = %q, want InitializeApp", inj.Name)
	}
	checkNoDiags(t, inj)
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "basic.NewApp", typ: "*basic.App",
		deps: []wantNode{
			{kind: graph.KindFunc, provider: "basic.NewGreeter", typ: "*basic.Greeter"},
		},
	}})
}

func TestBuild_NewSetLocal(t *testing.T) {
	inj := buildInjector(t, "newset_local")
	checkNoDiags(t, inj)
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "nslocal.NewApp", typ: "*nslocal.App",
		deps: []wantNode{
			{kind: graph.KindFunc, provider: "nslocal.NewGreeter", typ: "*nslocal.Greeter"},
		},
	}})
}

func TestBuild_NewSetCrossPackage(t *testing.T) {
	inj := buildInjector(t, "newset_crosspkg")
	checkNoDiags(t, inj)
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "nscross.NewApp", typ: "*nscross.App",
		deps: []wantNode{
			{kind: graph.KindFunc, provider: "service.NewService", typ: "*service.Service"},
		},
	}})
}

func TestBuild_Bind(t *testing.T) {
	inj := buildInjector(t, "bind")
	checkNoDiags(t, inj)
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "bindapp.NewApp", typ: "*bindapp.App",
		deps: []wantNode{
			{kind: graph.KindFunc, provider: "bindapp.NewFooImpl", typ: "bindapp.Fooer", bind: "*bindapp.FooImpl"},
		},
	}})
}

func TestBuild_StructFieldsOf(t *testing.T) {
	inj := buildInjector(t, "struct_fieldsof")
	checkNoDiags(t, inj)
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindStruct, provider: "wire.Struct(App)", typ: "*structapp.App",
		deps: []wantNode{{
			kind: graph.KindFields, provider: "wire.FieldsOf(Config.DB)", typ: "*structapp.DB",
			deps: []wantNode{
				{kind: graph.KindFunc, provider: "structapp.LoadConfig", typ: "*structapp.Config"},
			},
		}},
	}})
}

func TestBuild_ValueInterfaceValue(t *testing.T) {
	inj := buildInjector(t, "value_interfacevalue")
	checkNoDiags(t, inj)
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "valueapp.NewApp", typ: "*valueapp.App",
		deps: []wantNode{
			{kind: graph.KindValue, provider: "wire.Value", typ: "valueapp.Config"},
			{kind: graph.KindInterfaceValue, provider: "wire.InterfaceValue", typ: "io.Writer"},
		},
	}})
}

func TestBuild_Args(t *testing.T) {
	inj := buildInjector(t, "args")
	checkNoDiags(t, inj)
	if len(inj.Args) != 1 {
		t.Fatalf("got %d args, want 1", len(inj.Args))
	}
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "argsapp.NewApp", typ: "*argsapp.App",
		deps: []wantNode{
			{kind: graph.KindArg, provider: "(arg)", typ: "*argsapp.Config"},
		},
	}})
}

func TestBuild_Missing(t *testing.T) {
	inj := buildInjector(t, "missing")
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "missingapp.NewApp", typ: "*missingapp.App",
		deps: []wantNode{
			{kind: graph.KindMissing, provider: "", typ: "*missingapp.DB"},
		},
	}})
	if len(inj.Diags) != 1 || inj.Diags[0].Kind != graph.DiagMissing {
		t.Fatalf("diags = %+v, want one DiagMissing", inj.Diags)
	}
}

func TestBuild_Duplicate(t *testing.T) {
	inj := buildInjector(t, "duplicate")
	checkNodes(t, "roots", inj.Roots, []wantNode{{
		kind: graph.KindFunc, provider: "dupapp.NewApp", typ: "*dupapp.App",
		deps: []wantNode{
			{kind: graph.KindFunc, provider: "dupapp.NewGreeterA", typ: "*dupapp.Greeter"},
			{kind: graph.KindFunc, provider: "dupapp.NewGreeterB", typ: "*dupapp.Greeter"},
		},
	}})
	if len(inj.Diags) != 1 || inj.Diags[0].Kind != graph.DiagDuplicate {
		t.Fatalf("diags = %+v, want one DiagDuplicate", inj.Diags)
	}
}

func TestBuild_MultiInjector(t *testing.T) {
	res := loadCase(t, "multi_injector")
	if len(res.Injectors) != 2 {
		t.Fatalf("got %d injectors, want 2", len(res.Injectors))
	}
	var names []string
	for _, li := range res.Injectors {
		inj, err := graph.Build(li)
		if err != nil {
			t.Fatal(err)
		}
		checkNoDiags(t, inj)
		names = append(names, inj.Name)
	}
	if names[0] != "InitializeApp" || names[1] != "InitializeGreeter" {
		t.Errorf("injector names = %v", names)
	}
}
