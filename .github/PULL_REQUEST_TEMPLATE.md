## What changes and why

## How it was tested

## Checklist

- [ ] `gofmt -l .` prints nothing, and `go vet ./...` and `go test ./...` pass
- [ ] `sh scripts/check-secrets.sh` passes: no real addresses, MAC addresses, host names, user names or passwords
- [ ] A change in behaviour has an OpenSpec change in `openspec/changes/`
- [ ] A change to stored data raises the data version and ships a migration (see `CONTRIBUTING.md`)
- [ ] A change users will notice is noted for the next `CHANGELOG.md` section
