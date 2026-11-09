# Spec Delta

## Purpose

Keeps the relay from being used to read the NAS or the home network, or to serve web pages, when a station list or directory entry is malicious.

## MODIFIED Requirements

### Requirement: Relay can't be used to reach the NAS or the LAN
Anyone can add a station to the public directory, and a receiver's list can be imported from a file, so the relay SHALL by default fetch stream URLs only from addresses on the internet. This applies to every redirect and playlist entry on the way to the stream. When the administrator sets `VTUNER_RELAY_LAN=true`, stations in a receiver's list that didn't come from the directory MAY be fetched from addresses on the home network. The relay SHALL NOT connect to the NAS's loopback addresses, to any address of the NAS's own network interfaces, to `0.0.0.0/8` or to link-local addresses, whatever the setting.

#### Scenario: Directory station points at the NAS
- **GIVEN** a public directory station whose URL is `http://127.0.0.1:5000/`
- **WHEN** a client requests its relay URL
- **THEN** the relay answers with an error and makes no connection to that address

#### Scenario: Redirect or playlist leads inward
- **GIVEN** a public directory station whose server redirects to, or whose playlist lists, an address on the LAN
- **WHEN** a client requests its relay URL
- **THEN** the relay answers with an error and makes no connection to that address

#### Scenario: Imported station on the LAN
- **GIVEN** `VTUNER_RELAY_LAN` is not set, and an imported list has a station at `http://192.168.1.20/live.mp3` with the relay on
- **WHEN** a client requests its relay URL
- **THEN** the relay answers with an error, makes no connection to that address, and the log names the setting

#### Scenario: Station on the LAN with the setting on
- **GIVEN** `VTUNER_RELAY_LAN=true`, and a station at `http://192.168.1.20/live.mp3` with the relay on
- **WHEN** the receiver plays it
- **THEN** the relay streams it

#### Scenario: Station at the NAS's own address
- **GIVEN** `VTUNER_RELAY_LAN=true`, the NAS has the address `192.168.1.10`, and a station's URL is `http://192.168.1.10:5000/`
- **WHEN** a client requests its relay URL
- **THEN** the relay answers with an error and makes no connection to that address

## ADDED Requirements

### Requirement: Relayed answers are audio
The relay SHALL send a relayed answer with its upstream content type only when that type is an audio type, `application/ogg` or `application/octet-stream`, and SHALL send `audio/mpeg` otherwise. Every relayed answer SHALL tell browsers not to guess the type and not to run any content.

#### Scenario: Upstream answers with a web page
- **GIVEN** a relayed station's server answers with `Content-Type: text/html`
- **WHEN** a browser requests its relay URL
- **THEN** the answer has `Content-Type: audio/mpeg`, `X-Content-Type-Options: nosniff` and a `sandbox` content security policy

### Requirement: Relayed streams per client
The relay SHALL limit the relayed streams of one client address to 2 at a time, and SHALL check this limit before taking one of the overall slots.

#### Scenario: One client asks for a third stream
- **GIVEN** the client `192.168.1.50` plays 2 relayed streams
- **WHEN** it requests a third
- **THEN** the relay answers 503, and another client can still start a stream

### Requirement: Test URL reaches the home network
**Test URL** in the admin UI SHALL be able to test a URL on the home network, but not on the NAS itself, whatever `VTUNER_RELAY_LAN` says.

#### Scenario: Testing a LAN station
- **GIVEN** `VTUNER_RELAY_LAN` is not set
- **WHEN** an administrator tests `http://192.168.1.20/live.mp3`
- **THEN** the test connects to that server and reports the result
