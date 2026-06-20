package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gethuman-sh/human/internal/codenav/index"
)

// Writer persists one indexing run for a single project inside a transaction.
// References, edges and routes are buffered and resolved at Commit, once every
// symbol's id is known (definitions may appear after their use sites).
//
// Writer implements index.Sink.
type Writer struct {
	s         *Store
	tx        *sql.Tx
	project   string
	root      string
	projectID int64

	fileIDs map[string]int64 // repo-relative path -> file.id
	symIDs  map[string]int64 // qname -> symbol.id

	refs   []index.Reference
	edges  []index.Edge
	routes []index.Route
}

func scanID(tx *sql.Tx, query string, args ...any) (int64, error) {
	var id int64
	err := tx.QueryRow(query, args...).Scan(&id)
	return id, err
}

// NewWriter starts a fresh index of project at root. Any prior data for the
// project is removed first (M1 does full re-index).
func (s *Store) NewWriter(project, root string) (*Writer, error) {
	if err := s.DeleteProject(project); err != nil {
		return nil, fmt.Errorf("clear project: %w", err)
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	w := &Writer{
		s:       s,
		tx:      tx,
		project: project,
		root:    root,
		fileIDs: map[string]int64{},
		symIDs:  map[string]int64{},
	}
	id, err := scanID(tx, `INSERT INTO project(name, root_path) VALUES(?, ?) RETURNING id`, project, root)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("insert project: %w", err)
	}
	w.projectID = id
	return w, nil
}

// ensureFile returns the file id for a repo-relative path, inserting a stub row
// if the path has not been seen yet.
func (w *Writer) ensureFile(path string) (int64, error) {
	if id, ok := w.fileIDs[path]; ok {
		return id, nil
	}
	id, err := scanID(w.tx,
		`INSERT INTO file(project_id, path, content_hash) VALUES(?, ?, '')
		 ON CONFLICT(project_id, path) DO UPDATE SET path=excluded.path
		 RETURNING id`, w.projectID, path)
	if err != nil {
		return 0, err
	}
	w.fileIDs[path] = id
	return id, nil
}

// File records file metadata and indexes the file body for code search.
func (w *Writer) File(f index.FileRec) error {
	id, err := w.ensureFile(f.Path)
	if err != nil {
		return err
	}
	if _, err := w.tx.Exec(
		`UPDATE file SET lang=?, content_hash=?, fidelity=? WHERE id=?`,
		f.Lang, f.ContentHash, string(f.Fidelity), id); err != nil {
		return err
	}
	if body, err := os.ReadFile(filepath.Join(w.root, f.Path)); err == nil {
		if _, err := w.tx.Exec(
			`INSERT INTO fts_code(body, path, project) VALUES(?, ?, ?)`,
			string(body), f.Path, w.project); err != nil {
			return err
		}
	}
	return nil
}

// Symbol inserts a definition and makes it searchable.
func (w *Writer) Symbol(sym index.Symbol) error {
	fileID, err := w.ensureFile(sym.File)
	if err != nil {
		return err
	}
	id, err := scanID(w.tx,
		`INSERT INTO symbol(project_id, file_id, qname, name, kind, signature, doc,
		                    start_line, start_col, end_line, end_col)
		 VALUES(?,?,?,?,?,?,?,?,?,?,?) RETURNING id`,
		w.projectID, fileID, sym.QName, sym.Name, sym.Kind, sym.Signature, sym.Doc,
		sym.StartLine, sym.StartCol, sym.EndLine, sym.EndCol)
	if err != nil {
		return err
	}
	w.symIDs[sym.QName] = id
	_, err = w.tx.Exec(
		`INSERT INTO fts_symbol(name, qname, kind, symbol_id, project) VALUES(?,?,?,?,?)`,
		sym.Name, sym.QName, sym.Kind, id, w.project)
	return err
}

