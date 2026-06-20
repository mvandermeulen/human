package index

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/odvcencio/gotreesitter/grammars"
)

// SourceSignature computes a content hash over all source files codenav would index
// in a repo (Go files plus curated tree-sitter languages). It is used to skip
// re-indexing when nothing has changed. The signature is order-independent and
// sensitive to file content, additions and deletions.
func SourceSignature(scan RepoScan) string {
	var lines []string
	_ = filepath.WalkDir(scan.Root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		name := d.Name()
		include := strings.HasSuffix(name, ".go")
		if !include {
			if e := grammars.DetectLanguage(name); e != nil && tsLangs[e.Name] {
				include = true
			}
		}
		if !include {
			return nil
		}
		if rel, ok := relWithin(scan.Root, path); ok {
			lines = append(lines, rel+":"+hashFile(path))
		}
		return nil
	})
	sort.Strings(lines)
	sum := sha256.Sum256([]byte(strings.Join(lines, "\n")))
	return hex.EncodeToString(sum[:])
}
