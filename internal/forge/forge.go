// Package forge abstracts code-hosting (forge) operations such as opening
// pull requests. It is deliberately separate from internal/tracker: a pull
// request is a code-repository concept, not an issue-tracker one. Some
// backends (GitHub, GitLab) are both a tracker and a forge and implement
// both interfaces; pure issue trackers (Jira, Linear, Shortcut, …) implement
// only tracker.Provider.
package forge

import "context"

// PullRequest carries both the request to open a pull request and the created
// result. Repo/Base/Head/Title/Body are inputs; Number/URL are populated on
// return (Title is echoed back from the forge).
type PullRequest struct {
	Repo  string // "owner/repo" (GitHub) or "group/project" (GitLab)
	Base  string // target branch the PR merges into (e.g. "main")
	Head  string // source branch holding the changes
	Title string
	Body  string

	Number int    // populated on return
	URL    string // populated on return
}

// Creator opens a pull request on a code-forge host.
type Creator interface {
	CreatePullRequest(ctx context.Context, pr *PullRequest) (*PullRequest, error)
}

// Forge aggregates code-forge operations. Today that is only pull-request
// creation; future operations (list, merge, status) extend this interface.
type Forge interface {
	Creator
}

// IsForgeKind reports whether a tracker kind also acts as a code forge that
// can open pull requests. It gates which `human <kind>` command trees expose
// the `pr` subcommand, so pure issue trackers don't advertise an operation
// they can't perform.
func IsForgeKind(kind string) bool {
	switch kind {
	case "github":
		return true
	default:
		return false
	}
}
