// Package graph builds a dependency graph from a wire injector by
// recursively expanding wire.Build arguments and provider sets.
package graph

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/tkdn/wire-cruiser/internal/loader"
)

const wirePkgPath = "github.com/google/wire"

type NodeKind int

const (
	KindFunc NodeKind = iota
	KindStruct
	KindFields
	KindValue
	KindInterfaceValue
	KindArg
	KindMissing
)

// Node is a resolved provider in the dependency graph. Nodes for the
// same provided type are shared, so the structure is a DAG.
type Node struct {
	Kind     NodeKind
	Provider string     // 例: "service.NewUserService"、KindMissing では空
	Type     types.Type // 提供型(要求された型)
	Bind     types.Type // wire.Bind 経由の場合の実装型、それ以外は nil
	Deps     []*Node
}

type DiagKind int

const (
	DiagMissing DiagKind = iota
	DiagDuplicate
	DiagUnsupported
	DiagCycle
)

type Diagnostic struct {
	Kind DiagKind
	Msg  string
	Pos  token.Position
}

// Injector is the dependency graph rooted at one injector function.
type Injector struct {
	Name    string
	Pkg     string // 定義パッケージの import パス
	PkgName string // 定義パッケージ名(表示用)
	Args    []types.Type
	Result  types.Type
	Roots   []*Node
	Diags   []Diagnostic
}

// TypeString renders t with package-name qualifiers (e.g. "*app.App").
func TypeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return p.Name() })
}

// key returns the canonical identity of t, using full import paths.
func key(t types.Type) string {
	return types.TypeString(t, nil)
}

// provider is a collected (not yet resolved) provider entry.
type provider struct {
	kind NodeKind
	id   string // dedup identity: func full name or source position
	name string
	deps []types.Type
}

type builder struct {
	root      *packages.Package
	pkgs      map[string]*packages.Package // import path -> package (transitive)
	providers map[string][]*provider       // type key -> candidates
	bindings  map[string]types.Type        // interface key -> implementation type
	args      map[string]types.Type        // injector parameter types
	nodes     map[string][]*Node           // resolution cache (shared DAG nodes)
	visiting  map[string]bool
	diags     []Diagnostic
}

// Build resolves the dependency graph of one injector found by the loader.
func Build(in loader.Injector) (*Injector, error) {
	pkg := in.Pkg
	fn, ok := pkg.TypesInfo.Defs[in.Func.Name].(*types.Func)
	if !ok {
		return nil, fmt.Errorf("graph: no type information for %s", in.Func.Name.Name)
	}
	sig := fn.Type().(*types.Signature)
	if sig.Results().Len() == 0 {
		return nil, fmt.Errorf("graph: injector %s has no return value", fn.Name())
	}
	result := sig.Results().At(0).Type()

	b := &builder{
		root:      pkg,
		pkgs:      map[string]*packages.Package{},
		providers: map[string][]*provider{},
		bindings:  map[string]types.Type{},
		args:      map[string]types.Type{},
		nodes:     map[string][]*Node{},
		visiting:  map[string]bool{},
	}
	b.indexPackages(pkg)

	inj := &Injector{Name: fn.Name(), Pkg: pkg.PkgPath, PkgName: pkg.Name, Result: result}
	for v := range sig.Params().Variables() {
		t := v.Type()
		inj.Args = append(inj.Args, t)
		b.args[key(t)] = t
	}
	for _, arg := range in.Build.Args {
		b.collect(pkg, arg)
	}
	inj.Roots = b.resolve(result)
	inj.Diags = b.diags
	return inj, nil
}

func (b *builder) indexPackages(pkg *packages.Package) {
	if _, ok := b.pkgs[pkg.PkgPath]; ok {
		return
	}
	b.pkgs[pkg.PkgPath] = pkg
	for _, imp := range pkg.Imports {
		b.indexPackages(imp)
	}
}

func (b *builder) diag(kind DiagKind, pos token.Position, format string, a ...any) {
	b.diags = append(b.diags, Diagnostic{Kind: kind, Msg: fmt.Sprintf(format, a...), Pos: pos})
}

