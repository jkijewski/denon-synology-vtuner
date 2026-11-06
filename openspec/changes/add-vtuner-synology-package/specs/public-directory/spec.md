# Spec Delta

## Purpose

Lets the user find stations in the public radio-browser.info directory: by country and genre on the receiver, and by country, genre or name in the admin UI.

## ADDED Requirements

### Requirement: Browse by country on the receiver
The root menu SHALL offer "Browse by country", listing countries in alphabetical order, each opening that country's stations, most popular first.

#### Scenario: Open a country
- **WHEN** the receiver opens Browse by country, then "Poland"
- **THEN** it lists stations from Poland, most popular first

### Requirement: Clean country list
The country list, on the receiver and in the admin UI, SHALL leave out entries without a name, SHALL show each country once (entries for the same country are merged and their station counts added), and SHALL use the common short name where the official name is long, for example "United Kingdom" instead of "The United Kingdom Of Great Britain And Northern Ireland".

#### Scenario: Unnamed and duplicate entries
- **GIVEN** the directory returns an entry without a name and "Türkiye" twice
- **WHEN** the receiver opens Browse by country
- **THEN** the list has no blank entry and shows "Türkiye" once

#### Scenario: Long official name
- **WHEN** the receiver opens Browse by country
- **THEN** it shows "United Kingdom", not a shortened official name ending in "..."

### Requirement: Item counts on directory menus
Directory menus SHALL report their size to the receiver: "Browse by country" and "Browse by genre" report how many countries or genres they list, and each country or genre reports its station count, capped at the number of stations a listing shows. When the directory doesn't answer within 2 seconds, the root menu SHALL still be shown, with the counts left at 0.

#### Scenario: Browse menus
- **WHEN** the receiver opens the root menu and the directory lists 228 countries and 60 genres
- **THEN** "Browse by country" reports 228 items and "Browse by genre" reports 60

#### Scenario: Large country
- **GIVEN** Germany has 6313 stations in the directory and a listing shows at most 200
- **WHEN** the receiver opens Browse by country
- **THEN** Germany reports 200 items

### Requirement: Browse by genre on the receiver
The root menu SHALL offer "Browse by genre", listing the most used genres in alphabetical order, each opening its stations, most popular first.

#### Scenario: Open a genre
- **WHEN** the receiver opens Browse by genre, then "jazz"
- **THEN** it lists jazz stations

### Requirement: Only playable stations
Directory listings SHALL leave out stations the directory marks as broken, HLS streams, and codecs other than MP3 and AAC.

#### Scenario: HLS station
- **GIVEN** a directory station that only offers an HLS stream
- **WHEN** its country is listed
- **THEN** the station is not in the list

### Requirement: HTTPS directory stations play through the relay
A directory station whose stream URL uses HTTPS SHALL be given to the receiver as a relay URL on the NAS.

#### Scenario: HTTPS directory station
- **WHEN** a listed directory station has URL `https://stream.example.com/live.mp3`
- **THEN** the receiver gets an `http://` relay URL on a vtuner.com host name

### Requirement: Directory unavailable
When the directory can't be reached, the receiver SHALL get a single message item instead of an error, and the receiver's own list SHALL keep working.

#### Scenario: No internet
- **GIVEN** the NAS can't reach radio-browser.info
- **WHEN** the receiver opens Browse by country
- **THEN** it shows "Public directory not reachable. Try again later."

### Requirement: Polite use of the public API
The system SHALL send a descriptive User-Agent, find the API servers through `all.api.radio-browser.info`, try the next server on failure, and cache answers for at least 30 minutes.

#### Scenario: Repeated listing
- **WHEN** the receiver opens the same country twice within 30 minutes
- **THEN** the directory is called only once

### Requirement: Directory can be turned off
The administrator SHALL be able to turn the directory off in the package settings, which removes the browse entries from the root menu.

#### Scenario: Turned off
- **GIVEN** the directory is set to `off`
- **WHEN** the receiver opens the root menu
- **THEN** it shows only the receiver's folders and Search

### Requirement: Server discovery recovers from failure
When discovery finds no servers, the system SHALL use the fallback server for that request only and SHALL look again later. It SHALL keep using a server list it already has when a refresh fails, and SHALL refresh the list hourly. A slow lookup SHALL NOT hold up answers that are already cached.

#### Scenario: DNS not ready at boot
- **GIVEN** the first directory request after start fails to look up the servers
- **WHEN** a later request finds DNS working
- **THEN** the discovered servers are used, with failover between them
