# Tasks

## 1. Package metadata

- [x] 1.1 Drop `thirdparty` from INFO, add `changelog` from `CHANGELOG.md` and the optional `PROJECT_URL` links, and verify the generated INFO with and without `PROJECT_URL`
- [x] 1.2 Put `LICENSE` in the `.spk`, and verify with `tar -tvf`

## 2. Scripts

- [x] 2.1 Add `conf/resource` with port-config, write the `.sc` file in `postinst` and in the build, and verify the file for a wizard port and after a simulated upgrade
- [x] 2.2 Handle `prestart`/`prestop`, status 1 and the stderr log rotation in `start-stop-status`, and verify each action in a temporary folder
- [x] 2.3 Limit the install wizard port to 1024 to 65535, and verify the regex against every number from 0 to 69999
- [x] 2.4 Delete data in `postuninst` only on uninstall with Delete chosen, and verify upgrade, keep, delete and a symlinked folder in a temporary folder
- [x] 2.5 Remove `join-groupname` from `conf/privilege`

## 3. On the real NAS

- [ ] 3.0 Install build 0007 (no system group), open vTuner Radio from the DSM main menu, and verify that it signs in; if not, restore `join-groupname` and copy the login Details into the change
- [ ] 3.1 Install build 0006 over 0005, and verify that it runs, the lists are kept, and **vTuner Radio** shows in the firewall's application list with the chosen port
- [ ] 3.2 Uninstall with **Keep**, and verify that `/var/packages/vtunerradio/var` still holds the lists; reinstall and verify the receiver gets them
- [ ] 3.3 Uninstall with **Delete**, and verify that the folder is empty and the firewall entry is gone
- [ ] 3.4 Check that Package Center shows the license and the changelog

## 4. Admin UI

- [x] 4.1 Send the catalog version in `X-Catalog-Version`, accept weak `If-Match`, and offer Save anyway on a conflict; verify with `go test ./internal/admin`
- [ ] 4.2 On the NAS with build 0007, edit and save a list twice in a row, and verify both saves succeed

## 5. Docs

- [x] 5.1 Describe the uninstall choice and the firewall entry in the install guide
