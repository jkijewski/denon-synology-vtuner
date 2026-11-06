# Spec Delta

## Purpose

Brings the package closer to the DSM 7 Developer Guide: firewall registration, an uninstall choice, DSM 7 script actions and package metadata.

## MODIFIED Requirements

### Requirement: Uninstall warning
Uninstalling SHALL ask whether to keep or delete all receivers' lists and settings, with keep as the default, and SHALL point to the export in the admin UI. The package SHALL delete the data only on an uninstall where the user chose delete, never during an upgrade.

#### Scenario: Uninstall and keep
- **WHEN** the user uninstalls and keeps the default choice
- **THEN** the lists and settings stay on the NAS, and a later install serves them again

#### Scenario: Uninstall and delete
- **WHEN** the user uninstalls and chooses delete
- **THEN** the package's data folder is empty afterwards

#### Scenario: Upgrade
- **WHEN** a newer package is installed over the current one
- **THEN** no data is deleted

## ADDED Requirements

### Requirement: Port registered with DSM
The package SHALL register the port it listens on with DSM's firewall service list while it is installed, using the port chosen at install time, and SHALL remove the registration when it is uninstalled.

#### Scenario: Custom port
- **GIVEN** the user chose port `9123` in the install wizard
- **WHEN** the installation finishes
- **THEN** DSM's firewall lists **vTuner Radio** with TCP port `9123`

### Requirement: DSM 7 control script
The control script SHALL handle the DSM 7 actions `start`, `stop`, `status`, `prestart` and `prestop`, and `status` SHALL report 0 when running, 1 when the process died but its pid file remains, and 3 when stopped.

#### Scenario: Crashed process
- **GIVEN** the pid file names a process that no longer runs
- **WHEN** DSM asks for the status
- **THEN** the script returns 1

### Requirement: Package metadata
The package SHALL contain no deprecated INFO fields, SHALL include its license and the changelog of its version, and SHALL add project links only when the build is given them, so that the repository holds no account names.

#### Scenario: Install shows license
- **WHEN** the user installs the package
- **THEN** Package Center shows the license text
