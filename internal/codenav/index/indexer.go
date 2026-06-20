// Package index defines the indexing contract and the data types that flow
// from a language backend into the store. Backends parse source on disk and
// push symbols, references, edges and routes into a Sink.
package index

import "context"

// Fidelity describes how trustworthy a backend's output is.
type Fidelity string

const (
	// Precise output comes from a type-resolved analysis (Go via x/tools).
	Precise Fidelity = "precise"
	// Heuristic output comes from syntactic parsing plus name/import
	// resolution (tree-sitter backends).
	Heuristic Fidelity = "heuristic"
	// FTSOnly means a file is only full-text searchable, no symbols.
	FTSOnly Fidelity = "fts"
)

// Symbol is a definition (function, method, type, var, const, ...).
type Symbol struct {
	QName               string // <project>.<modpath>.<name> — stable, cross-repo for Go
	Name                string // bare name, for search and name-based resolution
	Kind                string // func|method|type|var|const|iface|field|...
	File                string // repo-relative path
	Signature           string
	Doc                 string
	StartLine, StartCol int
	EndLine, EndCol     int
}

// Reference is a use-site of a symbol (call, read, import, ...).
type Reference struct {
	ToQName string // qname of the referenced symbol (may resolve later)
	File    string // repo-relative path of the use site
	Line    int
	Col     int
	Role    string // call|read|write|import|impl
}

// Edge is a resolved relationship between two symbols, keyed by qname so the
// store can map both ends to symbol ids after all symbols are inserted.
type Edge struct {
	FromQName  string
	ToQName    string
	Kind       string  // CALLS|IMPORTS|IMPLEMENTS|CROSS_CALLS
	Confidence float64 // 1.0 for precise, <1.0 for heuristic
}

// Route is a web framework endpoint bound to a handler symbol.
type Route struct {
	Method       string
	Pattern      string
	HandlerQName string
	Framework    string
}

// FileRec records a scanned file so re-indexing can skip unchanged files.
type FileRec struct {
	Path        string
	Lang        string
	ContentHash string
	Fidelity    Fidelity
}

// Sink receives everything a backend extracts. Implementations batch and
// persist; calls are not safe for concurrent use unless documented otherwise.
type Sink interface {
	File(FileRec) error
	Symbol(Symbol) error
	Reference(Reference) error
	Edge(Edge) error
	Route(Route) error
}

// RepoScan is the input handed to a backend: the project name and absolute root.
type RepoScan struct {
	Project string
	Root    string // absolute path
}

// Indexer is a language backend.
type Indexer interface {
	Name() string
	Fidelity() Fidelity
	// CanHandle reports whether this backend should run for the repo
	// (e.g. GoNative when a go.mod is present).
	CanHandle(RepoScan) bool
	Index(ctx context.Context, scan RepoScan, sink Sink) error
}
