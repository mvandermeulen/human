package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestPickFor(t *testing.T) {
	// A Go repo: GoNative applies and, being precise, must come first.
	goDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(goDir, "go.mod"), []byte("module x\n\ngo 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got := PickFor(RepoScan{Project: "x", Root: goDir})
	if len(got) == 0 {
		t.Fatal("PickFor returned no backends for a Go repo")
	}
	if got[0].Name() != "go" {
		t.Errorf("first backend = %q, want go (precise first)", got[0].Name())
	}

	// A non-Go repo: GoNative must not apply; TreeSitter still does.
	plainDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(plainDir, "app.py"), []byte("def f():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, ix := range PickFor(RepoScan{Project: "p", Root: plainDir}) {
		names[ix.Name()] = true
	}
	if names["go"] {
		t.Error("GoNative should not handle a non-Go repo")
	}
	if !names["treesitter"] {
		t.Error("TreeSitter should handle any repo")
	}
}

func TestBackendAccessors(t *testing.T) {
	if (GoNative{}).Name() != "go" || (GoNative{}).Fidelity() != Precise {
		t.Errorf("GoNative accessors: name=%q fidelity=%q", (GoNative{}).Name(), (GoNative{}).Fidelity())
	}
	if (TreeSitter{}).Name() != "treesitter" || (TreeSitter{}).Fidelity() != Heuristic {
		t.Errorf("TreeSitter accessors: name=%q fidelity=%q", (TreeSitter{}).Name(), (TreeSitter{}).Fidelity())
	}
	if !(TreeSitter{}).CanHandle(RepoScan{}) {
		t.Error("TreeSitter.CanHandle should always be true")
	}
}
