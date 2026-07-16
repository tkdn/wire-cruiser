package main_test

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

var binPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "wire-cruiser-e2e")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	binPath = filepath.Join(dir, "wire-cruiser")
	out, err := exec.Command("go", "build", "-o", binPath, ".").CombinedOutput()
	if err != nil {
		panic("build failed: " + string(out))
	}
	os.Exit(m.Run())
}

func run(t *testing.T, args ...string) (stdout, stderr string, exitCode int) {
	t.Helper()
	cmd := exec.Command(binPath, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()
	code := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		code = exitErr.ExitCode()
	} else if err != nil {
		t.Fatal(err)
	}
	return outBuf.String(), errBuf.String(), code
}

func testdataDir(t *testing.T, name string) string {
	t.Helper()
	abs, err := filepath.Abs(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func TestRun_Basic(t *testing.T) {
	stdout, stderr, code := run(t, testdataDir(t, "basic"))
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	if !strings.Contains(stdout, "basic.InitializeApp → *basic.App") {
		t.Errorf("stdout does not contain injector header:\n%s", stdout)
	}
	if stderr != "" {
		t.Errorf("stderr should be empty, got: %s", stderr)
	}
}

func TestRun_MissingProvider(t *testing.T) {
	stdout, stderr, code := run(t, testdataDir(t, "missing"))
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout, "[MISSING] *missingapp.DB") {
		t.Errorf("stdout should still contain the graph:\n%s", stdout)
	}
	if !strings.Contains(stderr, "no provider found for *missingapp.DB") {
		t.Errorf("stderr should contain the warning, got: %s", stderr)
	}
}

func TestRun_MultiInjector(t *testing.T) {
	stdout, _, code := run(t, testdataDir(t, "multi_injector"))
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	for _, want := range []string{"multiapp.InitializeApp", "multiapp.InitializeGreeter"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

// TestRun_Dogfood は wire-cruiser 自身の injector を可視化できることを
// 検証する(カレントディレクトリは cmd/wire-cruiser)。
func TestRun_Dogfood(t *testing.T) {
	stdout, stderr, code := run(t, ".")
	if code != 0 {
		t.Errorf("exit code = %d, want 0 (stderr: %s)", code, stderr)
	}
	for _, want := range []string{
		"main.initializeApp → *main.App",
		"main.provideStdout → main.Stdout",
		"main.provideStderr → main.Stderr",
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout does not contain %q:\n%s", want, stdout)
		}
	}
}

func TestRun_LoadError(t *testing.T) {
	_, stderr, code := run(t, testdataDir(t, "no_such_dir"))
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (stderr: %s)", code, stderr)
	}
	if stderr == "" {
		t.Error("stderr should explain the load error")
	}
}

func TestRun_NoArgs(t *testing.T) {
	_, stderr, code := run(t)
	if code != 2 {
		t.Errorf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr, "usage") && !strings.Contains(stderr, "Usage") {
		t.Errorf("stderr should show usage, got: %s", stderr)
	}
}
