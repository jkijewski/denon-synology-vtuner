# Design

## Context

See `proposal.md` for the motivation and the product decisions. What shapes the approach:

- **DNS is solved.** The receiver already resolves `*.vtuner.com` to the NAS (`docs/redirect-denon-vtuner-to-synology.md`). It connects to port 80 and sends the vTuner host name in the `Host` header.
- **Port 80 belongs to DSM.** On DSM 7 the built-in nginx always listens on 80 and 443 (DSM redirect, Web Station, Reverse Proxy rules). A third-party package can't bind port 80 and doesn't run as root.
- **The protocol is a black box.** The receiver's requests are known from community projects that emulate vTuner for Denon, Marantz and Yamaha receivers (`/setupapp/<vendor>/asp/BrowseXML/loginXML.asp`, `navXML.asp`, `statxml.asp`, `Search.asp`, `ListOfItems` XML). They are a starting point only; the Denon RCD-N7's exact requests must be confirmed from the request log.
- **Target hardware.** DS220+ (Intel Celeron, x86_64), DSM 7.4.
- **Publishable.** No personal data in code, package or docs. The Go module path is `denon-synology-vtuner`, without a hosting account in it.

## Goals / Non-Goals

**Goals:**
- One self-contained `.spk` per architecture, with no other package needed.
- Small footprint: one process, a few MB of RAM when idle.
- The receivers' own lists keep working when the internet is down (only the streams and the public directory need it).

**Non-Goals:**
- Docker or Container Manager deployment (the user asked for a native package).
- Mirroring the public directory, audio transcoding, HTTPS on the vTuner side.
- Automating the DNS Server or Reverse Proxy setup from the package.

## Decisions

### D1. Server written in Go, shipped as a static binary
One binary (`vtunerd`) holds the vTuner endpoints, the admin API, the embedded admin UI, the directory client and the relay.
- *Why:* cross-compiles to every Synology architecture with no runtime on the NAS, low memory use, good HTTP streaming in the standard library.
- *Alternatives:* Python (needs the Python package, versions vary by DSM release); Node.js (heavier, also a dependency); PHP under Web Station (the relay and long-lived streams fit badly in PHP-FPM).

