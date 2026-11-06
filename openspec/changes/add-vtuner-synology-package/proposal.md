# Proposal

## Why

The public vTuner directory that older Denon receivers use for their "Internet Radio" menu is unreliable or paywalled, so the receiver can no longer list or play stations. The receiver's DNS already sends `*.vtuner.com` to the Synology NAS (see `docs/redirect-denon-vtuner-to-synology.md`), but nothing on the NAS answers yet. This change adds that missing piece as a native Synology package.

## What Changes

- New DSM package (`.spk`, installed through Package Center) that answers the receiver's vTuner requests on port 80 of the NAS and returns a user-curated station directory in the XML format the receiver expects.
- A station catalog: stations grouped in folders, stored on the NAS, editable without rebuilding or reinstalling, with import and export.
- A separate station list for each receiver. The receiver sends its own ID (its MAC address) when it logs in, and each receiver gets its own folders, for example favourites by genre and by country. A receiver seen for the first time starts with a copy of the default list.
- Browsing the public radio-browser.info directory by country and by genre on the receiver and in the admin UI, and searching it by name in the admin UI. The receiver's Search covers only its own list. Stations that aren't listed can be added by URL.
- A web admin UI, opened from the DSM main menu, to browse the directory, add, edit, reorder and test stations, manage receivers, and see the receivers' recent requests.
- A stream relay that lets the receiver play stations that are only offered over HTTPS or in a form the receiver can't handle (see assumption A5).
- Search over the local catalog, for receivers that show the "Search" menu entry. The entry is off by default (D-12).
- Virtual folders: a set of filters that defines a folder listing the matching public directory stations (D-11).
- Not in scope: setting up DNS (already covered by the docs guide), changing receiver firmware, transcoding audio, multi-room or playback control. Publishing the repository and a package source is a later step; this change keeps the code free of personal data so that it is possible.

### Product decisions

Answered by the user on 2026-11-05 unless marked as an assumption.

- **D-1 Target.** Synology DS220+ (Intel, x86_64) on DSM 7.4. The package is built for x86_64; other architectures are optional build targets.
- **D-2 Receiver.** Denon RCD-N7. Its exact requests are not captured yet, so the request log is part of the first version. *Assumption:* it can't play HTTPS streams, so the relay is included.
- **D-3 Port 80.** DSM's own web server always holds port 80 on DSM 7, so the app runs on its own local port and a DSM Reverse Proxy rule for each vtuner.com name forwards those names to it.
- **D-4 Station sources.** Browse the public directory by country and genre, search it by name (in the admin UI), and add own URLs for stations it doesn't list.
- **D-5 Lists.** Up to about 50 favourite stations per receiver, in folders by genre and by country. One station can be in several folders.
- **D-6 Separate list per receiver.** Each receiver, identified by the ID it sends, has its own list.
- **D-7 Admin UI.** A web interface for browsing and editing is required. Only DSM administrators (which includes the NAS owner) can use it.
- **D-8 Other brands.** Other vTuner receivers (Marantz, Yamaha) should work where the protocol is the same, but only the Denon is tested on real hardware.
- **D-9 Distribution.** Installed by hand at first. The repository and the package may be published later, so nothing personal goes into the code, the package or the docs.
- **D-10 Language.** English only.
- **D-11 Virtual folders.** A set of filters can define a folder that lists the public directory stations matching them (user request, 2026-11-06). *Assumption:* the filters are the ones the directory page already has, and they cover the public directory only, not the receiver's own stations.
- **D-12 No Search row by default.** The Denon RCD-N7 showed a row with no name that lists nothing, so the Search entry is off unless the `-search` option turns it on (user request, 2026-11-06). *Assumption:* the Search item is that row. It is the only root item without a `Title`.

## Capabilities

### New Capabilities

- `vtuner-protocol`: How the NAS answers the receiver's vTuner HTTP requests (hosts, paths, handshake, menus, paging, station details, search, XML format, fallback for unknown requests, request capture).
- `receiver-profiles`: Telling receivers apart by the ID they send, and keeping one station list per receiver.
- `public-directory`: Browsing the public radio-browser.info directory by country and genre (receiver and admin UI) and searching it by name (admin UI).
- `station-catalog`: One receiver's stored list of folders and stations: fields, stable ids, ordering, persistence, validation, import and export.
- `station-admin-ui`: The web UI used from a browser to manage receivers and their lists, browse the public directory, test stations and view recent receiver requests.
- `synology-package`: Packaging and lifecycle on DSM: install, start and stop, upgrade, uninstall, data location, unprivileged runtime, port 80 integration, logs.
- `stream-relay`: Relaying a station's stream to the receiver over plain HTTP when the receiver can't use the original URL.

### Modified Capabilities

None. There are no existing specs yet.

## Impact

- New code: a server program (vTuner endpoints, admin API, relay, directory client), a static admin UI, and a DSM package layout (`INFO`, scripts, `conf/privilege`, wizard files, icons).
- New build: a script that builds one `.spk` per architecture, and a check that fails the build on anything that isn't placeholder data.
- DSM: one Reverse Proxy rule for `denon.vtuner.com` on port 80, created by the user after installing, plus one for each other `vtuner.com` name a receiver uses. DSM's own web pages are not affected.
- Network: the NAS calls the public radio-browser.info API over HTTPS when the directory is browsed.
- Docs: an install and usage guide and a testing guide in `docs/`, and `CONTRIBUTING.md`.
- Testing: a test receiver (`cmd/fakereceiver`), a Docker Compose test environment that needs no NAS, and a CI workflow on GitHub.
