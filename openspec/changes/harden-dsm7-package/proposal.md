# Proposal

## Why

A review of build 0005 against Synology's DSM 7 Developer Guide found gaps. Manual install works, but some behaviour differs from the guide and its publication review checklist, and the uninstall wizard promises something the package doesn't do. The user asked to implement the review's recommendations.

## What Changes

- **Firewall registration:** the package registers its local port with DSM through the `port-config` resource worker (`conf/resource` and a `.sc` file written for the chosen port).
- **Uninstall choice:** the uninstall wizard asks to keep or delete the station lists and settings. Keep is the default. Only a real uninstall with Delete chosen removes data, never an upgrade.
- **Script rules:** `start-stop-status` accepts `prestart` and `prestop`, returns 1 from `status` when the process died but its pid file remains, drops the `log` action, and reports a failed start through `SYNOPKG_TEMP_LOGFILE`. The stderr log is rotated at 1 MB.
- **INFO:** drops the deprecated `thirdparty` field. Adds `changelog` from `CHANGELOG.md`, and `maintainer_url`, `support_url` and `helpurl` when the build sets `PROJECT_URL` (not stored in the repository).
- **LICENSE** is part of the `.spk`, so Package Center shows it during install.
- **Install wizard:** the port field only accepts 1024 to 65535.
- **Privilege:** `conf/privilege` no longer joins the `system` group (design D5), to be confirmed on the NAS.
- **Save behind DSM:** the admin UI sends the catalog version in an `X-Catalog-Version` header instead of `If-Match`, because DSM's web server weakens ETags when it compresses answers, which most likely made every save fail as "changed in another window" on the NAS. On a real conflict the UI offers to save anyway.
- Not in scope: the toolkit build, translations, and a package source.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `synology-package`: uninstall choice, firewall registration, DSM 7 script behaviour, package metadata.

## Impact

- `package/scripts/` (`common`, `postinst`, `postuninst`, `start-stop-status`), `package/conf/resource` (new), `package/WIZARD_UIFILES/`.
- `scripts/build-spk.sh`, new `CHANGELOG.md`.
- `docs/install-synology-package.md`.
- `internal/admin` (version header) and `internal/admin/ui/app.js`. No change to the stored data, so no data version change.
