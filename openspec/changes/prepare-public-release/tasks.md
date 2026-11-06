# Tasks

## 1. Code

- [x] 1.1 Split the admin routing and catalog validation into smaller functions, and verify with `go test ./...`
- [x] 1.2 Add the security headers and error statuses, and verify with `go test ./internal/admin`
- [x] 1.3 Make the folder tree usable by keyboard and name the icon buttons, and verify in a browser
- [ ] 1.4 On the NAS, verify that the admin UI still opens and saves with the security headers

## 2. Repository

- [x] 2.1 Add the community files, templates, Dependabot and the release workflow
- [x] 2.2 Rewrite the README with screenshots made from example data, and verify with `scripts/check-secrets.sh`
- [x] 2.3 Start the changelog at 1.0.0, tag releases as `v1.0.0`, and add static checks to CI
- [ ] 2.4 After publishing, turn on private vulnerability reporting and set the repository's topics
