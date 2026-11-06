# Spec Delta

## Purpose

Versions the stored data and converts older data, so that updates never need an export or import.

## ADDED Requirements

### Requirement: Stored data has a version
The data folder SHALL record one data version in a `schema-version` file, and each station list, including exported files, SHALL carry the same number in its `version` field. Data without a recorded version SHALL count as version 1.

#### Scenario: Data from before versions were recorded
- **GIVEN** a data folder with station lists and no `schema-version` file
- **WHEN** the app starts
- **THEN** the lists load unchanged and `schema-version` holds the current version

#### Scenario: New installation
- **GIVEN** an empty data folder
- **WHEN** the app starts
- **THEN** `schema-version` holds the current version and no copy is made

### Requirement: Older data is migrated on startup
When the stored data has an older version, the app SHALL save a copy of the station lists, receivers and settings in `backups/` before changing anything, convert the data one version at a time and record the new version, before anything else reads the data. The app SHALL keep the last 3 copies. The copies SHALL NOT contain the admin token.

#### Scenario: Update that changes the format
- **GIVEN** data at version 1 and an app at version 3
- **WHEN** the app starts
- **THEN** the data is at version 3, and `backups/` holds a copy of the version 1 data

#### Scenario: Migration fails
- **WHEN** a migration step fails
- **THEN** the app doesn't start, the log names the cause and the copy, and the next start continues from the last finished step with the same copy

### Requirement: Newer data is refused
The app SHALL NOT start on data with a newer version than it understands, and SHALL NOT change that data.

#### Scenario: Older app on newer data
- **GIVEN** data at a version newer than the app's
- **WHEN** the app starts
- **THEN** it exits with a message that says to install a newer version, and the data is unchanged

### Requirement: Import follows the data version
Importing a station list SHALL convert a file with an older version, and refuse a file with a newer version without changing the stored list.

#### Scenario: Export from a newer version
- **WHEN** the user imports a file with a newer version
- **THEN** the import is refused with a message, and the list doesn't change

#### Scenario: Export from an older version
- **WHEN** the user imports a file with an older version
- **THEN** it is converted and imported

### Requirement: Structure changes ship a migration
Every change to the structure of the stored data, including a new optional field, SHALL raise the data version and add a migration step and a sample of the previous version. The tests SHALL fail when the structure of the station lists or the receiver list changes without a new version, when a version has no step, or when a sample of an earlier version no longer loads.

#### Scenario: Field added without a migration
- **WHEN** a developer adds a stored field and leaves the data version as it is
- **THEN** `go test ./...` fails with a message that explains the procedure
