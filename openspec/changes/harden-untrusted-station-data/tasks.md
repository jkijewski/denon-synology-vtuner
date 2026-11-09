# Tasks

## 1. Relay

- [x] 1.1 Make stations from a receiver's list internet-only unless `-relay-lan` / `VTUNER_RELAY_LAN=true`, keep the LAN for Test URL, and verify with `go test ./internal/relay`
- [x] 1.2 Refuse the NAS's own interface addresses and `0.0.0.0/8` for every client, and verify with `go test ./internal/relay`
- [x] 1.3 Send relayed answers as an audio type with `nosniff` and a sandbox policy, and verify with an upstream answering `text/html`
- [x] 1.4 Limit relayed streams per client address to 2, and verify that a second client still gets a stream
- [x] 1.5 Log a hint about `VTUNER_RELAY_LAN` when a station from a list is refused for a home network address

## 2. Directory and client address

- [x] 2.1 Limit the directory cache to 64 MB and one answer to 8 MB, and verify eviction with `go test ./internal/directory`
- [x] 2.2 Take the last `X-Forwarded-For` entry before `X-Real-IP`, and verify made-up headers with `go test ./internal/reqlog`

## 3. Package and docs

- [x] 3.1 Add `VTUNER_RELAY_LAN=false` to new settings files and pass it from `start-stop-status`, treating anything but `true` as `false`
- [x] 3.2 Describe the setting in the install guide, the test guide, `compose.yaml` and the changelog
- [ ] 3.3 Build the package, install it over 1.0.0 on the NAS, and verify that a directory station and an HTTPS station of your own still play through the relay
- [ ] 3.4 On the NAS, verify that a relayed station pointing at the NAS's own address fails with "not allowed" in the log
- [ ] 3.5 On the NAS, verify that **Receiver requests** still shows the receiver's address, not the proxy's
