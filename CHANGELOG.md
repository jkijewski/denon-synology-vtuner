# Changelog

All notable changes to this project are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). The build script copies the section for the version it builds into the package, where Package Center shows it as "What's new".

## [1.0.1] - 2026-11-09

Security fixes from a review of what a malicious station list or directory entry could do. None of them allowed control of the NAS.

### Changed

- The relay plays stations you added (or imported) only from servers on the internet. To relay a station from a server on your home network, set `VTUNER_RELAY_LAN=true` in `settings.env`. Stations from the public directory always stay on the internet.
- The relay never connects to the NAS's own network addresses, not only its loopback address.
- A relayed answer is always sent as audio, never as a web page.
- One receiver address can play at most 2 relayed streams at a time, so one device can't take every relay slot.
- The public directory cache is limited to 64 MB, and one directory answer to 8 MB.
- The receiver address is taken from the last `X-Forwarded-For` entry, which the reverse proxy adds, so a client can't pretend to be another receiver.

## [1.0.0] - 2026-11-06

First public release.

### Added

- Synology package for DSM 7 that answers a receiver's vTuner requests from the NAS, tested with a Denon RCD-N7 on a DS220+ with DSM 7.4.
- A station list per receiver, with folders, sub-folders and your own stations.
- Browsing the public radio-browser.info directory by country and genre on the receiver.
- Virtual folders that fill themselves from the directory by country, up to 10 genres, language, format, minimum bitrate or name, with a sort order, a maximum size and a preview.
- Playing HTTPS stations through the NAS for the receiver.
- An admin UI in the DSM main menu, for DSM administrators only, with import and export, a URL test, receiver requests and a log.
- Automatic upgrades of the stored data after a package update, with a copy of the old data kept in the `backups` folder.
- A Docker test environment and a test receiver, so that you can try it without a NAS.
