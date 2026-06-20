package store_test

import (
	"path/filepath"
	"testing"

	"github.com/gethuman-sh/human/internal/codenav/index"
	"github.com/gethuman-sh/human/internal/codenav/query"
	"github.com/gethuman-sh/human/internal/codenav/store"
)

// openABC opens a store and writes a synthetic A -> B -> C call chain (no Go
// toolchain needed), with a reference to A at line 7. Used by the traversal
// tests below.
func openABC(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })

	w, err := st.NewWriter("p", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range [][2]string{{"pkg.A", "A"}, {"pkg.B", "B"}, {"pkg.C", "C"}} {
		if err := w.Symbol(index.Symbol{QName: s[0], Name: s[1], Kind: "func", File: "a.go", StartLine: 1, EndLine: 1}); err != nil {
			t.Fatal(err)
		}
	}
	for _, e := range [][2]string{{"pkg.A", "pkg.B"}, {"pkg.B", "pkg.C"}} {
		if err := w.Edge(index.Edge{FromQName: e[0], ToQName: e[1], Kind: "CALLS", Confidence: 1}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Reference(index.Reference{ToQName: "pkg.A", File: "a.go", Line: 7, Role: "ref"}); err != nil {
		t.Fatal(err)
	}
	if err := w.Commit("rev1"); err != nil {
		t.Fatal(err)
	}
	return st
}

func TestWriterProjectCounts(t *testing.T) {
	st := openABC(t)
	projs, err := st.ListProjects()
	if err != nil {
		t.Fatal(err)
	}
	if len(projs) != 1 || projs[0].Symbols != 3 || projs[0].Edges != 2 {
		t.Fatalf("projects = %+v, want 1 project / 3 symbols / 2 edges", projs)
	}
}

func TestTraversalCallees(t *testing.T) {
	st := openABC(t)
	callees, err := query.Callees(st.DB(), "pkg.A", 5)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, n := range callees {
		got[n.Name] = true
	}
	if !got["B"] || !got["C"] {
		t.Fatalf("callees(A) = %v, want B and C", got)
	}
}

func TestTraversalCallPath(t *testing.T) {
	st := openABC(t)
	paths, err := query.CallPath(st.DB(), "pkg.A", "pkg.C", 10, 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) == 0 || len(paths[0]) != 3 {
		t.Fatalf("callpath A->C = %v, want one 3-node path", paths)
	}
}

func TestReferenceRecorded(t *testing.T) {
	st := openABC(t)
	refs, err := query.Refs(st.DB(), "pkg.A")
	if err != nil {
		t.Fatal(err)
	}
	if len(refs) != 1 || refs[0].Line != 7 {
		t.Fatalf("refs(A) = %v, want one at line 7", refs)
	}
}

func TestReindexReplaces(t *testing.T) {
	st := openABC(t)
	// Re-indexing the same project replaces its rows rather than duplicating.
	w2, err := st.NewWriter("p", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := w2.Commit("rev2"); err != nil {
		t.Fatal(err)
	}
	projs, _ := st.ListProjects()
	if len(projs) != 1 || projs[0].Symbols != 0 {
		t.Fatalf("after reindex projects = %+v, want 1 project / 0 symbols", projs)
	}
}

// TestCrossRepoLinking verifies that a call whose target is defined in another
// indexed project becomes a CROSS_CALLS edge and is traversable.
func TestCrossRepoLinking(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	// Project B defines ext.Bar.
	wb, err := st.NewWriter("b", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := wb.Symbol(index.Symbol{QName: "ext.Bar", Name: "Bar", Kind: "func", File: "b.go", StartLine: 1, EndLine: 1}); err != nil {
		t.Fatal(err)
	}
	if err := wb.Commit(""); err != nil {
		t.Fatal(err)
	}

	// Project A defines a.Foo which calls ext.Bar (defined in B, not in A).
	wa, err := st.NewWriter("a", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := wa.Symbol(index.Symbol{QName: "a.Foo", Name: "Foo", Kind: "func", File: "a.go", StartLine: 1, EndLine: 1}); err != nil {
		t.Fatal(err)
	}
	if err := wa.Edge(index.Edge{FromQName: "a.Foo", ToQName: "ext.Bar", Kind: "CALLS", Confidence: 1}); err != nil {
		t.Fatal(err)
	}
	if err := wa.Commit(""); err != nil {
		t.Fatal(err)
	}

	// Cross-repo callers of ext.Bar must include a.Foo (via CROSS_CALLS).
	callers, err := query.Callers(st.DB(), "ext.Bar", 5)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, n := range callers {
		if n.QName == "a.Foo" {
			found = true
		}
	}
	if !found {
		t.Fatalf("cross-repo callers(ext.Bar) = %v, want a.Foo", callers)
	}
}
