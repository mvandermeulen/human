// Package graph answers reachability questions over the CALLS edge table using
// depth-capped, cycle-guarded recursive CTEs.
package graph

import "database/sql"

// Node is a symbol reached during traversal.
type Node struct {
	ID    int64
	QName string
	Name  string
	Kind  string
	File  string
	Line  int
	Depth int
}

// Callees returns symbols transitively called by start, up to maxDepth.
func Callees(db *sql.DB, start int64, maxDepth int) ([]Node, error) {
	return walk(db, start, maxDepth, true)
}

// Callers returns symbols that transitively call start, up to maxDepth.
func Callers(db *sql.DB, start int64, maxDepth int) ([]Node, error) {
	return walk(db, start, maxDepth, false)
}

// walk runs the directional reachability CTE. outbound=true follows callees
// (src->dst); outbound=false follows callers (dst->src).
func walk(db *sql.DB, start int64, maxDepth int, outbound bool) ([]Node, error) {
	join := "e.src_id = w.id" // step to dst (callees)
	next := "e.dst_id"
	if !outbound {
		join = "e.dst_id = w.id" // step to src (callers)
		next = "e.src_id"
	}
	q := `
WITH RECURSIVE w(id, depth) AS (
  SELECT ?, 0
  UNION
  SELECT ` + next + `, w.depth + 1
  FROM edge e JOIN w ON ` + join + `
  WHERE e.kind IN ('CALLS','CROSS_CALLS') AND w.depth < ?
)
SELECT s.id, s.qname, s.name, s.kind, f.path, s.start_line, MIN(w.depth) AS d
FROM w
JOIN symbol s ON s.id = w.id
JOIN file f   ON f.id = s.file_id
WHERE w.id <> ?
GROUP BY s.id
ORDER BY d, s.qname`
	rows, err := db.Query(q, start, maxDepth, start)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	return scanNodes(rows)
}

// CallPaths returns up to limit distinct call paths from -> to (each a sequence
// of nodes), shortest first.
func CallPaths(db *sql.DB, from, to int64, maxDepth, limit int) ([][]Node, error) {
	q := `
WITH RECURSIVE cp(id, depth, path) AS (
  SELECT ?, 0, ',' || ? || ','
  UNION ALL
  SELECT e.dst_id, cp.depth + 1, cp.path || e.dst_id || ','
  FROM edge e JOIN cp ON e.src_id = cp.id
  WHERE e.kind IN ('CALLS','CROSS_CALLS')
    AND cp.depth < ?
    AND instr(cp.path, ',' || e.dst_id || ',') = 0
)
SELECT path FROM cp WHERE id = ? ORDER BY depth LIMIT ?`
	rows, err := db.Query(q, from, from, maxDepth, to, limit)
	if err != nil {
		return nil, err
	}
	var idPaths [][]int64
	for rows.Next() {
		var path string
		if err := rows.Scan(&path); err != nil {
			_ = rows.Close()
			return nil, err
		}
		idPaths = append(idPaths, parseIDPath(path))
	}
	err = rows.Err()
	_ = rows.Close() // release the connection before per-node lookups (pool serializes)
	if err != nil {
		return nil, err
	}

	var out [][]Node
	for _, ids := range idPaths {
		nodes, err := nodesByID(db, ids)
		if err != nil {
			return nil, err
		}
		out = append(out, nodes)
	}
	return out, nil
}

func scanNodes(rows *sql.Rows) ([]Node, error) {
	var out []Node
	for rows.Next() {
		var n Node
		if err := rows.Scan(&n.ID, &n.QName, &n.Name, &n.Kind, &n.File, &n.Line, &n.Depth); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// nodesByID loads nodes for an ordered list of ids, preserving order.
func nodesByID(db *sql.DB, ids []int64) ([]Node, error) {
	byID := map[int64]Node{}
	for _, id := range ids {
		var n Node
		err := db.QueryRow(`
			SELECT s.id, s.qname, s.name, s.kind, f.path, s.start_line
			FROM symbol s JOIN file f ON f.id = s.file_id WHERE s.id = ?`, id).
			Scan(&n.ID, &n.QName, &n.Name, &n.Kind, &n.File, &n.Line)
		if err == sql.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}
		byID[id] = n
	}
	out := make([]Node, 0, len(ids))
	for i, id := range ids {
		if n, ok := byID[id]; ok {
			n.Depth = i
			out = append(out, n)
		}
	}
	return out, nil
}

// parseIDPath turns ",1,2,3," into []int64{1,2,3}.
func parseIDPath(s string) []int64 {
	var ids []int64
	var cur int64
	inNum := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= '0' && c <= '9' {
			cur = cur*10 + int64(c-'0')
			inNum = true
		} else if inNum {
			ids = append(ids, cur)
			cur, inNum = 0, false
		}
	}
	if inNum {
		ids = append(ids, cur)
	}
	return ids
}
