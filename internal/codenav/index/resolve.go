package index

// resolve.go holds the heuristic name/scope resolution used by the TreeSitter
// backend to turn call sites into edges. This mirrors how codebase-memory-mcp
// and codegraph build their graphs (tree-sitter has no semantic resolution of
// its own). Confidence is recorded so heuristic edges are distinguishable from
// GoNative's precise (confidence 1.0) edges.

// enclosingDef returns the qname of the innermost definition whose byte span
// contains [start,end] — i.e. the function/method that contains a call site.
func enclosingDef(defs []tsDef, start, end uint32) string {
	best := ""
	var bestStart uint32
	for _, d := range defs {
		if d.startByte <= start && d.endByte >= end {
			if best == "" || d.startByte >= bestStart {
				best, bestStart = d.qname, d.startByte
			}
		}
	}
	return best
}

// resolveCallee maps a referenced name to a definition qname via a confidence
// cascade: same-file exact match wins; otherwise a project-wide unique name.
// Ambiguous project-wide names are left unresolved rather than guessed.
func resolveCallee(name string, f tsFile, nameToQNames map[string][]string) (string, float64) {
	// 1. Same-file definition (high confidence — local scope).
	for _, d := range f.defs {
		if d.name == name {
			return d.qname, 0.85
		}
	}
	// 2. Project-wide unique definition (medium confidence).
	if cands := nameToQNames[name]; len(cands) == 1 {
		return cands[0], 0.6
	}
	// 3. Ambiguous or unknown — do not guess.
	return "", 0
}
