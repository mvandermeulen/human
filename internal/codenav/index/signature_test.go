package index

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSourceSignature(t *testing.T) {
	dir := writePolyglot(t)
	scan := RepoScan{Root: dir}

	s1 := SourceSignature(scan)
	if s1 == "" {
		t.Fatal("empty signature")
	}
	if s2 := SourceSignature(scan); s1 != s2 {
		t.Fatalf("signature not stable: %q != %q", s1, s2)
	}

	// A content change must change the signature.
	if err := os.WriteFile(filepath.Join(dir, "py/app.py"), []byte("def x():\n    pass\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if s3 := SourceSignature(scan); s3 == s1 {
		t.Fatal("signature unchanged after editing a file")
	}
}
