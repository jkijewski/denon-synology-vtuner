# Tasks

## 1. Project setup

- [x] 1.1 Create the Go module (`denon-synology-vtuner`), folder layout (`cmd/`, `internal/`, `package/`, `scripts/`) and `.gitignore`, and verify `go build ./...` succeeds
- [x] 1.2 Add `Makefile` targets `test`, `vet`, `spk`, `check`, `run`, and verify `make test` passes

## 2. Station catalog and receivers

- [x] 2.1 Implement the catalog (fields, nested folders, stable ids, validation, atomic save, merge import) and verify `go test ./internal/catalog` passes
- [x] 2.2 Implement the receiver registry (MAC to key, copy of the default list, IP fallback, rename, remove) and verify `go test ./internal/devices` passes

## 3. vTuner protocol

- [x] 3.1 Implement host matching, case-insensitive paths, token handshake, root menu, folder listing, paging, station lookup, search, XML rendering and the unknown-request fallback, and verify `go test ./internal/vtuner` passes
- [x] 3.2 Implement the bounded request log and verify `go test ./internal/reqlog` passes

## 4. Public directory

- [x] 4.1 Implement the radio-browser client (server discovery, failover, cache, filters, numeric ids) and verify `go test ./internal/directory` passes against a fake server
- [x] 4.2 Add Browse by country and Browse by genre to the receiver menu, and verify the directory tests in `go test ./internal/vtuner` pass
- [x] 4.3 Clean the country list (no blank or duplicate entries, short names) and report item counts on the browse menus, and verify `go test ./internal/directory ./internal/vtuner` passes

## 5. Stream relay

- [x] 5.1 Implement the relay (known stations only, playlists, redirects, limit, disconnect clean-up) and the URL test, and verify `go test ./internal/relay` passes

## 6. Admin UI

- [x] 6.1 Implement the admin API (receivers, lists, import, export, directory, URL test, request log, service log) behind the token, and verify `go test ./internal/admin` passes
- [x] 6.2 Build the web UI (receiver selector, folder tree, station editor, directory browser, requests, import and export, log) and verify it in a browser against `make run`
- [x] 6.3 Implement the DSM CGI login check and verify `go test ./internal/dsmauth` passes with fake DSM CGIs
- [x] 6.4 Add the directory folder picker, folder labels on directory results and the Format filter, and verify them in a browser and with `go test ./internal/directory ./internal/admin`

## 7. Synology package

- [x] 7.1 Write `INFO` generation, `conf/privilege`, start-stop-status and install scripts, wizards, DSM app config and icons, and verify the scripts start, report and stop the server outside DSM
- [x] 7.2 Write `scripts/build-spk.sh` and `scripts/check-secrets.sh`, and verify `make spk` produces `dist/vtunerradio-0.1.0-0001-x86_64.spk` with the secrets check passing
- [ ] 7.3 Install the `.spk` on the DS220+ (DSM 7.4) and verify it shows as Running, survives a restart, and that the DSM main menu entry opens the admin UI for an administrator and refuses a non-administrator
- [ ] 7.4 Add the Reverse Proxy rule and verify `curl -H "Host: denon.vtuner.com" "http://192.168.1.10/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0"` returns the token, and that DSM's own pages still work

## 8. Documentation

- [x] 8.1 Write `docs/install-synology-package.md` (build, install, Reverse Proxy rule, first run, troubleshooting)

## 9. Test environment for contributors

- [x] 9.1 Add `cmd/fakereceiver` (interactive and `-auto` modes) and verify `fakereceiver -auto -no-directory` passes against `make run`
- [x] 9.2 Add `Dockerfile`, `compose.yaml` and `test/nginx.conf` (nginx in the app's network namespace, like DSM's reverse proxy) and verify `docker compose up --build -d` followed by `make smoke` passes
- [x] 9.3 Add the CI workflow (format, vet, race tests, secrets check, package build, Docker smoke test), `CONTRIBUTING.md`, the receiver report issue template and `docs/testing.md`, and verify every command in `docs/testing.md` levels 1 and 2 runs as written

## 10. Receiver checks (needs the real Denon RCD-N7)

- [ ] 10.1 Open Internet Radio on the receiver and verify its requests appear in the request log with its ID; record any request that got the "unknown request" note
- [ ] 10.2 Verify browsing a folder, a country and a genre, and playing an HTTP station and an HTTPS station through the relay
- [ ] 10.3 Adjust the protocol for anything the receiver does differently, and update the vtuner-protocol spec to match

## 11. Code review fixes

- [x] 11.1 Protect the relay against reaching the NAS and LAN through public directory URLs, redirects and playlists, and end stalled streams; verify with `go test ./internal/relay ./internal/vtuner`
- [x] 11.2 Refuse empty entries and out-of-range values in catalogs, and verify with `go test ./internal/catalog`
- [x] 11.3 Make catalog updates atomic and refuse saves from an out-of-date copy, and verify with `go test -race ./internal/catalog ./internal/admin`
- [x] 11.4 Add durable writes, recovery from a damaged `devices.json`, and the limit on receivers, and verify with `go test ./internal/atomicfile ./internal/devices`
- [x] 11.5 Refuse cross-site admin requests and non-JSON bodies, and test the CGI handler and `ClientIP`; verify with `go test ./internal/dsmauth ./internal/admin ./internal/reqlog`
- [x] 11.6 Fix server discovery, the HTTPS-only check and graceful shutdown; verify with `go test ./internal/directory ./internal/relay`
- [x] 11.7 Move to Go 1.27 and add `govulncheck` to CI; verify that `govulncheck ./...` reports nothing
- [ ] 11.8 On the NAS, verify that cross-site protection doesn't refuse the admin UI's own requests (add to the checks in 7.3)

- [x] 11.9a Make the DSM login check explain a refusal without secrets, send DSM's CGIs a plain GET, and accept a missing SynoToken; verify with `go test ./internal/dsmauth`
- [ ] 11.9 On the NAS, verify that a signed-in administrator can open the admin UI with `join-groupname: system` in `conf/privilege`; if not, use the refusal page's details to decide how D6 changes

## 12. Virtual folders and the nameless menu row

- [x] 12.1 Add folder filters with validation to the catalog, and verify with `go test ./internal/catalog`
- [x] 12.2 List virtual folders on the receiver from the directory, with counts and messages, and verify with `go test ./internal/vtuner`
- [x] 12.3 Leave the Search entry out of the top menu unless `-search` is set, and verify with `go test ./internal/vtuner`
- [x] 12.4 Add virtual folders to the admin UI (create, edit, preview, save from directory filters), and verify in a browser against the test environment
- [ ] 12.5 On the real RCD-N7, verify that the row with no name is gone and that a virtual folder opens and plays

## 13. Updates keep the data

- [x] 13.1 Run the upgrade script sequence against sample data, and verify that every file in the data folder is unchanged
- [x] 13.2 Document how to update the package without export or import
- [ ] 13.3 On the real NAS, install a newer `.spk` over the old one without exporting, and verify that the lists and settings are still there

## Workflow follow-up

- Archive the change after the receiver checks pass and the user approves.
