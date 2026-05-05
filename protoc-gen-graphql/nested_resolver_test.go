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

// TestNestedResolverArgs covers upstream issue
// https://github.com/ysugimoto/grpc-graphql-gateway/issues/83 — fields
// annotated with `(graphql.field).resolver = "..."` should expose the
// resolver target's request fields as arguments on the nested GraphQL
// field, not just type. Locks the behavior in so we don't regress while
// editing the resolver branch of the template.
func TestNestedResolverArgs(t *testing.T) {
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
		"nested_resolver.proto",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("protoc: %v\n%s", err, out)
	}

	src, err := os.ReadFile(filepath.Join(outDir, "nested_resolver.graphql.go"))
	if err != nil {
		t.Fatalf("read generated: %v", err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "nested_resolver.graphql.go", src, parser.AllErrors); err != nil {
		t.Fatalf("generated source does not parse: %v\n%s", err, src)
	}
	got := string(src)

	// The "comments" field on GetPostResponse must surface `after` as a
	// graphql.ArgumentConfig of type graphql.Int — that's what's missing
	// in the issue's reported output.
	mustContain := []string{
		`"comments": &graphql.Field{`,
		`Type: Gql__type_GetCommentsResponse(),`,
		`"after": &graphql.ArgumentConfig{`,
		`Type: graphql.Int,`,
	}
	for _, want := range mustContain {
		if !strings.Contains(got, want) {
			t.Errorf("nested resolver wiring missing: expected substring %q", want)
		}
	}

	// Sanity: the comments-field block specifically must contain the
	// after arg, not just "after" appearing somewhere unrelated.
	commentsBlock := extractField(got, `"comments": &graphql.Field{`)
	if !strings.Contains(commentsBlock, `"after": &graphql.ArgumentConfig{`) {
		t.Errorf("comments field missing `after` argument; block was:\n%s", commentsBlock)
	}
}

// extractField returns the text starting at marker through the matching
// outer brace. Approximate but adequate for the grep-style assertions
// here.
func extractField(src, marker string) string {
	i := strings.Index(src, marker)
	if i < 0 {
		return ""
	}
	depth := 0
	for j := i; j < len(src); j++ {
		switch src[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return src[i : j+1]
			}
		}
	}
	return src[i:]
}
