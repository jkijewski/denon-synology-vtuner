# Design

## Context

All data is in `SYNOPKG_PKGVAR`, which an upgrade keeps. Station lists have had a `version` field set to 1 from the start, and `devices.json` is a bare array. The DSM upgrade order runs the new package's `preupgrade` before the old package is removed, and its binary isn't unpacked yet at that point. This order comes from Synology's documentation and hasn't been observed on a NAS.

## Goals / Non-Goals

**Goals:**
- An update never needs an export or import, including when the format changes.
- The app never overwrites data it doesn't understand.
- Forgetting a migration fails the tests.

**Non-Goals:**
- Converting data down to an older version.
- A restore screen. The copies are plain files.

## Decisions

### D1. One version number for all stored data
Separate numbers per file would add combinations to test. `schema-version` holds only the number, so that the shell script can read it. Station lists also carry it, because an export travels without the folder.

### D2. Conversion happens in vtunerd on startup
Not in `postupgrade`, so that the same code runs in Docker and on a NAS, and any start picks up unfinished work. It runs before anything else reads the data.

### D3. Check in `preupgrade`, with the binary as the second line
The script can't run the new binary yet, so the build appends `SCHEMA_VERSION` to the `common` file. If DSM's order differs from the documentation, the app's own refusal still protects the data.

### D4. Steps run one version at a time, and can resume
Each step converts every list up to its own version and then records that version. Lists are written atomically and a list already at that version is skipped, so an interrupted upgrade continues on the next start. Steps for other files must be safe to run twice.

### D5. One copy per upgrade
The copy is made once before the first change and a `schema-upgrading` marker names it, so repeated failed starts can't replace or prune it. Pruning to the last 3 copies happens only after success. Copies leave out the admin token and logs.

### D6. A failure stops the app
Serving with half-converted lists could overwrite them. A damaged list stops an upgrade for the same reason, and the error names the file. Without a pending upgrade, a damaged list behaves as before.

### D7. The rule is enforced by a shape test
`TestShape` compares the JSON fields and types of the stored structs with `testdata/shape-v<N>.txt`. A shape file is only written when it's missing, so changing the structure means raising the version. `TestSamplesFromEveryVersionLoad` upgrades a sample of each version, and `Plan.Check` requires one step per version. `settings.env` isn't a struct, so the test can't see it and that part relies on review.

## Risks / Trade-offs

- Any new field raises the version, so a package older than the data can't be installed even when the change was small. To go back, the user restores a copy. This matches V-1.
- Packages before build 0005 have no check. Installing one over newer data leaves the old app unable to load it.
