package daemon

import (
	"context"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/gethuman-sh/human/internal/audit"
)

// newAuditServer returns a Server with a live audit writer+store backed by an
// in-memory database, plus the store for assertions. The writer must be Closed
// (drained) before querying.
func newAuditServer(t *testing.T) (*Server, *audit.Store, *audit.Writer) {
	t.Helper()
	store, err := audit.NewStore(":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	w := audit.NewWriter(context.Background(), store, zerolog.Nop())
	srv := &Server{Logger: zerolog.Nop(), AuditSink: w, AuditStore: store}
	return srv, store, w
}

func queryAll(t *testing.T, store *audit.Store) []audit.Event {
	t.Helper()
	now := time.Now().UTC()
	events, err := store.Query(context.Background(), audit.Filter{
		Since: now.Add(-time.Hour), Until: now.Add(time.Hour), Limit: 100,
	})
	require.NoError(t, err)
	return events
}

func TestEmitAudit_successOnExit0(t *testing.T) {
	srv, store, w := newAuditServer(t)
	srv.emitAudit([]string{"jira", "issue", "create", "--project=KAN", "Title"},
		audit.OutcomeSuccess, func(string) string { return "" })
	w.Close()

	events := queryAll(t, store)
	require.Len(t, events, 1)
	assert.Equal(t, audit.OutcomeSuccess, events[0].Data.Outcome)
	assert.Equal(t, "create", events[0].Data.Operation)
}

func TestEmitAudit_failureOnExit1(t *testing.T) {
	srv, store, w := newAuditServer(t)
	srv.emitAudit([]string{"jira", "issue", "delete", "KAN-1"},
		audit.OutcomeFailure, func(string) string { return "" })
	w.Close()

	events := queryAll(t, store)
	require.Len(t, events, 1)
	assert.Equal(t, audit.OutcomeFailure, events[0].Data.Outcome)
}

func TestEmitAudit_deniedOnAbort(t *testing.T) {
	srv, store, w := newAuditServer(t)
	srv.emitAudit([]string{"jira", "issue", "delete", "KAN-1"},
		audit.OutcomeDenied, func(string) string { return "" })
	w.Close()

	events := queryAll(t, store)
	require.Len(t, events, 1)
	assert.Equal(t, audit.OutcomeDenied, events[0].Data.Outcome)
	assert.Equal(t, "delete", events[0].Data.Operation)
}

func TestEmitAudit_decisionFromEnv(t *testing.T) {
	srv, store, w := newAuditServer(t)
	lookup := func(k string) string {
		if k == "HUMAN_AUDIT_RATIONALE" {
			return "cleaning up stale ticket"
		}
		if k == "HUMAN_AUDIT_MODEL_ID" {
			return "claude-opus-4"
		}
		return ""
	}
	srv.emitAudit([]string{"jira", "issue", "delete", "KAN-1"}, audit.OutcomeSuccess, lookup)
	w.Close()

	events := queryAll(t, store)
	require.Len(t, events, 1)
	assert.Equal(t, "cleaning up stale ticket", events[0].Data.Decision.Rationale)
	assert.Equal(t, "claude-opus-4", events[0].Data.Decision.ModelID)
}

func TestEmitAudit_noSink(t *testing.T) {
	srv := &Server{Logger: zerolog.Nop()} // AuditSink nil
	// Must not panic and must be a no-op.
	srv.emitAudit([]string{"jira", "issue", "delete", "KAN-1"},
		audit.OutcomeSuccess, func(string) string { return "" })
}

func TestEmitAudit_readOnlyNotRecorded(t *testing.T) {
	srv, store, w := newAuditServer(t)
	srv.emitAudit([]string{"jira", "issue", "get", "KAN-1"},
		audit.OutcomeSuccess, func(string) string { return "" })
	w.Close()

	assert.Empty(t, queryAll(t, store))
}

func TestHandleAuditQuery_emptyStore(t *testing.T) {
	srv := &Server{Logger: zerolog.Nop()} // AuditStore nil
	resp := captureHandlerResponse(t, func(c net.Conn) { srv.handleAuditQuery(c, nil) })
	assert.Equal(t, "[]\n", resp.Stdout)
}

func TestParseAuditFilter(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		f := parseAuditFilter(nil)
		assert.Empty(t, f.Subject)
		assert.Empty(t, f.TrackerKind)
		assert.Equal(t, 0, f.Limit)
		assert.WithinDuration(t, time.Now().UTC(), f.Until, 2*time.Second)
		// Default window spans 7 days.
		assert.WithinDuration(t, time.Now().UTC().Add(-7*24*time.Hour), f.Since, 2*time.Second)
	})

	t.Run("equalsForms", func(t *testing.T) {
		f := parseAuditFilter([]string{"--subject=KAN-1", "--tracker=jira", "--limit=5"})
		assert.Equal(t, "KAN-1", f.Subject)
		assert.Equal(t, "jira", f.TrackerKind)
		assert.Equal(t, 5, f.Limit)
	})

	t.Run("spaceForms", func(t *testing.T) {
		f := parseAuditFilter([]string{"--subject", "KAN-2", "--tracker", "linear", "--limit", "9"})
		assert.Equal(t, "KAN-2", f.Subject)
		assert.Equal(t, "linear", f.TrackerKind)
		assert.Equal(t, 9, f.Limit)
	})

	t.Run("rfc3339Times", func(t *testing.T) {
		f := parseAuditFilter([]string{"--since", "2026-01-01T00:00:00Z", "--until=2026-02-01T00:00:00Z"})
		assert.Equal(t, 2026, f.Since.Year())
		assert.Equal(t, time.February, f.Until.Month())
	})
}

func TestHandleAuditQuery_returnsEvents(t *testing.T) {
	srv, _, w := newAuditServer(t)
	srv.emitAudit([]string{"jira", "issue", "create", "--project=KAN", "Title"},
		audit.OutcomeSuccess, func(string) string { return "" })
	w.Close()

	resp := captureHandlerResponse(t, func(c net.Conn) { srv.handleAuditQuery(c, nil) })

	var events []audit.Event
	require.NoError(t, json.Unmarshal([]byte(resp.Stdout), &events))
	require.Len(t, events, 1)
	assert.Equal(t, "create", events[0].Data.Operation)
}
