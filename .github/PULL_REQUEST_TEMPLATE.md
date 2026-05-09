<!--
Thanks for the PR! Please fill in the sections below. PRs without a test plan
are unlikely to land. PRs against `main` will be redirected to `staging`.
-->

## Summary

<!-- One paragraph: what does this PR change and why. -->

## Linked issues

<!-- e.g. Closes #123, Fixes #456. If there is no issue, explain the motivation here. -->

## Type of change

<!-- Mark the relevant one with x. -->

- [ ] Bug fix (non-breaking change which fixes an issue)
- [ ] New feature (non-breaking change which adds functionality)
- [ ] Breaking change (fix or feature that would change existing behavior)
- [ ] Documentation update
- [ ] Performance improvement
- [ ] Refactor (no functional change)

## Test plan

<!--
Required. List the commands you ran and the result. Example:

- [x] `go build ./...` — clean
- [x] `go vet ./...` — clean
- [x] `go test ./...` — all packages green
- [x] New regression `TestFoo_BarBaz` — verified it FAILS on staging without the
      fix and PASSES with the fix.
-->

- [ ] `go build ./...`
- [ ] `go vet ./...`
- [ ] `go test ./...`
- [ ] New regression test added (or existing test extended)

## Checklist

- [ ] My code follows the project's [language conventions](../README.md#language-conventions) (English in code/logs/docs, French in learner-facing strings)
- [ ] I have read [CONTRIBUTING.md](../CONTRIBUTING.md)
- [ ] My commits are focused and follow the conventional-commit prefix (`fix(scope):`, `feat(scope):`, …)
- [ ] I have updated [CHANGELOG.md](../CHANGELOG.md) if this PR is user-visible
- [ ] I have updated documentation under `docs/` if I touched the regulation pipeline or MCP surface
- [ ] I targeted `staging` (not `main`)

## Notes for the reviewer

<!-- Optional: anything that's not obvious from the diff. Trade-offs you
considered, alternatives you rejected, follow-ups deferred to a later PR. -->
