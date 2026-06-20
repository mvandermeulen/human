package graph

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/gethuman-sh/human/internal/codenav/index"
	"github.com/gethuman-sh/human/internal/codenav/store"
)

// indexFixture writes a small A → B → C Go call chain, indexes it, and returns
// the live DB so the traversal functions have a real call graph to walk.
func indexFixture(t *testing.T) *sql.DB {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod": "module example.com/fix\n\ngo 1.21\n",
		"main.go": `package main

func A() { B() }
func B() { C() }
func C() {}
func main() { A() }
`,
	}
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "codenav.db"))
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })

	scan := index.RepoScan{Project: "fix", Root: root}
	w, err := st.NewWriter(scan.Project, scan.Root)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}
	for _, ix := range index.PickFor(scan) {
		if err := ix.Index(context.Background(), scan, w); err != nil {
			t.Fatalf("index with %s: %v", ix.Name(), err)
		}
	}
	if err := w.Commit(""); err != nil {
		t.Fatalf("commit: %v", err)
	}
	return st.DB()
}

func symID(t *testing.T, db *sql.DB, qname string) int64 {
	t.Helper()
	var id int64
	if err := db.QueryRow(`SELECT id FROM symbol WHERE qname = ?`, qname).Scan(&id); err != nil {
		t.Fatalf("lookup %s: %v", qname, err)
	}
	return id
}

func TestTraverse(t *testing.T) {
	db := indexFixture(t)
	a := symID(t, db, "example.com/fix.A")
	c := symID(t, db, "example.com/fix.C")

	callers, err := Callers(db, c, 5)
	if err != nil {
		t.Fatalf("Callers: %v", err)
	}
	if len(callers) == 0 {
		t.Error("Callers(C) returned nothing (A and B call into C)")
	}

	callees, err := Callees(db, a, 5)
	if err != nil {
		t.Fatalf("Callees: %v", err)
	}
	if len(callees) == 0 {
		t.Error("Callees(A) returned nothing (A calls B → C)")
	}

	paths, err := CallPaths(db, a, c, 12, 8)
	if err != nil {
		t.Fatalf("CallPaths: %v", err)
	}
	if len(paths) == 0 {
		t.Error("CallPaths(A → C) found no path")
	}
}
