# Spec Delta

## Purpose

Limits what a browser lets the admin UI load, and makes its error statuses say what went wrong.

## ADDED Requirements

### Requirement: Security headers
Every admin UI answer SHALL carry a content security policy that lets the page load only its own files, and SHALL tell browsers not to guess content types and not to send a referrer.

#### Scenario: Opening the page
- **WHEN** a browser opens the admin UI
- **THEN** the answer carries the policy, and the page works with it

### Requirement: Error statuses
The admin API SHALL answer 413 to a request that's too big, 500 to a failure reading or writing files on the NAS, and 400 to other bad requests.

#### Scenario: Upload too big
- **WHEN** a list larger than the limit is imported
- **THEN** the answer is 413

#### Scenario: Disk failure
- **GIVEN** the list can't be written to disk
- **WHEN** the user saves
- **THEN** the answer is 500 and the message names the failure
