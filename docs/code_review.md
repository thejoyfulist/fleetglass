# Review checklist

Used by the reviewer (Codex) on every branch, and by the Claude worker before it reports a task done. Each line comes from defects found in past reviews of this repo; the list grows at the monthly retrospective. A line that does not apply to the diff is skipped; a line that applies and fails is a finding.

No review findings recorded yet for this repo: defect patterns are added at the monthly retrospective.

## Spec

- Every numbered requirement of the task spec is met, and a test proves it. A requirement with no test that would fail without it counts as not met.

## Documentation

- A user-visible change adds an entry under `## [Unreleased]` in `docs/CHANGELOG.md` and updates the README or the matching page in `docs/` in the same branch; new config keys go in `docs/CONFIGURATION.md`.
