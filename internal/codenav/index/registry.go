package index

// Registry holds the available backends in priority order (highest fidelity
// first). GoNative owns Go (precise); TreeSitter covers curated non-Go
// languages (heuristic) and skips Go.
var Registry = []Indexer{
	GoNative{},
	TreeSitter{},
}

// PickFor returns the backends that apply to a repo, highest fidelity first.
func PickFor(scan RepoScan) []Indexer {
	var out []Indexer
	for _, ix := range Registry {
		if ix.CanHandle(scan) {
			out = append(out, ix)
		}
	}
	return out
}
