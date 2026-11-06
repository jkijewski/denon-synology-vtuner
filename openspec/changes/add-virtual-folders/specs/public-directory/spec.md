# Spec Delta

## Purpose

Lets station queries use every virtual folder filter and keeps nameless entries out of directory results.

## ADDED Requirements

### Requirement: Filtered station queries
Station queries SHALL be able to filter on name, country code, one or several genres, language, codec and minimum bitrate, and SHALL be able to set the sort order and the maximum number of stations. They SHALL use radio-browser's station search with `hidebroken=true`, and answers SHALL be cached like other directory calls.

#### Scenario: All filters
- **WHEN** a query sets country `PL`, genre `rock`, language `polish`, codec `MP3`, minimum bitrate 128, order `votes` and a maximum of 20
- **THEN** the search request carries `countrycode=PL`, `tag=rock`, `language=polish`, `codec=MP3`, `bitrateMin=128`, `order=votes`, `reverse=true` and `hidebroken=true`, and at most 20 stations are returned

#### Scenario: Several genres
- **GIVEN** station A has genre `80s`, station B has genre `90s` and station C has both
- **WHEN** a query asks for genres `80s` and `90s`
- **THEN** the directory is searched once per genre, with `tagExact=true`, and A, B and C are each returned once, in the query's sort order, up to its maximum

#### Scenario: One genre can't be read
- **GIVEN** a query with two genres, and the directory fails to answer for one of them
- **WHEN** the query runs
- **THEN** the query fails, rather than returning the stations of the other genre as the whole list

### Requirement: Sort order of station queries
The default order SHALL be most played first. Name order SHALL run from A to Z, and other orders from the highest value down.

#### Scenario: Name order
- **WHEN** a query sets order `name`
- **THEN** the search request carries `order=name` and `reverse=false`

### Requirement: Maximum counts playable stations
The maximum number of stations SHALL count playable stations, so HLS and unplayable entries don't use up places.

#### Scenario: HLS doesn't use up a place
- **GIVEN** a query has a maximum of 2 and the first of three matches is an HLS station
- **WHEN** the query runs
- **THEN** the two playable stations are returned

### Requirement: Nameless directory entries are dropped
Countries, genres, languages and stations from the directory SHALL be left out when their name is empty or made only of spaces or invisible characters, on the receiver and in the admin UI.

#### Scenario: Blank genre
- **GIVEN** the directory returns a genre whose name is only spaces
- **WHEN** the genre list is requested
- **THEN** that genre isn't in it

#### Scenario: Blank station
- **GIVEN** the directory returns a station whose name is a zero-width space
- **WHEN** a station listing is requested
- **THEN** that station isn't in it

### Requirement: Language list
The system SHALL offer the most used languages of the directory, sorted by name, for the admin UI.

#### Scenario: Languages
- **WHEN** the admin UI asks for the language list
- **THEN** it gets language names with station counts, sorted by name
