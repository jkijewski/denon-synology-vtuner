# Design

## D1: One setting instead of trusting stations by origin

A station's origin can't be trusted: the catalog only knows `source`, and an imported file sets it. Marking imported stations as untrusted would need a new stored field (a data version and a migration), and a restore from an export would lose the trust of the user's own stations.

Relayed stations on the home network are rare: the relay exists for HTTPS streams on the internet. So the relay reaches only the internet by default, and `VTUNER_RELAY_LAN=true` in `settings.env` restores the 1.0.0 behaviour for stations from a receiver's list. The control script passes anything other than `true` as `false`, so a typo can't stop the package from starting.

## D2: The NAS's own addresses

DSM and other packages listen on the NAS's network addresses, not only on `127.0.0.1`. The relay reads the interface addresses (`net.InterfaceAddrs`) at connect time, cached for 30 seconds because DHCP can change them, and refuses them for every client. `0.0.0.0/8` is refused too, because a connection to `0.0.0.0` reaches the local host on Linux. The check runs in the dialer, like the existing ones, so redirects, playlist entries and DNS answers are covered.

## D3: Test URL

**Test URL** keeps a client that may reach the home network (without the NAS), because the receiver plays an unrelayed LAN station directly and the test should say that it works. Only DSM administrators can call it.

## D4: Relayed content type

Types starting with `audio/`, `application/ogg` and `application/octet-stream` pass through; anything else becomes `audio/mpeg`, which receivers already assume for a stream without a type. `X-Content-Type-Options: nosniff` and `Content-Security-Policy: sandbox; default-src 'none'` stop a browser from running anything in an answer.

## D5: Per-client relay limit

The limit is taken before the global slot, so a client at its limit can't hold a global slot. 2 per address allows the short overlap when a receiver switches stations. The address is `reqlog.ClientIP`, which D6 makes harder to fake.

## D6: Forwarded address

DSM's reverse proxy appends the peer address to `X-Forwarded-For` (`$proxy_add_x_forwarded_for`, as the reverse proxy guide sets up), so the last entry is the one the proxy saw; the earlier ones come from the client. `X-Real-IP` is used only when there is no `X-Forwarded-For`, because a proxy that doesn't set it passes the client's own value on.

## D7: Directory cache

The cache tracks its total size. Before storing, it drops expired answers, then the oldest ones until the new answer fits in 64 MB and 500 entries. An answer bigger than the whole budget is used once and not cached. One answer is read up to 8 MB; the largest real answer (2000 stations for a virtual folder of 1000) is about 3 MB.
