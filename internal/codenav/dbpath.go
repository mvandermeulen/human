// Package codenav holds shared helpers for the code-navigation feature, whose
// engine lives in the store, query, graph, and index subpackages.
package codenav

import (
	"os"
	"path/filepath"
)

// DefaultDBPath returns the path to the code-navigation index database under
// ~/.human/, creating the directory if needed and falling back to ./.human/
// when the home directory is unknown. Mirrors the convention used by
// internal/index and internal/stats.
func DefaultDBPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".human", "codenav.db")
	}
	dir := filepath.Join(home, ".human")
	_ = os.MkdirAll(dir, 0o750)
	return filepath.Join(dir, "codenav.db")
}
