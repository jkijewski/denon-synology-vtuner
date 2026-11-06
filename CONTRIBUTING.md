# Contributing

Thanks for helping. You don't need a Synology NAS or a receiver to work on this project: the Docker test environment and the test receiver cover most of it. See [docs/testing.md](docs/testing.md).

## Ground rules

- **No personal data.** Never commit real IP addresses, MAC addresses, host names, user names, passwords or tokens, in code, docs, tests or commit messages. Use the placeholders below. `sh scripts/check-secrets.sh` checks this and runs in CI. It can't recognise host names and user names by their shape, so list your own in `.secrets-denylist`, which git ignores.

  | Kind | Placeholder |
  |---|---|
  | NAS | `192.168.1.10`, `nas.example.lan` |
  | Receiver | `192.168.1.50`, `00:11:22:33:44:55` |
  | Router | `192.168.1.1` |
  | Test computer | `192.168.1.20` |
  | Stream URLs | `http://stream.example.com/...` |
  | Password | `changeme` |

- **Spec first.** The project uses [OpenSpec](https://github.com/Fission-AI/OpenSpec). Behaviour changes start as a change proposal in `openspec/changes/` (`openspec new change <name>`, or `/opsx:propose` in Claude Code). Small fixes that don't change behaviour don't need one.
- **English** for code, docs and the UI.

Please also read the [code of conduct](CODE_OF_CONDUCT.md). To report a security problem, follow [SECURITY.md](SECURITY.md) instead of opening an issue.

## Set up

You need Go 1.27 or later. Docker is optional.

```sh
go test ./...                     # unit tests
make run                          # app on http://127.0.0.1:8580, admin UI at /admin/ (no login)
docker compose up --build -d      # full test environment, see docs/testing.md
```

## Changing the stored data

Every change to the structure of the stored data must ship a migration in the same pull request, even a new optional field. That covers station lists (`internal/catalog`), the receiver list (`internal/devices`) and `settings.env`. People update the package without exporting anything, so their data has to arrive in the new format by itself.

1. Before changing anything, save a sample of the data in the current format as `internal/schema/testdata/v<current>/`, unless one is already there. Use example data only.
2. Raise `Current` in `internal/schema/schema.go`.
3. Add a `Step` to `steps` with `To` set to the new version. `Catalog` changes one station list, and also runs on imported files. `Folder` changes the other files, and must be safe to run twice. Leave out what doesn't change.
4. Make the structure change, then record the new shape with `go test ./internal/schema -run TestShape -add-shape`.
5. Add the new version to the spec change that describes the structure change.

`go test ./...` fails if the structure changes without a new version, if a version has no step, or if a sample from an earlier version no longer loads. It can't see changes to `settings.env`, so check those yourself. Never edit a shape file or a sample of an earlier version.

## Before you open a pull request

```sh
gofmt -l .                        # must print nothing
go vet ./...
go test ./...
sh scripts/check-secrets.sh
make spk                          # optional: builds the Synology package into dist/
```

CI runs the same checks, builds packages for x86_64 and armv8, and walks the menus with the test receiver in Docker.

## Releasing

Versions follow [Semantic Versioning](https://semver.org/).

1. Add a section to `CHANGELOG.md`, headed like `## [1.1.0] - 2026-12-01`. Package Center shows it as the package's changelog.
2. Merge to `main` and wait for CI to pass.
3. Push a tag such as `v1.1.0`. The release workflow runs the checks, builds both packages and publishes them with that changelog section. It sets `PROJECT_URL`, so the package links to the project's site, issues and install guide.

The package's own version adds a build number, such as `1.1.0-0001`. For test builds of the same version, raise it: `make spk BUILD=2`.

## Project layout

| Path | What it is |
|---|---|
| `cmd/vtunerd` | The server (also the DSM admin CGI) |
| `cmd/fakereceiver` | Test receiver that walks the vTuner menus |
| `internal/vtuner` | vTuner protocol (menus, XML) |
| `internal/catalog`, `internal/devices` | Station lists, one per receiver |
| `internal/schema` | Data version, compatibility check and migrations |
| `internal/directory` | radio-browser.info client |
| `internal/relay` | Stream relay for HTTPS stations |
| `internal/admin` | Admin API and web UI |
| `internal/dsmauth` | DSM login check for the admin UI |
| `package/` | Synology package files (scripts, wizards, icons) |
| `scripts/` | Package build and personal-data check |
| `test/`, `compose.yaml`, `Dockerfile` | Docker test environment |
| `openspec/` | Specs and change proposals |
| `docs/` | Guides for users |

## Testing with a receiver

Reports from real receivers are the most useful contribution. Use the **Receiver test report** issue template, and replace your MAC address and IP addresses with placeholders before posting.
