---
name: human-pickup-review
description: Pick up a PM ticket flagged with [human:ready-for-review] and run a peer review on each linked engineering ticket
argument-hint: <pm-ticket-key>
---

You are picking up a code review handoff. The engineer (human or AI) who finished the work posted a structured comment on a PM ticket with the format:

```
[human:ready-for-review]
engineering: HUM-89, HUM-90
branch: main
commits: 2037e40, 64bb370
```

Your job: parse that comment, run the `human-reviewer` agent against each engineering key, then post a follow-up comment on the PM ticket summarising the outcome.

## Steps

1. **Resolve the PM ticket.** `$ARGUMENTS` is the PM ticket key (e.g. `SC-79`). Run `human tracker list` to find its tracker. Use the tracker marked with `role: pm` — if roles are not set, pick the tracker whose kind matches the key prefix (`SC-…` → Shortcut, `KAN-…`/issue-in-project → Jira, etc.).

2. **Read the latest handoff comment.** Run `human <pm-tracker> issue comment list <PM_KEY>`. Scan comments newest-first for a body starting with `[human:ready-for-review]`. If none is found, stop and report: `No ready-for-review handoff on <PM_KEY>`. Do not guess.

3. **Parse the block.** Extract:
   - `engineering:` — comma-separated engineering ticket keys.
   - `branch:` — the branch the reviewer should be on.
   - `commits:` — short SHAs, for cross-checking.

   If the current branch does not match `branch:`, warn the user but proceed (the reviewer agent operates on the current branch; the user may have chosen to review from a different branch deliberately).

4. **Run the reviewer per engineering key.** For each key in `engineering:`, invoke the existing reviewer agent via the Task tool:
   ```
   Task(subagent_type="human-reviewer", prompt="Review changes for ticket <ENG_KEY>")
   ```
   Each run writes `.human/reviews/<eng_key_lowercased>.md`.

5. **Collect verdicts.** Open each `.human/reviews/<key>.md` the reviewer produced. The first line under `## Summary` is the verdict (`pass`, `pass with notes`, or `fail`). Roll them up into an overall verdict:
   - all pass → `pass`
   - any pass-with-notes, no fails → `pass with notes`
   - any fail → `fail`

6. **Post the follow-up comment on the PM ticket.** Use this format:
   ```
   [human:review-complete]
   verdict: <overall-verdict>
   reviews:
     <ENG_KEY_1>: <verdict> — .human/reviews/<eng_key_1>.md
     <ENG_KEY_2>: <verdict> — .human/reviews/<eng_key_2>.md
   ```
   Post it with `human <pm-tracker> issue comment add <PM_KEY> "<body>"`. The `[human:review-complete]` header mirrors the handoff header so future tooling can close the loop (e.g. auto-transition the engineering tickets to Done when all reviews pass).

7. **Report to the user.** Summarise in plain text: which engineering tickets were reviewed, the overall verdict, and where each review lives.

## Principles

- **Do not invoke `human-reviewer` on keys outside the handoff block.** Only review what the handoff claims. Scope creep in the review defeats the purpose of the convention.
- **Do not transition ticket status.** Closing/rejecting is the user's decision after reading the review. This skill reports, it does not decide.
- **Do NOT use `AskUserQuestion`.** Execute autonomously. If the handoff is missing or malformed, stop and report.
