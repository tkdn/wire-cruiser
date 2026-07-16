// Package loader loads a target package with the wireinject build tag
// and locates wire injector declarations in it.
package loader

import (
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// wirePkgPath identifies the wire marker package. Detection matches by
// import path, not by identifier name, to avoid false positives.
const wirePkgPath = "github.com/google/wire"

// Injector is a function declaration that calls wire.Build.
type Injector struct {
	Pkg   *packages.Package
	Func  *ast.FuncDecl
	Build *ast.CallExpr
}

// Result holds the loaded target package and the injectors found in it.
type Result struct {
	Pkg       *packages.Package
	Injectors []Injector
}

// Load interprets path as a .go file or a package directory, loads the
// containing package with -tags=wireinject, and finds all injector
// declarations. A file path narrows detection to that file only.
func Load(path string) (*Result, error) {
	dir := path
	var onlyFile string
	if strings.HasSuffix(path, ".go") {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}
		onlyFile = abs
		dir = filepath.Dir(path)
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("loader: %s is not a directory", dir)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
			packages.NeedSyntax | packages.NeedTypesInfo,
		Dir:        dir,
		BuildFlags: []string{"-tags=wireinject"},
	}
	pkgs, err := packages.Load(cfg, ".")
	if err != nil {
		return nil, fmt.Errorf("loader: %w", err)
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("loader: expected a single package in %s, got %d", dir, len(pkgs))
	}
	pkg := pkgs[0]
	if len(pkg.Syntax) == 0 {
		return nil, fmt.Errorf("loader: no Go source loaded for %s: %v", dir, pkg.Errors)
	}

	res := &Result{Pkg: pkg}
	for _, file := range pkg.Syntax {
		if onlyFile != "" && pkg.Fset.Position(file.Package).Filename != onlyFile {
			continue
		}
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if build := findBuildCall(pkg, fn.Body); build != nil {
				res.Injectors = append(res.Injectors, Injector{Pkg: pkg, Func: fn, Build: build})
			}
		}
	}
	return res, nil
}

// findBuildCall returns the first wire.Build call in body, or nil.
func findBuildCall(pkg *packages.Package, body *ast.BlockStmt) *ast.CallExpr {
	var found *ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		if found != nil {
			return false
		}
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		obj := pkg.TypesInfo.Uses[sel.Sel]
		if obj == nil || obj.Pkg() == nil {
			return true
		}
		if obj.Pkg().Path() == wirePkgPath && obj.Name() == "Build" {
			found = call
			return false
		}
		return true
	})
	return found
}