func (b *builder) pos(pkg *packages.Package, n ast.Node) token.Position {
	return pkg.Fset.Position(n.Pos())
}

// collect classifies one wire.Build / wire.NewSet argument and registers
// providers and bindings. expr belongs to pkg.
func (b *builder) collect(pkg *packages.Package, expr ast.Expr) {
	switch e := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		name, ok := b.wireCall(pkg, e)
		if !ok {
			b.diag(DiagUnsupported, b.pos(pkg, e), "unsupported expression in provider set")
			return
		}
		switch name {
		case "NewSet":
			for _, a := range e.Args {
				b.collect(pkg, a)
			}
		case "Bind":
			b.collectBind(pkg, e)
		case "Struct":
			b.collectStruct(pkg, e)
		case "FieldsOf":
			b.collectFieldsOf(pkg, e)
		case "Value":
			b.collectValue(pkg, e)
		case "InterfaceValue":
			b.collectInterfaceValue(pkg, e)
		default:
			b.diag(DiagUnsupported, b.pos(pkg, e), "unsupported wire.%s", name)
		}
	case *ast.Ident, *ast.SelectorExpr:
		obj := usedObject(pkg, e)
		switch o := obj.(type) {
		case *types.Func:
			b.addFunc(pkg, e, o)
		case *types.Var:
			if isProviderSet(o.Type()) {
				b.expandSetVar(pkg, e, o)
				return
			}
			b.diag(DiagUnsupported, b.pos(pkg, e), "unsupported variable %s in provider set", o.Name())
		default:
			b.diag(DiagUnsupported, b.pos(pkg, e), "unsupported expression in provider set")
		}
	default:
		b.diag(DiagUnsupported, b.pos(pkg, expr), "unsupported expression in provider set")
	}
}

// wireCall reports whether call invokes a function of the wire package,
// returning its name. Matching is by import path, not identifier text.
func (b *builder) wireCall(pkg *packages.Package, call *ast.CallExpr) (string, bool) {
	obj := usedObject(pkg, call.Fun)
	if obj == nil || obj.Pkg() == nil || obj.Pkg().Path() != wirePkgPath {
		return "", false
	}
	return obj.Name(), true
}

// usedObject returns the object referenced by an identifier or selector.
func usedObject(pkg *packages.Package, expr ast.Expr) types.Object {
	switch e := ast.Unparen(expr).(type) {
	case *ast.Ident:
		return pkg.TypesInfo.Uses[e]
	case *ast.SelectorExpr:
		return pkg.TypesInfo.Uses[e.Sel]
	}
	return nil
}

func isProviderSet(t types.Type) bool {
	named, ok := t.(*types.Named)
	if !ok {
		return false
	}
	obj := named.Obj()
	return obj.Pkg() != nil && obj.Pkg().Path() == wirePkgPath && obj.Name() == "ProviderSet"
}

// expandSetVar locates the wire.NewSet initializer of a ProviderSet
// variable, possibly in another package, and expands it recursively.
func (b *builder) expandSetVar(pkg *packages.Package, ref ast.Expr, v *types.Var) {
	defPkg := b.pkgs[v.Pkg().Path()]
	if defPkg == nil {
		b.diag(DiagUnsupported, b.pos(pkg, ref), "package %s of provider set %s is not loaded", v.Pkg().Path(), v.Name())
		return
	}
	for _, file := range defPkg.Syntax {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if defPkg.TypesInfo.Defs[name] == v && i < len(vs.Values) {
						b.collect(defPkg, vs.Values[i])
						return
					}
				}
			}
		}
	}
	b.diag(DiagUnsupported, b.pos(pkg, ref), "cannot find wire.NewSet definition of %s", v.Name())
}

func (b *builder) addProvider(t types.Type, p *provider) {
	k := key(t)
	for _, exist := range b.providers[k] {
		if exist.id == p.id {
			return // 同一プロバイダが複数の Set 経由で重複到達しただけなら重複扱いにしない
		}
	}
	b.providers[k] = append(b.providers[k], p)
}

