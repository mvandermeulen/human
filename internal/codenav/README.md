# Code Navigation

`human codenav` indexes a repository into a local SQLite database and answers structural questions about it — go-to-definition, find-references, call graphs, blast radius, and full-text search — fast, offline, and token-frugal, so an AI agent can navigate code without dumping whole files into context. It runs locally where the code is and is never forwarded to the daemon.

- Indexes Go precisely and other languages structurally
- Go-to-definition and find-references with exact locations
- Walks call graphs: callers, callees, and call paths
- Reports blast radius of a symbol or a git diff
- Full-text search over code bodies and symbol names
- Lists detected web routes and their handlers
- Keeps one local index for many repositories
