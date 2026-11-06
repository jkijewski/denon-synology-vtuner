# Spec Delta

## Purpose

Tells receivers apart by the ID they send to vTuner, so that each receiver in the home has its own station list.

## ADDED Requirements

### Requirement: Identify the receiver at login
The system SHALL identify a receiver by the MAC address it sends in the vTuner login request, and SHALL keep one station list per receiver.

#### Scenario: Two receivers
- **GIVEN** receivers with IDs `00:11:22:33:44:55` and `00:11:22:33:44:66`
- **WHEN** each opens Internet Radio
- **THEN** each sees its own folders and stations

### Requirement: New receiver starts from the default list
A receiver seen for the first time SHALL get its own copy of the default list. Later changes to either list SHALL not affect the other.

#### Scenario: First login
- **WHEN** a receiver with an unknown ID logs in
- **THEN** it is added to the receiver list and its menu shows the folders of the default list

### Requirement: Receiver kept through the session
Every menu, search and play URL the system gives a receiver SHALL carry that receiver's ID, so later requests use the same list. A request without an ID SHALL use the receiver last seen at the same IP address, or the default list.

#### Scenario: Request without an ID
- **GIVEN** receiver `00:11:22:33:44:55` last logged in from `192.168.1.50`
- **WHEN** a request without an ID arrives from `192.168.1.50`
- **THEN** that receiver's list is used

#### Scenario: Unknown sender
- **WHEN** a request without an ID arrives from an address no receiver has used
- **THEN** the default list is used

### Requirement: Manage receivers
Administrators SHALL be able to rename a receiver, copy another receiver's list over it, and remove a receiver with its list. The default list SHALL not be removable.

#### Scenario: Copy a list between receivers
- **GIVEN** receivers "Living room" and "Kitchen"
- **WHEN** an administrator selects "Kitchen" and chooses to copy the list from "Living room"
- **THEN** a confirmation names both receivers, for example "Replace the list of "Kitchen" with a copy of the list of "Living room"?", and says the source list is not changed
- **AND** after confirming, "Kitchen" has a copy of the folders and stations of "Living room", and later changes to either list don't affect the other

#### Scenario: Copy cancelled
- **WHEN** the administrator cancels the confirmation
- **THEN** neither list changes

#### Scenario: Remove a receiver
- **WHEN** an administrator removes receiver `00:11:22:33:44:66`
- **THEN** its list is deleted, and if it connects again it starts from the default list

### Requirement: Limit on receivers
The system SHALL keep at most 32 lists, counting the default list, because any device on the network can send a login with a new ID. A receiver beyond the limit SHALL use the default list, and receivers already known SHALL keep working.

#### Scenario: Limit reached
- **GIVEN** 32 lists exist
- **WHEN** a device logs in with a new ID
- **THEN** no new list is created and the device gets the default list

### Requirement: Damaged receiver list doesn't stop the service
If the file listing the receivers is damaged, the system SHALL start anyway, keep the damaged file for inspection, and rebuild the list of receivers from the station list files. Receiver names and last-seen data MAY be lost, but no station list SHALL be lost.

#### Scenario: Empty file after a power cut
- **GIVEN** `devices.json` is empty and `devices/001122334455.json` exists
- **WHEN** the service starts
- **THEN** it runs, and receiver `001122334455` still has its list