func (b *builder) addFunc(pkg *packages.Package, ref ast.Expr, f *types.Func) {
	sig := f.Type().(*types.Signature)
	if sig.Results().Len() == 0 {
		b.diag(DiagUnsupported, b.pos(pkg, ref), "provider %s has no return value", f.Name())
		return
	}
	p := &provider{
		kind: KindFunc,
		id:   f.FullName(),
		name: fmt.Sprintf("%s.%s", f.Pkg().Name(), f.Name()),
	}
	for v := range sig.Params().Variables() {
		p.deps = append(p.deps, v.Type())
	}
	b.addProvider(sig.Results().At(0).Type(), p)
}

// pointee unwraps one pointer level: new(T) has type *T, so the wire DSL
// communicates T as the pointee of its argument's type.
func pointee(t types.Type) (types.Type, bool) {
	ptr, ok := t.(*types.Pointer)
	if !ok {
		return nil, false
	}
	return ptr.Elem(), true
}

func (b *builder) collectBind(pkg *packages.Package, call *ast.CallExpr) {
	if len(call.Args) != 2 {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.Bind needs 2 arguments")
		return
	}
	iface, ok1 := pointee(pkg.TypesInfo.TypeOf(call.Args[0]))
	impl, ok2 := pointee(pkg.TypesInfo.TypeOf(call.Args[1]))
	if !ok1 || !ok2 {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.Bind arguments must be pointers like new(Iface)")
		return
	}
	b.bindings[key(iface)] = impl
}

// structFields returns the fields of st selected by the literal names in
// args; "*" selects every field.
func structFields(st *types.Struct, names []string) []*types.Var {
	var fields []*types.Var
	for f := range st.Fields() {
		for _, n := range names {
			if n == "*" || n == f.Name() {
				fields = append(fields, f)
				break
			}
		}
	}
	return fields
}

// fieldNames extracts string literal arguments (field selectors of
// wire.Struct / wire.FieldsOf).
func fieldNames(args []ast.Expr) []string {
	var names []string
	for _, a := range args {
		lit, ok := ast.Unparen(a).(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			continue
		}
		if s, err := strconv.Unquote(lit.Value); err == nil {
			names = append(names, s)
		}
	}
	return names
}

func namedStruct(t types.Type) (*types.Named, *types.Struct, bool) {
	named, ok := t.(*types.Named)
	if !ok {
		return nil, nil, false
	}
	st, ok := named.Underlying().(*types.Struct)
	if !ok {
		return nil, nil, false
	}
	return named, st, true
}

func (b *builder) collectStruct(pkg *packages.Package, call *ast.CallExpr) {
	if len(call.Args) < 1 {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.Struct needs a struct pointer argument")
		return
	}
	target, ok := pointee(pkg.TypesInfo.TypeOf(call.Args[0]))
	if !ok {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.Struct argument must be like new(T)")
		return
	}
	named, st, ok := namedStruct(target)
	if !ok {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.Struct target %s is not a named struct", TypeString(target))
		return
	}
	p := &provider{
		kind: KindStruct,
		id:   b.pos(pkg, call).String(),
		name: fmt.Sprintf("wire.Struct(%s)", named.Obj().Name()),
	}
	for _, f := range structFields(st, fieldNames(call.Args[1:])) {
		p.deps = append(p.deps, f.Type())
	}
	// wire.Struct(new(T), ...) は T と *T の両方を提供する
	b.addProvider(named, p)
	b.addProvider(types.NewPointer(named), p)
}

