# How to test vTuner Radio without a Synology NAS

You can run and test vTuner Radio on any computer. Pick the level you need:

| Level | What you need | What it tests |
|---|---|---|
| [1. Run the app with Go](#1-run-the-app-with-go) | Go 1.27+ | Menus, receiver lists, directory, admin UI |
| [2. Docker test environment](#2-docker-test-environment) | Docker with Compose | The same, behind nginx on port 80, like on a NAS |
| [3. A real receiver](#3-test-with-a-real-receiver) | Level 2 and a receiver | What the receiver really sends and plays |
| [4. The package on Virtual DSM](#4-test-the-package-on-virtual-dsm) | Linux with KVM | Package install, DSM login check |

> All addresses and MAC addresses here are example values. Replace them with your own.

## The test receiver

`fakereceiver` sends the same requests a receiver sends, with the real `vtuner.com` URLs, to the server you name. It works in two modes:

- **Interactive:** shows a menu, you type a number to open a folder, search, or play a station (it downloads the first 64 KB of the stream).
- **Automatic** (`-auto`): logs in, opens every folder including sub-folders and virtual folders, searches if the server offers Search (the Docker image does), opens the genre list, browses one country and plays one station. It exits with an error if something fails, or if a menu holds a row with no name. Add `-no-directory` when the public directory is off or there's no internet.

Use `-mac` to act as a different receiver, for example `-mac 00:11:22:33:44:66`. Each receiver ID gets its own station list.

## 1. Run the app with Go

1. Start the server:

   ```sh
   make run
   ```

   Without `make`: `go run ./cmd/vtunerd serve -dev -data ./data`

2. Open the admin UI at <http://127.0.0.1:8580/admin/>. In this mode it needs no login.
3. In a second terminal, act as a receiver:

   ```sh
   go run ./cmd/fakereceiver -server 127.0.0.1:8580
   ```

4. Station lists are saved in `./data`. Delete that folder to start again.

## 2. Docker test environment

This runs the app with nginx in front of it. nginx does what DSM's Reverse Proxy rules do on a NAS: requests for `vtuner.com` names on port 80 go to the app, and other host names don't. For convenience nginx uses a wildcard here, while DSM needs a rule for each exact name.

1. Start it:

   ```sh
   docker compose up --build -d
   ```

2. Open the admin UI at <http://127.0.0.1:8581/admin/>. It's only reachable from this computer and needs no login.
3. Act as a receiver, from the container:

   ```sh
   docker compose exec vtuner fakereceiver -server 127.0.0.1:80
   ```

   Or from your computer, if you have Go: `go run ./cmd/fakereceiver -server 127.0.0.1:8000`

4. Stop it with `docker compose down`. Add `-v` to also delete the station lists.

### Settings

Set these as environment variables or in a `.env` file next to `compose.yaml`. The `.env` file is ignored by git.

| Variable | Default | Meaning |
|---|---|---|
| `VTUNER_HTTP_PORT` | `8000` | Port for receiver requests on your computer. A real receiver always uses port 80, so set `80` for [level 3](#3-test-with-a-real-receiver). |
| `VTUNER_ADMIN_PORT` | `8581` | Port for the admin UI, on `127.0.0.1` only. Change it if another program uses 8581. |
| `VTUNER_DIRECTORY` | `auto` | `auto` uses the public radio-browser.info directory, `off` works without internet |
| `VTUNER_RELAY_LAN` | `false` | `true` lets relayed stations you added play from a server on your network |

Example `.env`:

```sh
VTUNER_HTTP_PORT=8001
VTUNER_DIRECTORY=off
```

With another port, tell the test receiver: `fakereceiver -server 127.0.0.1:8001`.

### Quick checks with curl

```sh
# the token handshake
curl -H "Host: denon.vtuner.com" "http://127.0.0.1:8000/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0"

# the root menu, as receiver 00:11:22:33:44:55
curl -H "Host: denon.vtuner.com" "http://127.0.0.1:8000/setupapp/denon/asp/BrowseXML/loginXML.asp?mac=00:11:22:33:44:55"

# another host name gets the "not a vtuner.com host" answer, like DSM's own pages on a NAS
curl -H "Host: nas.example.lan" "http://127.0.0.1:8000/"
```

## 3. Test with a real receiver

The receiver must send its `vtuner.com` requests to the computer running the Docker test environment, for example `192.168.1.20`.

1. Start the Docker test environment on port 80, because the receiver always uses port 80: `VTUNER_HTTP_PORT=80 docker compose up --build -d`. Stop anything else on your computer that uses port 80 first (a local web server, for example).
2. Allow incoming connections on port 80 in the computer's firewall.
3. Make `denon.vtuner.com` and `*.vtuner.com` resolve to `192.168.1.20` for the receiver. Follow [How to redirect Denon vTuner to a Synology NAS](redirect-denon-vtuner-to-synology.md), but use `192.168.1.20` instead of the NAS address in the A records. Any DNS server on your network that can override names works the same way.
4. Open Internet Radio on the receiver.
5. In the admin UI, open **Receiver requests**. The receiver's requests should be listed, and it appears under **Receiver** at the top.
6. Please report what works with the **Receiver test report** issue template. Replace the MAC address with `00:11:22:33:44:55` before posting.

The admin UI shows the computer's Docker network address as the client for every request, not the receiver's address. That's expected in this setup.

## 4. Test the package on Virtual DSM

To test the Synology package itself (install, start and stop, the DSM main menu entry, the DSM login check), you need DSM. The community project [Virtual DSM](https://github.com/vdsm/virtual-dsm) runs DSM in a Docker container. It needs Linux with KVM.

1. Build the package: `make spk`.
2. Start Virtual DSM as its README describes, and finish the DSM setup in the browser.
3. Install `dist/vtunerradio-<version>-x86_64.spk` with **Package Center > Manual Install**.
4. Add the Reverse Proxy rule from [the install guide](install-synology-package.md#step-3-forward-port-80-to-the-package).
5. Point the test receiver at Virtual DSM's address: `fakereceiver -server <virtual DSM address>:80`.

Synology's licence allows DSM only on Synology hardware. Use Virtual DSM as a test tool and check the licence terms yourself.

## Unit tests

```sh
go test ./...
```

They run without network access: the public directory and the streams are replaced by local test servers.
