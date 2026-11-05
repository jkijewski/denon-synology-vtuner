# How to redirect Denon vTuner to a Synology NAS

This guide makes a Denon receiver resolve `denon.vtuner.com` and other `vtuner.com` names to a Synology NAS on your network, so the Denon's vTuner requests reach the Synology instead of the internet.

> All addresses, MAC addresses and names in this guide are example values. Replace them with the values from your own network.

## Background

The TP-Link Archer C6 (firmware 1.3.7) has no DNS override or hosts file setting. Its only DNS control is the **DHCP Server** page, which sets one DNS server for the whole network.

So the Synology does the DNS override, and only the Denon is pointed at it. The rest of the network keeps using its normal DNS.

## Example network

| Device | IP address | MAC address | Note |
|---|---|---|---|
| Router, TP-Link Archer C6 | `192.168.1.1` | `AA-BB-CC-00-00-01` | Gateway |
| Synology NAS (`synology-nas`) | `192.168.1.10` | `AA-BB-CC-00-00-02` | Reserved in the router's DHCP |
| Denon receiver | `192.168.1.50` | `AA-BB-CC-00-00-03` | Reserved in the router's DHCP |

## Step 1: Set up DNS on the Synology

Menu names may differ slightly between DSM versions.

1. In DSM, open **Package Center** and install **DNS Server**.
2. Open **DNS Server > Zones > Create > Master Zone**.
3. Set these zone values:

   | Setting | Value |
   |---|---|
   | Domain type | Forward Zone |
   | Domain name | `vtuner.com` |
   | Master DNS server | `192.168.1.10` |

4. In that zone, add an **A** record with the name `denon` and the IP `192.168.1.10`.
5. Add a second **A** record with the name `*` and the IP `192.168.1.10`, to cover other vTuner hosts. If DSM rejects `*`, add each host by name as it appears in the router's **Insights** history (see Step 3).
6. Open **Resolution** and turn on **Enable forwarders**, with `8.8.8.8` as a forwarder, so that other names such as radio station streams still resolve.
7. If the Synology firewall is on, allow port **53** over UDP and TCP from the Denon (`192.168.1.50`).

> **Note:** Your vTuner replacement must also answer on the Synology's **port 80**, because the Denon contacts `denon.vtuner.com:80`. DSM's own web server may already use that port.

## Step 2: Point the Denon at the Synology

Set this on the Denon, not on the router, so the rest of the network keeps its current DNS and doesn't depend on the Synology.

1. On the Denon, open **Setup > Network > Settings**. The exact path depends on the model.
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

3. Start internet radio on the Denon and check the web server log on the Synology for a request from `192.168.1.50`.
4. Optional: if you created a Parental Controls profile for the Denon to watch its traffic, open **Advanced > Parental Controls > *your profile* > Insights** in the router. `denon.vtuner.com` should get no new entries, because that traffic now stays on your network. Station streams should still appear.

## Undo

- Turn **DHCP** back on in the Denon's network settings.
- If you created a Parental Controls profile for tracking, delete it in the router.

## Troubleshooting

| Symptom | Check |
|---|---|
| `nslookup` times out | The DNS Server package is running, and port 53 is open in the Synology firewall. |
| `nslookup` returns a public IP for vTuner | The zone is `vtuner.com` and its A records are saved. |
| Other sites don't resolve | Forwarders are enabled under **Resolution**. |
| The Denon has no network | Re-check the IP address, subnet mask and gateway on the Denon. |
| The name resolves, but there is no radio | The vTuner replacement is running and listening on port 80 of the Synology (for example, `curl -v http://192.168.1.10/` from another computer gets a response). If DSM's web server holds port 80, move it or put the replacement behind it. Check the Synology firewall allows port 80 from `192.168.1.50`. |