func (b *builder) collectFieldsOf(pkg *packages.Package, call *ast.CallExpr) {
	if len(call.Args) < 2 {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.FieldsOf needs a struct pointer and field names")
		return
	}
	// new(S) → 依存は S、new(*S) → 依存は *S
	dep, ok := pointee(pkg.TypesInfo.TypeOf(call.Args[0]))
	if !ok {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.FieldsOf argument must be like new(S)")
		return
	}
	structType := dep
	if elem, ok := pointee(dep); ok {
		structType = elem
	}
	named, st, ok := namedStruct(structType)
	if !ok {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.FieldsOf target %s is not a named struct", TypeString(structType))
		return
	}
	for _, f := range structFields(st, fieldNames(call.Args[1:])) {
		b.addProvider(f.Type(), &provider{
			kind: KindFields,
			id:   b.pos(pkg, call).String() + "." + f.Name(),
			name: fmt.Sprintf("wire.FieldsOf(%s.%s)", named.Obj().Name(), f.Name()),
			deps: []types.Type{dep},
		})
	}
}

func (b *builder) collectValue(pkg *packages.Package, call *ast.CallExpr) {
	if len(call.Args) != 1 {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.Value needs 1 argument")
		return
	}
	t := pkg.TypesInfo.TypeOf(call.Args[0])
	if t == nil {
		b.diag(DiagUnsupported, b.pos(pkg, call), "cannot determine type of wire.Value argument")
		return
	}
	b.addProvider(t, &provider{
		kind: KindValue,
		id:   b.pos(pkg, call).String(),
		name: "wire.Value",
	})
}

func (b *builder) collectInterfaceValue(pkg *packages.Package, call *ast.CallExpr) {
	if len(call.Args) != 2 {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.InterfaceValue needs 2 arguments")
		return
	}
	iface, ok := pointee(pkg.TypesInfo.TypeOf(call.Args[0]))
	if !ok {
		b.diag(DiagUnsupported, b.pos(pkg, call), "wire.InterfaceValue first argument must be like new(Iface)")
		return
	}
	b.addProvider(iface, &provider{
		kind: KindInterfaceValue,
		id:   b.pos(pkg, call).String(),
		name: "wire.InterfaceValue",
	})
}

// resolve returns the shared nodes providing t, creating them on first
// request. Multiple nodes are returned only for duplicate providers.
func (b *builder) resolve(t types.Type) []*Node {
	k := key(t)
	if ns, ok := b.nodes[k]; ok {
		return ns
	}
	if b.visiting[k] {
		b.diag(DiagCycle, token.Position{}, "dependency cycle at %s", TypeString(t))
		return nil
	}
	if at, ok := b.args[k]; ok {
		return b.cache(k, &Node{Kind: KindArg, Provider: "(arg)", Type: at})
	}
	if impl, ok := b.bindings[k]; ok {
		return b.resolveBound(k, t, impl)
	}
	provs := b.providers[k]
	if len(provs) == 0 {
		b.diag(DiagMissing, token.Position{}, "no provider found for %s", TypeString(t))
		return b.cache(k, &Node{Kind: KindMissing, Type: t})
	}
	if len(provs) > 1 {
		var names []string
		for _, p := range provs {
			names = append(names, p.name)
		}
		b.diag(DiagDuplicate, token.Position{}, "multiple providers for %s: %s", TypeString(t), strings.Join(names, ", "))
	}
	b.visiting[k] = true
	defer delete(b.visiting, k)
	var ns []*Node
	for _, p := range provs {
		n := &Node{Kind: p.kind, Provider: p.name, Type: t}
		for _, dt := range p.deps {
			n.Deps = append(n.Deps, b.resolve(dt)...)
		}
		ns = append(ns, n)
	}
	return b.cache(k, ns...)
}

// resolveBound resolves an interface type bound to impl by wire.Bind.
// The resulting node carries the implementation's provider and deps.
func (b *builder) resolveBound(k string, iface, impl types.Type) []*Node {
	b.visiting[k] = true
	implNodes := b.resolve(impl)
	delete(b.visiting, k)
	var ns []*Node
	for _, in := range implNodes {
		ns = append(ns, &Node{Kind: in.Kind, Provider: in.Provider, Type: iface, Bind: impl, Deps: in.Deps})
	}
	if ns == nil {
		ns = []*Node{{Kind: KindMissing, Type: iface, Bind: impl}}
	}
	return b.cache(k, ns...)
}

func (b *builder) cache(k string, ns ...*Node) []*Node {
	b.nodes[k] = ns
	return ns
}
