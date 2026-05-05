package main

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestCrossPackageQualifiedRef pins the fix for the issue where two
// protos that share the same trailing Go package name (e.g. both
// ending with `;v1`) caused field references to drop the package
// qualifier — yielding `Gql__type_Test()` plus an unused import
// instead of `v1.Gql__type_Test()`.
func TestCrossPackageQualifiedRef(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc not found on PATH")
	}
	if _, err := exec.LookPath("protoc-gen-go"); err != nil {
		t.Skip("protoc-gen-go not found on PATH")
	}
	wkProtoInclude := findProtoInclude(t)

	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)
	repoRoot := filepath.Dir(pkgDir)
	graphqlInclude := filepath.Join(repoRoot, "include", "graphql")
	testdata := filepath.Join(pkgDir, "testdata")

	tmp := t.TempDir()
	plugin := filepath.Join(tmp, "protoc-gen-graphql")
	build := exec.Command("go", "build", "-o", plugin, ".")
	build.Dir = pkgDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building plugin: %v\n%s", err, out)
	}

	outDir := t.TempDir()
	cmd := exec.Command(protoc,
		"-I", testdata,
		"-I", wkProtoInclude,
		"-I", graphqlInclude,
		"--plugin=protoc-gen-graphql="+plugin,
		"--go_out="+outDir,
		"--go_opt=paths=source_relative",
		"--graphql_out="+outDir,
		"--graphql_opt=paths=source_relative",
		"cross_pkg/a.proto",
		"cross_pkg/b.proto",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, out)
	}

	src, err := os.ReadFile(filepath.Join(outDir, "cross_pkg", "a.graphql.go"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "a.graphql.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, src)
	}
	got := string(src)

	// The cross-package reference must be qualified by the imported
	// package's alias (`v1`) — otherwise the import is unused and the
	// identifier is unresolved.
	if !strings.Contains(got, "v1.Gql__type_Test()") {
		t.Errorf("expected qualified cross-package ref `v1.Gql__type_Test()` in:\n%s", got)
	}
	if strings.Contains(got, "(Gql__type_Test())") {
		t.Errorf("found unqualified `Gql__type_Test()` — package qualifier was dropped:\n%s", got)
	}
	if !strings.Contains(got, `v1 "example.com/repo/gen/v1/b"`) {
		t.Errorf("expected import alias for cross-package dependency in:\n%s", got)
	}
}

// TestCrossPackageAliasCollision pins the case where two distinct
// imports share the same trailing Go package name. The generator must
// rewrite their import aliases (and the field-level qualifiers that
// reference them) to keep the generated file compilable.
func TestCrossPackageAliasCollision(t *testing.T) {
	protoc, err := exec.LookPath("protoc")
	if err != nil {
		t.Skip("protoc not found on PATH")
	}
	if _, err := exec.LookPath("protoc-gen-go"); err != nil {
		t.Skip("protoc-gen-go not found on PATH")
	}
	wkProtoInclude := findProtoInclude(t)

	_, thisFile, _, _ := runtime.Caller(0)
	pkgDir := filepath.Dir(thisFile)
	repoRoot := filepath.Dir(pkgDir)
	graphqlInclude := filepath.Join(repoRoot, "include", "graphql")
	testdata := filepath.Join(pkgDir, "testdata")

	tmp := t.TempDir()
	plugin := filepath.Join(tmp, "protoc-gen-graphql")
	build := exec.Command("go", "build", "-o", plugin, ".")
	build.Dir = pkgDir
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building plugin: %v\n%s", err, out)
	}

	outDir := t.TempDir()
	cmd := exec.Command(protoc,
		"-I", testdata,
		"-I", wkProtoInclude,
		"-I", graphqlInclude,
		"--plugin=protoc-gen-graphql="+plugin,
		"--go_out="+outDir,
		"--go_opt=paths=source_relative",
		"--graphql_out="+outDir,
		"--graphql_opt=paths=source_relative",
		"cross_pkg_alias/a.proto",
		"cross_pkg_alias/b.proto",
		"cross_pkg_alias/c.proto",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, out)
	}

	src, err := os.ReadFile(filepath.Join(outDir, "cross_pkg_alias", "a.graphql.go"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "a.graphql.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, src)
	}
	got := string(src)

	// Both imports share trailing name `v1`; the generator picks
	// disambiguated aliases derived from path segments.
	wantImports := []string{
		`v1_b "example.com/repo/alias/v1/b"`,
		`v1_c "example.com/repo/alias/v1/c"`,
	}
	for _, want := range wantImports {
		if !strings.Contains(got, want) {
			t.Errorf("missing import line %q in:\n%s", want, got)
		}
	}

	wantRefs := []string{
		"v1_b.Gql__type_BType()",
		"v1_c.Gql__type_CType()",
	}
	for _, want := range wantRefs {
		if !strings.Contains(got, want) {
			t.Errorf("missing qualified ref %q in:\n%s", want, got)
		}
	}
}