// Reference buffers a use-site for commit-time resolution.
func (w *Writer) Reference(r index.Reference) error { w.refs = append(w.refs, r); return nil }

// Edge buffers a relationship for commit-time resolution.
func (w *Writer) Edge(e index.Edge) error { w.edges = append(w.edges, e); return nil }

// Route buffers a route for commit-time resolution.
func (w *Writer) Route(r index.Route) error { w.routes = append(w.routes, r); return nil }

// Commit resolves buffered references/edges/routes against the symbol table and
// finalizes the transaction.
func (w *Writer) Commit(vcsRev string) error {
	if err := w.commitRefs(); err != nil {
		return w.fail(err)
	}
	if err := w.commitEdges(); err != nil {
		return w.fail(err)
	}
	if err := w.commitRoutes(); err != nil {
		return w.fail(err)
	}
	if _, err := w.tx.Exec(`UPDATE project SET indexed_at=?, vcs_rev=? WHERE id=?`,
		time.Now().Unix(), vcsRev, w.projectID); err != nil {
		return w.fail(err)
	}
	return w.tx.Commit()
}

// commitRefs resolves buffered references to symbol ids (where known) and
// persists them.
func (w *Writer) commitRefs() error {
	for _, r := range w.refs {
		fileID, err := w.ensureFile(r.File)
		if err != nil {
			return err
		}
		var symID any
		if id, ok := w.symIDs[r.ToQName]; ok {
			symID = id
		}
		if _, err := w.tx.Exec(
			`INSERT INTO reference(project_id, symbol_id, qname, file_id, line, col, role)
			 VALUES(?,?,?,?,?,?,?)`,
			w.projectID, symID, r.ToQName, fileID, r.Line, r.Col, r.Role); err != nil {
			return err
		}
	}
	return nil
}

// commitEdges persists CALLS edges, resolving each target to an intra-repo
// symbol or, failing that, a symbol in another indexed project (CROSS_CALLS).
func (w *Writer) commitEdges() error {
	for _, e := range w.edges {
		from, ok := w.symIDs[e.FromQName]
		if !ok {
			continue // caller not defined in this repo
		}
		// Intra-repo: both ends in this project.
		if to, ok := w.symIDs[e.ToQName]; ok {
			if _, err := w.tx.Exec(
				`INSERT OR IGNORE INTO edge(src_id, dst_id, kind, confidence) VALUES(?,?,?,?)`,
				from, to, e.Kind, e.Confidence); err != nil {
				return err
			}
			continue
		}
		// Cross-repo: resolve the target qname against another indexed project.
		var dst int64
		err := w.tx.QueryRow(
			`SELECT id FROM symbol WHERE qname=? AND project_id<>? LIMIT 1`,
			e.ToQName, w.projectID).Scan(&dst)
		if err == sql.ErrNoRows {
			continue // stdlib or unindexed dependency — drop
		}
		if err != nil {
			return err
		}
		if _, err := w.tx.Exec(
			`INSERT OR IGNORE INTO edge(src_id, dst_id, kind, confidence) VALUES(?,?,?,?)`,
			from, dst, "CROSS_CALLS", e.Confidence); err != nil {
			return err
		}
	}
	return nil
}

// commitRoutes persists detected web routes, linking to the handler symbol id
// when it was defined in this repo.
func (w *Writer) commitRoutes() error {
	for _, rt := range w.routes {
		var handler any
		if id, ok := w.symIDs[rt.HandlerQName]; ok {
			handler = id
		}
		if _, err := w.tx.Exec(
			`INSERT INTO route(project_id, method, pattern, handler_id, framework) VALUES(?,?,?,?,?)`,
			w.projectID, rt.Method, rt.Pattern, handler, rt.Framework); err != nil {
			return err
		}
	}
	return nil
}

func (w *Writer) fail(err error) error {
	_ = w.tx.Rollback()
	return err
}

// Rollback aborts the indexing transaction.
func (w *Writer) Rollback() error { return w.tx.Rollback() }
