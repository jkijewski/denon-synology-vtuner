# Security policy

## Supported versions

Only the latest release gets fixes.

## Reporting a vulnerability

Please don't open a public issue. Use **Report a vulnerability** on the repository's **Security** tab, which starts a private conversation with the maintainers. Include the version, what you did and what happened, with real addresses and names replaced by example values.

You can expect a first answer within two weeks. This is a spare-time project, so a fix may take longer.

## Things to keep in mind

- vTuner Radio is made for a home network. The receiver side has no login, because receivers can't log in, so don't forward port 80 from the internet to these host names.
- The admin UI is only reachable through DSM's own login, and only for DSM administrators.
- The stream relay refuses to connect to the NAS itself, and for directory stations to any private address. A way around this is a vulnerability, so please report it.
