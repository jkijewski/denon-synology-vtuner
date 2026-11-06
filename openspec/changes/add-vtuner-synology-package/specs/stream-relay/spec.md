# Spec Delta

## Purpose

Lets the receiver play stations whose original stream it cannot use, such as HTTPS-only streams, by passing the audio through the NAS over plain HTTP.

## ADDED Requirements

### Requirement: Relay per station
For a station with the relay turned on, the system SHALL give the receiver a plain HTTP URL on the NAS and SHALL pass the original stream's audio bytes through without re-encoding.

#### Scenario: HTTPS station
- **GIVEN** station `1002` has URL `https://stream.example.com/live.mp3` and the relay is on
- **WHEN** the receiver plays it
- **THEN** the receiver gets a URL like `http://denon.vtuner.com/relay/001122334455/1002`, which the NAS answers, and hears the stream

### Requirement: Follow playlists and redirects
The relay SHALL follow HTTP redirects and SHALL resolve `.m3u` and `.pls` playlist URLs to their first working stream.

#### Scenario: Playlist URL
- **GIVEN** a station URL points at a `.pls` file
- **WHEN** the receiver plays the station
- **THEN** the relay streams the first reachable entry of that playlist

### Requirement: Relay only for known stations
The relay SHALL only fetch the URL of a station in a receiver's list that has the relay turned on, or of a station in the public directory, and SHALL refuse any other target.

#### Scenario: Unknown relay id
- **WHEN** a client requests `/relay/001122334455/9999` and that receiver has no station `9999`
- **THEN** the system answers 404 and fetches nothing

#### Scenario: Relay turned off
- **WHEN** a client requests the relay URL of a station whose relay setting is off
- **THEN** the system answers 404

### Requirement: Clean up on disconnect
The relay SHALL close the upstream connection within a few seconds of the receiver disconnecting, and SHALL limit the number of simultaneous relayed streams.

#### Scenario: Receiver switches station
- **WHEN** the receiver stops playing a relayed station
- **THEN** the upstream connection is closed

#### Scenario: Limit reached
- **GIVEN** the limit is 2 streams and 2 are playing
- **WHEN** a third relayed stream is requested
- **THEN** the system answers 503

### Requirement: Relay can't be used to reach the NAS or the LAN
Anyone can add a station to the public directory, so a stream URL from it SHALL only be fetched from addresses on the internet. This applies to stations added to a receiver's list from the directory, and to every redirect and playlist entry on the way to the stream. A URL an administrator entered MAY point at the LAN, but SHALL NOT reach the NAS's own loopback address or a link-local address.

#### Scenario: Directory station points at the NAS
- **GIVEN** a public directory station whose URL is `http://127.0.0.1:5000/`
- **WHEN** a client requests its relay URL
- **THEN** the relay answers with an error and makes no connection to that address

#### Scenario: Redirect or playlist leads inward
- **GIVEN** a public directory station whose server redirects to, or whose playlist lists, an address on the LAN
- **WHEN** a client requests its relay URL
- **THEN** the relay answers with an error and makes no connection to that address

#### Scenario: Station on the LAN entered by an administrator
- **GIVEN** an administrator entered a station at `http://192.168.1.20/live.mp3` with the relay on
- **WHEN** the receiver plays it
- **THEN** the relay streams it

### Requirement: Stalled streams end
The relay SHALL end a stream when no data has moved for 30 seconds, and SHALL give up on a connection attempt after 10 seconds, so that a stalled stream doesn't hold a relay slot.

#### Scenario: Upstream stops sending
- **WHEN** a station's server stops sending data but keeps the connection open
- **THEN** the relay closes the stream within about 30 seconds and the slot is free again
