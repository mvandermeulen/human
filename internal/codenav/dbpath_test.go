package codenav

import (
	"path/filepath"
	"testing"
)

func TestDefaultDBPath(t *testing.T) {
	got := DefaultDBPath()
	if filepath.Base(got) != "codenav.db" {
		t.Errorf("DefaultDBPath() = %q, want it to end in codenav.db", got)
	}
	if dir := filepath.Base(filepath.Dir(got)); dir != ".human" {
		t.Errorf("DefaultDBPath() parent dir = %q, want .human", dir)
	}
}
