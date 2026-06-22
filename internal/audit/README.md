# Audit Trail

Records a structured, queryable trail of every mutating action an AI agent
takes against issue trackers through the daemon, using a [CloudEvents
1.0](https://cloudevents.io) envelope. The trail answers *what* an agent did
and *why* — durably, in a form tools and humans can query.

## Capabilities

- **Rationale capture** — every recorded action carries the agent's reasoning,
  not just the mechanical change.
- **CloudEvents envelope** — each action is a standard event (`id`, `source`,
  `type`, `time`, `specversion`, `subject`, `data`), so the trail is
  machine-queryable rather than ad-hoc free text.
- **Actor / resource / operation / outcome** — each event decomposes into which
  configured tracker credential acted, which ticket/project it targeted, which
  operation ran (`create`, `edit`, `delete`, `comment`, `status`, `start`), and
  the result (`success` / `failure` / `denied`).
- **At-decision-time capture** — the model id/version, the inputs, and the
  reasoning are recorded as they existed at emission. Agent decisions are
  non-deterministic, so a past decision cannot be reconstructed by replaying the
  prompt later; it must be captured when it happens.
- **SQLite storage** — the full envelope is stored as JSON alongside decomposed,
  indexed columns. Events are retained for 90 days (accountability records
  outlive the shorter stats trend window) and pruned automatically.
- **`human audit list` / `human audit show`** — read the trail through the
  daemon, as a fixed-width table, raw CloudEvents JSON (`--json`), or a single
  full envelope by event id.

## Decision context (`HUMAN_AUDIT_*`)

An agent harness forwards the at-decision-time context via environment
variables, set once per session. The client forwards them to the daemon, which
records them with each mutating action:

- `HUMAN_AUDIT_MODEL_ID` — the exact model id (e.g. `claude-opus-4`).
- `HUMAN_AUDIT_MODEL_VERSION` — the model version/date.
- `HUMAN_AUDIT_INPUTS` — the inputs that drove the decision.
- `HUMAN_AUDIT_RATIONALE` — the *why* behind the action.

All are optional: a missing rationale is allowed and recorded empty; the event
still captures actor, resource, operation, outcome, and model context.

## Known boundary

Only commands that flow **through the daemon** are audited — that is the single
choke point that knows the authenticated actor identity and the outcome exit
code. Direct CLI invocations with no daemon running are not recorded by this
path. This matches the trust model: autonomous agents run against the daemon.
Secrets passed via `--*-token` / `--*-key` flags are stripped before storage, so
the trail is safe to review and share.
