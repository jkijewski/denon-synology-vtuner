# Changelog

All notable changes to this project are listed here. The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow [Semantic Versioning](https://semver.org/). The build script copies the section for the version it builds into the package, where Package Center shows it as "What's new".

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
