package loader_test

import (
	"path/filepath"
	"testing"

	"github.com/tkdn/wire-cruiser/internal/loader"
)

func testdataDir(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestLoad_PackageDir(t *testing.T) {
	res, err := loader.Load(testdataDir(t, "basic"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Injectors) != 1 {
		t.Fatalf("got %d injectors, want 1", len(res.Injectors))
	}
	inj := res.Injectors[0]
	if got, want := inj.Func.Name.Name, "InitializeApp"; got != want {
		t.Errorf("injector name = %q, want %q", got, want)
	}
	if inj.Build == nil {
		t.Error("injector Build call is nil")
	}
}

func TestLoad_FilePath(t *testing.T) {
	res, err := loader.Load(filepath.Join(testdataDir(t, "basic"), "wire.go"))
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Injectors) != 1 {
		t.Fatalf("got %d injectors, want 1", len(res.Injectors))
	}
}

func TestLoad_NotFound(t *testing.T) {
	if _, err := loader.Load(testdataDir(t, "no_such_dir")); err == nil {
		t.Fatal("want error for missing directory, got nil")
	}
}
