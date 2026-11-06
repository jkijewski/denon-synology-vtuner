# Spec Delta

## Purpose

Delivers the vTuner replacement as a native DSM package that installs, runs, upgrades and uninstalls through Package Center like any other Synology app.

## ADDED Requirements

### Requirement: Install from Package Center
The system SHALL be delivered as a `.spk` file that installs through Package Center's Manual Install on DSM 7.0 or later, built at least for x86_64 (for example a DS220+ on DSM 7.4).

#### Scenario: Manual install
- **WHEN** the user uploads the `.spk` for their NAS's architecture in Package Center > Manual Install
- **THEN** the package installs, starts, and shows as Running

#### Scenario: Wrong architecture
- **WHEN** the user uploads a `.spk` built for another architecture
- **THEN** Package Center refuses it before installing

### Requirement: Lifecycle from Package Center
The package SHALL start, stop and report its status through Package Center, and SHALL start automatically after a DSM reboot when it was running before.

#### Scenario: Stop
- **WHEN** the user stops the package
- **THEN** vTuner requests are no longer answered and DSM itself keeps working

#### Scenario: Reboot
- **GIVEN** the package was running
- **WHEN** the NAS reboots
- **THEN** the package is running again and answers the receiver

### Requirement: Runs without root
The package's processes SHALL run as the package's own unprivileged DSM user and SHALL not need root at runtime. The server SHALL listen on the loopback address only.

#### Scenario: Process owner
- **WHEN** the package is running
- **THEN** its processes belong to the package user, not root

### Requirement: Port 80 for vTuner names only
The vTuner host names SHALL reach the package on port 80 through a DSM Reverse Proxy rule for `denon.vtuner.com`, documented in the install wizard and the install guide, and one more rule for each other `vtuner.com` name a receiver uses. The package SHALL not take port 80 from DSM or change how DSM answers other host names.

#### Scenario: Reverse proxy rule in place
- **GIVEN** the rule forwards `denon.vtuner.com` on port 80 to `localhost:8580`
- **WHEN** the receiver requests `http://denon.vtuner.com/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0`
- **THEN** the package answers it

#### Scenario: DSM web pages unaffected
- **GIVEN** the package is installed and running
- **WHEN** a browser opens DSM on its usual address and port
- **THEN** DSM works as before

### Requirement: Configurable backend port
The package SHALL let the user choose the internal port its server listens on, and SHALL warn at install time if that port is already in use.

#### Scenario: Port conflict
- **GIVEN** port `8580` is used by another package
- **WHEN** the user installs with port `8580`
- **THEN** the installer reports the conflict and asks for another port

### Requirement: Data kept across upgrades
Upgrading the package SHALL keep the catalog, settings and logs without any export or import by the user. The package SHALL keep this data in DSM's per-package data folder (`SYNOPKG_PKGVAR`, `/var/packages/vtunerradio/var`), never in the program folder that an upgrade replaces, and its install and upgrade scripts SHALL NOT overwrite data that already exists there.

#### Scenario: Upgrade
- **WHEN** a newer `.spk` is installed over the old one
- **THEN** the catalog and settings are unchanged

#### Scenario: No export needed
- **GIVEN** a receiver list that was never exported
- **WHEN** the user installs a newer `.spk` over the old one
- **THEN** the receiver gets the same list, and the user has imported nothing

### Requirement: Uninstall warning
Uninstalling SHALL warn that all receivers' lists are deleted and point to the export in the admin UI.

#### Scenario: Uninstall
- **WHEN** the user starts uninstalling
- **THEN** the wizard explains how to export the lists first

### Requirement: Logs
The package SHALL write a service log in its data folder, rotate it by size, and make it reachable from the admin UI.

#### Scenario: Log size limit
- **WHEN** the log grows beyond its size limit
- **THEN** it is rotated and old files beyond the kept count are deleted

### Requirement: Admin API protected
The server's admin API SHALL require a random token created on first start and stored only in the package's data folder. The DSM admin page SHALL add the token after checking the DSM login.

#### Scenario: Direct call without token
- **WHEN** a request to the admin API arrives without the token
- **THEN** the server answers 403

### Requirement: No secrets in the package
The built package and the repository SHALL contain no passwords, tokens, user names or real network addresses; any value a user must supply SHALL be entered at install time or in the admin UI.

#### Scenario: Build check
- **WHEN** a release `.spk` is built
- **THEN** a check of its contents finds only placeholder addresses such as `192.168.1.10`
