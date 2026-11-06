# How to redirect Denon vTuner to a Synology NAS

This guide makes a Denon receiver resolve `denon.vtuner.com` and other `vtuner.com` names to a Synology NAS on your network, so the Denon's vTuner requests reach the Synology instead of the internet.

> All addresses, MAC addresses and names in this guide are example values. Replace them with the values from your own network.

## Background

The Denon has to look up `vtuner.com` names on a DNS server that answers with the NAS. Most home routers can't do this for one device. The TP-Link Archer C6 (firmware 1.3.7), for example, has no DNS override or hosts file setting, and its **DHCP Server** page sets one DNS server for the whole network.

So the Synology does the DNS override, and only the Denon is pointed at it. The rest of the network keeps using its normal DNS, and the router needs no special settings. For other ways to do this, see [Alternatives](#alternatives).

## Example network

| Device | IP address | MAC address | Note |
|---|---|---|---|
| Router | `192.168.1.1` | `AA-BB-CC-00-00-01` | Gateway |
| Synology NAS (`synology-nas`) | `192.168.1.10` | `AA-BB-CC-00-00-02` | Needs an address that doesn't change |
| Denon receiver | `192.168.1.50` | `AA-BB-CC-00-00-03` | Gets a fixed address in Step 2 |

Reserve both addresses in your router's DHCP settings, or keep them outside the range it hands out, so that no other device gets them.

## Step 1: Set up DNS on the Synology

Menu names may differ slightly between DSM versions.

1. In DSM, open **Package Center** and install **DNS Server**. Check that the package shows as **Running**.
2. Open **DNS Server > Zones > Create > Master Zone**.
3. Set these zone values:

   | Setting | Value |
   |---|---|
   | Domain type | Forward Zone |
   | Domain name | `vtuner.com` |
   | Master DNS server | `192.168.1.10` |

4. In that zone, add an **A** record with the name `denon` and the IP `192.168.1.10`.
5. Add a second **A** record with the name `*` and the IP `192.168.1.10`, to cover other vTuner hosts. If DSM rejects `*`, add each host by name. If your router shows which names a device looks up, you can find them there.
6. Open **Resolution** and turn on **Enable forwarders**, with `8.8.8.8` as a forwarder, so that other names such as radio station streams still resolve.
7. If the Synology firewall is on, allow port **53** over UDP and TCP from the Denon (`192.168.1.50`).
8. Open **DNS Server > Zones** and check that the `vtuner.com` zone is enabled (ticked).

> **Note:** The Denon contacts `denon.vtuner.com:80`, and port 80 belongs to DSM's own web server. vTuner Radio sits behind a DSM reverse proxy rule, so set that up too: see [How to set up the DSM reverse proxy](reverse-proxy.md). If DSM rejects `*.vtuner.com` as the source hostname, use `denon.vtuner.com`.

## Step 2: Point the Denon at the Synology

Set this on the Denon, not on the router, so the rest of the network keeps its current DNS and doesn't depend on the Synology.

Finish Step 1 and pass the `nslookup` tests in [Step 3](#step-3-test) before you change the Denon. A Denon pointed at a NAS that doesn't answer DNS loses internet radio entirely.

A DHCP reservation on the router fixes the Denon's address but not its DNS, so the Denon needs the static settings below.

1. On the Denon, open **Setup > Network > Settings**. Use the front panel or on-screen menu, because the Denon's web page may not show the network settings. On the RCD-N7, it only offers the friendly name. The exact path depends on the model.
2. Turn **DHCP** off and enter these values:

   | Setting | Value |
   |---|---|
   | IP address | `192.168.1.50` |
   | Subnet mask | `255.255.255.0` |
   | Gateway | `192.168.1.1` |
   | Primary DNS | `192.168.1.10` |
   | Secondary DNS | Leave empty or `0.0.0.0` |

3. Save, then restart the Denon.

> **Warning:** Don't enter a public secondary DNS such as `8.8.8.8`. The Denon could use it and reach the real vTuner host.

## Step 3: Test

1. From a computer on your network, run:

   ```sh
   nslookup denon.vtuner.com 192.168.1.10
   ```

   The answer should be `192.168.1.10`.

2. Run:

   ```sh
   nslookup google.com 192.168.1.10
   ```

   It should return a public address, which shows that the forwarder works.

3. Run:

   ```sh
   curl -H "Host: denon.vtuner.com" "http://192.168.1.10/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0"
   ```

   The answer should be `<EncryptedToken>0000000000000000</EncryptedToken>`. A different, random token means you reached the real vTuner.

4. On the Denon, open the Internet Radio menu. Then open **vTuner Radio > Receiver requests** and click **Refresh**. Requests from `192.168.1.50` should appear.

   A station that keeps playing proves nothing, because saved stations can play without a vTuner lookup. Open the menu instead.

## Undo

Turn **DHCP** back on in the Denon's network settings.

## Alternatives

These let the Denon keep DHCP on, so you skip the static settings in Step 2. The Synology DNS setup in Step 1 stays the same.

| Option | When it works | Trade-off |
|---|---|---|
| DNS per device, from the router | Your router can hand one device its own DNS server. OpenWrt can, for example, but the Archer C6 can't. | Only for routers with this feature |
| DNS for the whole network, from the router | Your router can set the DNS server it hands out, as the Archer C6 can on its **DHCP Server** page. Set it to the NAS, with no secondary DNS. | Every device depends on the NAS: when the NAS is off or restarting, the whole network loses name lookups |

## Troubleshooting

| Symptom | Check |
|---|---|
| `nslookup` says `connection refused` or `no servers could be reached` | Nothing is listening on port 53: the DNS Server package isn't installed or is stopped. Check it in **Package Center**. A firewall block usually shows as a timeout instead. |
| `nslookup` times out | The DNS Server package is running, and port 53 is open in the Synology firewall. |
| `nslookup` returns a public IP for vTuner | You included the Synology's address in the command. Without it, `nslookup` asks your computer's normal DNS server, usually the router, which returns the public address. If you did include it, check that the zone is `vtuner.com`, that it is enabled, and that its A records are saved. |
| `nslookup` against the NAS returns the NAS, but **Receiver requests** stays empty | The Denon still uses the router's DNS. Check that its network screen shows DHCP off and Primary DNS `192.168.1.10`, with no secondary. |
| `curl` returns a token other than `0000000000000000` | You reached the real vTuner, not the NAS. Check the address in the command. |
| Other sites don't resolve | Forwarders are enabled under **Resolution**. |
| The Denon has no network | Re-check the IP address, subnet mask and gateway on the Denon. |
| The name resolves, but there is no radio | The vTuner replacement is running and listening on port 80 of the Synology (for example, `curl -v http://192.168.1.10/` from another computer gets a response). If DSM's web server holds port 80, move it or put the replacement behind it. Check the Synology firewall allows port 80 from `192.168.1.50`. |
