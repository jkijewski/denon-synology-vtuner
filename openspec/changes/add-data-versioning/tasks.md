# Tasks

## 1. Data version and migration

- [x] 1.1 Add `internal/schema` with the version, the steps, the copy and the resume logic, and verify with `go test ./internal/schema`
- [x] 1.2 Upgrade the data folder before startup reads it, and verify with the built binary on a sample folder, including refusal of newer data
- [x] 1.3 Upgrade or refuse station lists in `catalog.Parse`, which also covers import, and verify with `go test ./internal/catalog`

## 2. Compatibility check on update

- [x] 2.1 Add the version to the package's `common` file at build time, and check it in `preupgrade`, and verify by running the script with older, equal, newer and no recorded versions
- [ ] 2.2 On the real NAS, install build 0005 over the old package without exporting, and verify that the lists are kept and `schema-version` holds 1
- [ ] 2.3 On the real NAS, set `schema-version` to a higher number, install a newer build, and verify that DSM shows the refusal and nothing changes; then set the number back

## 3. Rule for future changes

- [x] 3.1 Add the shape test and a version 1 sample, and verify that changing a stored field makes `go test ./internal/schema` fail
- [x] 3.2 Describe the procedure in `CONTRIBUTING.md` and the update behaviour in the install guide
