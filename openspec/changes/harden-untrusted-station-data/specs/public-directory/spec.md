# Spec Delta

## Purpose

Bounds the memory that directory answers use, because receivers on the LAN can trigger new directory queries without a login.

## ADDED Requirements

### Requirement: Bounded directory cache
The system SHALL read at most 8 MB of one directory answer, and SHALL keep at most 64 MB and 500 answers in its cache, dropping expired answers first and then the oldest ones.

#### Scenario: Many different searches
- **WHEN** clients on the LAN send many searches with different texts
- **THEN** the cache stays within 64 MB, and the most recent answers stay cached

#### Scenario: Oversized answer
- **WHEN** a directory server sends an answer larger than 8 MB
- **THEN** the system treats the request as failed and caches nothing
