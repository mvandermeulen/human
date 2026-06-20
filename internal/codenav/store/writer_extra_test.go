package store_test

import (
	"path/filepath"
	"testing"

	"github.com/gethuman-sh/human/internal/codenav/index"
	"github.com/gethuman-sh/human/internal/codenav/store"
)

// TestFileRouteAndSig covers the writer paths the traversal test skips — File
// records, Route records, and the project source-signature round-trip — plus
// project removal.
func TestFileRouteAndSig(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	w, err := st.NewWriter("p", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.File(index.FileRec{Path: "h.go", Lang: "go", ContentHash: "abc123", Fidelity: index.Precise}); err != nil {
		t.Fatalf("File: %v", err)
	}
	if err := w.Symbol(index.Symbol{QName: "pkg.Handler", Name: "Handler", Kind: "func", File: "h.go", StartLine: 1, EndLine: 3}); err != nil {
		t.Fatalf("Symbol: %v", err)
	}
	if err := w.Route(index.Route{Method: "GET", Pattern: "/health", HandlerQName: "pkg.Handler", Framework: "net/http"}); err != nil {
		t.Fatalf("Route: %v", err)
	}
	if err := w.Commit("rev1"); err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if err := st.SetProjectSig("p", "sig-xyz"); err != nil {
		t.Fatalf("SetProjectSig: %v", err)
	}
	got, err := st.ProjectSig("p")
	if err != nil {
		t.Fatalf("ProjectSig: %v", err)
	}
	if got != "sig-xyz" {
		t.Errorf("ProjectSig = %q, want sig-xyz", got)
	}

	if err := st.DeleteProject("p"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	projs, err := st.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for _, p := range projs {
		if p.Name == "p" {
			t.Error("project still present after DeleteProject")
		}
	}
}

// TestRollback verifies an aborted writer leaves no project behind.
func TestRollback(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()

	w, err := st.NewWriter("doomed", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Symbol(index.Symbol{QName: "pkg.X", Name: "X", Kind: "func", File: "x.go", StartLine: 1, EndLine: 1}); err != nil {
		t.Fatal(err)
	}
	if err := w.Rollback(); err != nil {
		t.Fatalf("Rollback: %v", err)
	}

	projs, err := st.ListProjects()
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	for _, p := range projs {
		if p.Name == "doomed" {
			t.Error("rolled-back project was persisted")
		}
	}
}
