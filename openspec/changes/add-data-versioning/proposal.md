# Proposal

## Why

Updating the package keeps the data folder, so nothing has to be exported. But nothing protects that data when its format changes. Station lists are read strictly, so a package that doesn't know a field stops loading the list, and a package that changes a field has no way to convert what's already stored. The user wants updates to stay free of manual export and import as the format evolves.

## What Changes

- The stored data gets one version number. It's in a `schema-version` file in the data folder and in the `version` field of each station list, including exports. Data from before this change counts as version 1.
- **Compatibility check on update:** a package that's too old for the data on the NAS refuses to install. As a second line, the app refuses to start on data newer than it understands.
- **Migration procedure:** when the data is older, the app saves a copy, converts the data step by step on its first start and records the new version. If that fails, it stays stopped and keeps the copy.
- **Standing rule:** every change to the stored structure ships a migration, even a new optional field. A test fails when the structure changes without one, and `CONTRIBUTING.md` describes the procedure.
- Importing a file follows the same rules: older files are converted, newer ones refused.
- Not in scope: going back to an older version automatically, and a screen for restoring copies. Packages before this change don't make the check.

### Product decisions

- **V-1 What needs a migration.** Every structure change, as the user asked on 2026-11-06. A new optional field raises the version too, even when its step changes nothing, because older builds refuse unknown fields.
- **V-2 Current version stays 1.** No format changes now, so no conversion runs on existing data (assumption).

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `synology-package`: compatibility check on update.
- `station-catalog`: data version, migration and import rules.

## Impact

- New package `internal/schema`, called from `cmd/vtunerd` at startup and from `catalog.Parse`.
- `package/scripts/preupgrade` and `scripts/build-spk.sh`.
- New files in the data folder: `schema-version`, `backups/`, and `schema-upgrading` during an upgrade.
