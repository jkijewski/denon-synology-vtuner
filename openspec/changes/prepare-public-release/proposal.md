# Proposal

## Why

The project is about to be published for other people to use, so the repository needs what GitHub and its community expect, and the remaining cleanups from the code review need finishing.

## What Changes

- The admin UI sends security headers, and answers with a status that tells a request that's too big (413) and a failure on the NAS (500) from a bad request (400).
- New data folders and logs are no longer readable by other users on the NAS.
- Community files, issue and pull request templates, Dependabot and a release workflow that publishes packages from a version tag.
- A README that says what the app is, what it does and what it was tested on.
- Internal cleanups with no change in behaviour.

## Capabilities

### Modified Capabilities

- `station-admin-ui`: security headers and error statuses.

## Impact

- Code: `internal/admin`, `internal/catalog`, `internal/reqlog`, `internal/relay`, `internal/directory`, `internal/devices`, `internal/logfile`, the admin UI, `scripts/check-secrets.sh`.
- Data: none.
