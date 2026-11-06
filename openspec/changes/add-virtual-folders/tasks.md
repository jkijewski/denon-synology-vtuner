# Tasks

## 1. Rows without a name

- [x] 1.1 Find the cause: compare the XML of every menu with what the RCD-N7 shows, and check whether package 0003 has the `-search` option
- [x] 1.2 Drop nameless genres, languages and stations in the directory client, and verify with `go test ./internal/directory`
- [x] 1.3 Refuse names with only spaces or invisible characters in the catalog and the admin API, and verify with `go test ./internal/catalog ./internal/admin`
- [x] 1.4 Filter every receiver answer, add `Title` to `Search` and `Display` items, and add a test that walks all menus, and verify with `go test ./internal/vtuner`

## 2. Catalog

- [x] 2.1 Add `language`, `minBitrate`, `order` and `limit` to the filter with validation, allow any one filter, and verify with `go test ./internal/catalog`
- [x] 2.2 Add a test that lists from older versions still load, and verify with `go test ./internal/catalog`

## 3. Directory

- [x] 3.1 Extend `Query`, map filters with `FromFilter`, ask for twice the limit, add `Languages`, and verify with `go test ./internal/directory`

## 4. Receiver and admin API

- [x] 4.1 List virtual folders with the new filters, showing an empty list when the directory can't answer, and verify with `go test ./internal/vtuner`
- [x] 4.2 Add the preview and languages endpoints, and test export and import with the new fields, and verify with `go test ./internal/admin`

## 5. Admin UI and docs

- [x] 5.1 Add the new form fields, placement, Preview and a badge, and verify in a browser against the test environment
- [x] 5.2 Make `fakereceiver -auto` fail on rows with no name and walk sub-folders, and verify it against the test environment on port 8000
- [x] 5.3 Update `docs/install-synology-package.md` and `docs/testing.md`, and verify with `scripts/check-secrets.sh`

## 6. Package and hardware

- [x] 6.1 Run `go vet ./...` and `go test ./...`, and verify both pass
- [x] 6.2 Build the package with `make spk BUILD=4`, and verify the build finishes and its secrets check passes
- [ ] 6.3 On the real RCD-N7 with package 0004, verify that the row with no name is gone
- [ ] 6.4 On the real RCD-N7, verify that a virtual folder appears, opens and plays, including one inside a folder

## 7. Several genres

- [x] 7.1 Replace `genre` with a `genres` list of up to 10, raise the data version to 2 with a migration, a shape file and a sample, and verify with `go test ./internal/catalog ./internal/schema`
- [x] 7.2 Merge one directory search per genre, without repeats and in the folder's order, and verify with `go test ./internal/directory ./internal/vtuner`
- [x] 7.3 Add the genre picker to the virtual folder form and show the genres in the summary, and verify in a browser against the test environment
- [x] 7.4 Start the service on a data folder of version 1, and verify the virtual folder keeps its genre, the version is 2 and a copy of the old data is in `backups`
- [x] 7.5 Update `docs/install-synology-package.md` and `CHANGELOG.md`, and build the package with `make spk BUILD=8`
- [x] 7.6 On the NAS, install package 0008 over 0007, and verify existing virtual folders are still there with their genres
- [x] 7.7 On the real RCD-N7, verify that a folder with 80s and 90s opens and plays
