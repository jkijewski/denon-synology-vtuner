# Proposal

## Why

A security review of 1.0.0 asked what happens if a station list, a public directory entry or a stream server is malicious. It found no way to take control of the NAS, but it found these gaps:

- **F1:** the relay for stations in a receiver's list that didn't come from the directory could reach any address on the home network, including the NAS's own network address (only its loopback address was refused). An imported file decides that itself, by leaving `source` empty and turning the relay on. Anyone on the LAN can then read the answers through `/relay/`, which has no login.
- **F2:** the relay passed on the upstream content type, so a server answering `text/html` was shown as a page under the NAS's `vtuner.com` name.
- **F3:** the public directory cache could hold 500 answers of up to 16 MB each, and receivers on the LAN can add entries without a login.
- **F4:** one client on the LAN could hold every relay slot, and could make up its address with `X-Forwarded-For`.

## What Changes

- Stations from a receiver's list are relayed from internet addresses only, unless the administrator sets `VTUNER_RELAY_LAN=true`. Directory stations are internet-only as before.
- The relay never connects to the NAS's own interface addresses or `0.0.0.0/8`, whatever the setting.
- **Test URL** in the admin UI may still reach the home network, without the NAS itself, because only an administrator can use it.
- Relayed answers are sent as an audio type, with `nosniff` and a sandbox policy.
- At most 2 relayed streams per client address.
- The directory cache is limited to 64 MB in total and 8 MB per answer, dropping the oldest answers first.
- The client address is the last `X-Forwarded-For` entry, which the reverse proxy adds, before `X-Real-IP`.
- Not in scope: limiting how many receivers the LAN can register (the existing limit of 32 stays; deleting lists automatically could delete the user's work), and signing release packages.

## Capabilities

### New Capabilities

None.

### Modified Capabilities

- `stream-relay`: which addresses the relay may reach, the content type of relayed answers, a per-client limit.
- `public-directory`: a bounded cache.
- `receiver-profiles`: which forwarded address identifies a receiver.

## Impact

- `internal/relay`, `internal/directory`, `internal/reqlog`, `cmd/vtunerd` (new `-relay-lan` flag, also read from `VTUNER_RELAY_LAN`).
- `package/scripts/start-stop-status` and `postinst` (new setting, default `false`), `compose.yaml`.
- Docs: `docs/install-synology-package.md`, `docs/testing.md`, `CHANGELOG.md`.
- No change to the stored data, so no data version change. After the update, a relayed station on the home network stops playing until the setting is turned on; the changelog and the log message say so.
