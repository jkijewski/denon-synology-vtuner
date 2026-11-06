# Spec Delta

## Purpose

Adds a compatibility check when a package is installed over an older one.

## ADDED Requirements

### Requirement: Compatibility check on update
Before an upgrade changes anything, the new package SHALL compare the data version on the NAS with the version it supports, and SHALL stop the upgrade with a message if the data is newer. Older data SHALL NOT stop it.

#### Scenario: Package too old for the data
- **GIVEN** data at a newer version than the package supports
- **WHEN** the user installs the package over the current one
- **THEN** DSM stops the installation and shows that a newer package is needed, and the data and the installed package are unchanged

#### Scenario: Normal update
- **GIVEN** data at the same or an older version than the package supports
- **WHEN** the user installs the package over the current one
- **THEN** the update continues and the data is kept