### D2. Port 80 through a DSM Reverse Proxy rule
`vtunerd` listens on `127.0.0.1:<port>` (default `8580`, chosen in the install wizard). The user adds one rule in **Control Panel > Login Portal > Advanced > Reverse Proxy**: source HTTP, `denon.vtuner.com`, port 80; destination HTTP, `localhost`, `8580`. Each further `vtuner.com` name a receiver uses needs its own rule. Other host names keep DSM's default behaviour.
- DSM on the target DS220+ rejected `*.vtuner.com` as an invalid domain name, so the design doesn't rely on a wildcard rule. The server doesn't need one: it builds the URLs it generates from the host name of the request, or from `VTUNER_HOST` (default `denon.vtuner.com`), so a receiver that logs in through a rule stays on that rule.
- A request for a name without a rule stops at DSM and never reaches `vtunerd`, so the request log can't reveal missing names. They have to be found from the DNS side.
- *Why:* it is a supported DSM feature, survives DSM updates, needs no root and no Web Station.
- *Alternatives:* bind port 80 directly (blocked, would break DSM's redirect); write nginx config from a root script (not allowed for DSM 7 third-party packages, lost on updates); Web Station web-service resource (can't express a reverse proxy to a package's own port reliably; can be revisited later to automate the rule).
- The relay also uses this route: relay URLs are `http://<vtuner host>/relay/...`, so no extra firewall rule is needed.
- Everything on this route, including the relay, is open to anyone who can reach port 80 with a Host header that matches a rule. That is meant for the LAN, so the docs say not to forward port 80 from the internet to these names.

### D3. One catalog file per receiver
`/var/packages/vtunerradio/var/devices.json` lists receivers (key, name, first/last seen, last IP). Each receiver's list is `devices/<key>.json`, where the key is the MAC address without separators, and `default.json` is the list for unknown receivers and the template for new ones. Files are written through `internal/atomicfile` (temp file, `fsync`, rename, `fsync` of the folder), kept in memory, and use a growing id counter so ids are never reused. Updates to one catalog run one at a time. Because the station lists are separate files, a damaged `devices.json` is rebuilt from them rather than stopping the service. At most 32 lists are kept, since anyone on the LAN can send a login with a new MAC.
- *Why:* fits lists of about 50 to a few hundred stations, human-readable, doubles as the export format.
- *Alternatives:* SQLite (more than needed, harder to edit by hand).

### D4. Receiver ID carried in every generated URL
The login request carries `mac=`. All `UrlDir`, search and relay URLs the server generates include `dev=<key>` (or `/relay/<key>/...`), because later requests from the receiver only follow those URLs. Requests without it fall back to the receiver last seen at that IP, then to the default list.

### D5. Public directory: radio-browser.info, cached, filtered
Server discovery follows the radio-browser docs: resolve `all.api.radio-browser.info`, reverse-resolve each address, try servers in turn. A failed lookup is not remembered: the fallback server serves that request only and the lookup is retried, and the list is refreshed hourly. Answers are cached for 30 minutes. Listings drop broken stations, HLS, and codecs other than MP3/AAC. Directory stations get a numeric id derived from their UUID, because receivers expect numeric `StationId`s. HTTPS streams are given as relay URLs. Directory URLs are untrusted, because anyone can add a station: the relay fetches them, including redirects and playlist entries, with a client whose dialer refuses every address that isn't on the internet. The check runs on the resolved address of each connection, so DNS tricks don't get around it. URLs an administrator entered may be on the LAN, but not on the NAS itself.

### D6. Admin UI is a DSM app that uses the DSM login
The package adds a DSM main menu entry for `/webman/3rdparty/vtunerradio/index.cgi/`. The CGI is the same binary (`vtunerd cgi`): it gets the caller's user from DSM's `login.cgi` and `authenticate.cgi` (the documented third-party app method), checks membership of `administrators`, then forwards the request to `vtunerd` with a random token from `var/admin.token`. The server refuses admin calls without that token. The package never stores a password. Because the CGI fetches the `SynoToken` itself, the login check alone doesn't show that a request came from the admin page, so the CGI first refuses cross-site requests with `http.CrossOriginProtection`, and the admin API accepts bodies only as `application/json`. A `PUT` carries the `ETag` of the version it edited in `If-Match`, so two windows can't overwrite each other.
- *Alternatives:* a separate password (one more secret to manage); no auth on the LAN (anyone on the network could edit the lists).

### D7. vTuner responses built from one item model
Folders, stations, search results and messages map to one item type, then to `ListOfItems` XML. Paging, escaping and title shortening happen in one place. Responses are buffered and sent with `Content-Length`, because some receiver firmware doesn't handle chunked encoding.

### D8. Build and packaging
`scripts/build-spk.sh` (or `make spk`) cross-compiles `vtunerd`, assembles `INFO`, `package.tgz`, `scripts/`, `conf/privilege`, `WIZARD_UIFILES/` and icons, and writes `dist/vtunerradio-<version>-<build>-<arch>.spk`. `scripts/check-secrets.sh` runs on the package contents and the repository and fails on any address, MAC, key or password that isn't a documented placeholder.

### D9. Docker test environment
`compose.yaml` runs `vtunerd -dev` behind nginx for testing without a NAS. It is not a way to deploy the app (see Non-Goals). `-dev` turns off the admin token, so `test/nginx.conf` answers `/admin` with 404 on the port published to the network, and serves the admin UI only on a port bound to `127.0.0.1`.

### D10. Virtual folders are catalog folders with a filter
A virtual folder is an ordinary `Folder` with a `filter` object, so ids, order, import, export and copying between receivers work unchanged. Its stations are never stored: opening it runs the same directory query as the browse menus, with the same 30-minute cache and playability rules. The menu's item count comes from the same query, under one 2-second limit for the whole listing, so a slow directory leaves counts at 0 and doesn't hold up the menu. Alternative considered: saving a station copy when the folder is created. That goes stale and is what plain folders already do.

## Risks / Trade-offs

- [The RCD-N7 uses paths or parameters not in the community notes] → the request log and the "valid empty list" fallback let us see and add them without breaking the menu.
- [The RCD-N7 doesn't send `mac=`, or sends it in another form] → requests fall back to the receiver last seen at that IP, then to the default list; the request log shows what it sends.
- [The CGI runs as a different user and can't read `admin.token`] → first thing to verify on the NAS (task 7.3); fix by adjusting the token file's group.
- [DSM's login CGIs refuse the package user] → first seen on the DS220+ as "Please sign in" for a signed-in administrator. A public report for DSM 7.3.2 describes `login.cgi` as mode `0550 system:system` and `authenticate.cgi` exiting with 5 when not run as root. `conf/privilege` therefore adds `join-groupname: system`, a community-reported setting that Synology's privilege docs don't list, so it is unconfirmed until task 11.9 passes. The check also takes a SynoToken from the request and carries on without one, and a refusal page lists what it saw, with no cookies, tokens or CGI output. If the setting isn't enough, D6 has to change, for example to a separate admin password.
- [DSM's reverse proxy buffers or times out long streams] → relay through nginx is the same as for any proxied stream; if needed, raise the timeouts in the rule's advanced settings (documented).
- [Receiver can't play AAC streams] → the URL test warns; the codec filter can be narrowed to MP3 only.
- [radio-browser.info is down or rate-limits] → caching, server failover, and the receiver's own list keeps working.
- [The receiver uses another `vtuner.com` name that has no rule] → the request fails at DSM and the request log stays empty; the reverse proxy guide shows how to find the name and add a rule.
- [Unsigned package shows a warning on install] → accepted; documented in the install guide.

## Migration Plan

New package, nothing to migrate. Rollout: install the `.spk`, add the Reverse Proxy rule, open Internet Radio on the receiver, check the request log shows it, then fill its list. Rollback: stop or uninstall the package and delete the rule; DSM and DNS keep working, and the receiver just shows no stations.

## Open Questions

- Which exact requests the RCD-N7 sends (confirmed from the request log on first run; the fallback keeps the menu working meanwhile).
