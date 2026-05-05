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

// TestOneofGeneration runs the plugin against testdata/oneof.proto and
// asserts that the generated .graphql.go has working oneof wiring on
// both the input (wrapper construction) and output (per-variant
// Resolve) sides. Skipped when protoc / protoc-gen-go are not on PATH;
// `make doctor` shows how to install them.
func TestOneofGeneration(t *testing.T) {
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
		"--graphql_out="+outDir,
		"oneof.proto",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, out)
	}

	src, err := os.ReadFile(filepath.Join(outDir, "oneof.graphql.go"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}

	if _, err := parser.ParseFile(token.NewFileSet(), "oneof.graphql.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, src)
	}

	got := string(src)

	// Output side: per-variant Resolve must emit type-asserts so the
	// right oneof member shows up; graphql-go's reflection can't walk
	// the wrapper interface on its own.
	mustContain := map[string]string{
		"name variant resolver":   `src.GetKey().(*LookupRequest_Name)`,
		"id variant resolver":     `src.GetKey().(*LookupRequest_Id)`,
		"text variant resolver":   `src.GetBody().(*LookupReply_Text)`,
		"number variant resolver": `src.GetBody().(*LookupReply_Number)`,
	}
	for label, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("missing %s — expected substring %q in generated source", label, want)
		}
	}

	// Input side: the request is unmarshaled via runtime.MarshalRequest,
	// which delegates to protojson for proto.Message values. The
	// generator must NOT emit hand-rolled wrapper construction — that
	// would diverge from the canonical proto3 JSON behaviour for bytes,
	// enums, and nested messages, and would silently last-write-wins on
	// multiple variants instead of erroring.
	mustNotContain := map[string]string{
		"hand-rolled name wrapper": `&LookupRequest_Name{`,
		"hand-rolled id wrapper":   `&LookupRequest_Id{`,
	}
	for label, banned := range mustNotContain {
		if strings.Contains(got, banned) {
			t.Errorf("%s present — generator should defer oneof input to protojson, not emit %q",
				label, banned)
		}
	}
}

func findProtoInclude(t *testing.T) string {
	t.Helper()
	for _, c := range []string{"/usr/local/include", "/usr/include"} {
		if _, err := os.Stat(filepath.Join(c, "google", "protobuf", "descriptor.proto")); err == nil {
			return c
		}
	}
	if out, err := exec.Command("brew", "--prefix", "protobuf").Output(); err == nil {
		c := filepath.Join(strings.TrimSpace(string(out)), "include")
		if _, err := os.Stat(filepath.Join(c, "google", "protobuf", "descriptor.proto")); err == nil {
			return c
		}
	}
	t.Skip("google/protobuf/descriptor.proto not found in any standard include path")
	return ""
}
