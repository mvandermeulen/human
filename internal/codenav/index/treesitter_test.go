package index

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func writePolyglot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"py/app.py":   "def handler():\n    helper()\n\ndef helper():\n    return 1\n",
		"web/app.ts":  "export function route() { dispatch(); }\nfunction dispatch() {}\n",
		"skip/big.go": "package skip\n\nfunc Ignored() {}\n", // Go is GoNative's job; TreeSitter must skip it
	}
	for name, body := range files {
		p := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func TestTreeSitterIndex(t *testing.T) {
	dir := writePolyglot(t)
	sink := newCollectSink()
	if err := (TreeSitter{}).Index(context.Background(), RepoScan{Project: "poly", Root: dir}, sink); err != nil {
		t.Fatal(err)
	}

	wantSyms := []string{
		"py/app.py:handler", "py/app.py:helper",
		"web/app.ts:route", "web/app.ts:dispatch",
	}
	for _, qn := range wantSyms {
		if _, ok := sink.symbols[qn]; !ok {
			t.Errorf("missing symbol %q (got %d: %v)", qn, len(sink.symbols), keys(sink.symbols))
		}
	}

	// Go files must be ignored by the tree-sitter backend.
	for qn := range sink.symbols {
		if qn == "skip/big.go:Ignored" {
			t.Errorf("TreeSitter should not index Go files, got %q", qn)
		}
	}

	// Heuristic same-file call resolution.
	wantEdges := [][2]string{
		{"py/app.py:handler", "py/app.py:helper"},
		{"web/app.ts:route", "web/app.ts:dispatch"},
	}
	for _, e := range wantEdges {
		if !sink.edges[e] {
			t.Errorf("missing CALLS edge %s -> %s (edges=%v)", e[0], e[1], sink.edges)
		}
	}

	for _, f := range sink.files {
		if f.Fidelity != Heuristic {
			t.Errorf("file %s fidelity = %q, want heuristic", f.Path, f.Fidelity)
		}
	}
}

func keys(m map[string]Symbol) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
