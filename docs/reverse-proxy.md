# How to set up the DSM reverse proxy for vTuner Radio

The receiver always connects to port 80, but vTuner Radio listens on its own local port (`8580` by default). Port 80 belongs to DSM's built-in web server, so you can't give it to the package directly. Instead, a reverse proxy rule tells DSM: "requests for `denon.vtuner.com` on port 80 go to vTuner Radio; everything else stays with DSM".

This tutorial creates that rule and checks that it works. It takes about five minutes.

> All addresses, MAC addresses and names here are example values. Replace them with the values from your own network.

| Example value | Meaning |
|---|---|
| `192.168.1.10` | The NAS |
| `192.168.1.50` | The receiver |
| `8580` | The local port vTuner Radio listens on (chosen during install) |

## Before you start

- vTuner Radio is installed and shows as **Running** in Package Center (see [the install guide](install-synology-package.md)).
- You know the local port you chose during install. If you aren't sure, open vTuner Radio from the DSM main menu and look at **Log**: a line near the top says `listening on 127.0.0.1:8580`.
- You are signed in to DSM with an account in the **administrators** group.

## Step 1: Open the reverse proxy settings

1. Open **Control Panel**.
2. Click **Login Portal**.
3. Open the **Advanced** tab.
4. Click **Reverse Proxy**.

A list of reverse proxy rules opens. It is often empty.

## Step 2: Create the rule

1. Click **Create**.
2. On the **General** tab, fill in:

   | Setting | Value | Why |
   |---|---|---|
   | Reverse proxy name | `vTuner` | Any name; it only shows in this list. |
   | **Source** Protocol | `HTTP` | The receiver doesn't use HTTPS. |
   | **Source** Hostname | `denon.vtuner.com` | The name a Denon receiver asks for. Use the exact name: DSM can refuse a wildcard such as `*.vtuner.com` as an invalid domain name. |
   | **Source** Port | `80` | The port the receiver connects to. |
   | Enable HSTS | off | HSTS is for HTTPS only, and the receiver can't follow it. |
   | Access control profile | Not configured | The receiver must always get in. |
   | **Destination** Protocol | `HTTP` | |
   | **Destination** Hostname | `localhost` | vTuner Radio only listens on the NAS itself. |
   | **Destination** Port | `8580` | Your local port from the install. |

3. Don't click **Save** yet. Continue with Step 3.

> **Keep this rule on your home network.** There's no login on this route, because receivers can't log in. If your router forwards port 80 to the NAS, anyone on the internet could use this rule by sending a `vtuner.com` host name, so limit that forward first.

## Step 3: Set the timeouts for radio streams

Most requests are short menu lists. Stations marked **Play through the NAS (relay)** are different: the stream runs through this rule for as long as you listen. The default timeouts are long enough for continuous audio, but raising them avoids cut-offs when a station pauses.

1. Open the **Advanced Settings** tab of the same dialog.
2. Set:

   | Setting | Value |
   |---|---|
   | Proxy connection timeout | `60` (keep the default) |
   | Proxy send timeout | `3600` |
   | Proxy read timeout | `3600` |
   | Proxy HTTP version | `HTTP 1.1` |

   If DSM doesn't accept `3600`, use the largest value it accepts.

3. Leave the **Custom Header** tab empty for now. Step 5 says when to use it.
4. Click **Save**.

The rule now appears in the list as `HTTP denon.vtuner.com:80 → HTTP localhost:8580`.

## Step 4: Check the rule from a computer

Run this on a computer in the same network (macOS and Linux have `curl`; on Windows use PowerShell's `curl.exe`):

```sh
curl -H "Host: denon.vtuner.com" "http://192.168.1.10/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0"
```

You should get:

```xml
<EncryptedToken>0000000000000000</EncryptedToken>
```

Then check that the rule didn't take anything else away:

1. Open `http://192.168.1.10/` in a browser. You should still get DSM's sign-in page or your own website, as before.
2. Open DSM as usual on its own port, for example `http://192.168.1.10:5000/`.

## Step 5: Check with the receiver

1. On the receiver, open **Internet Radio**.
2. In vTuner Radio, open **Receiver requests** and click **Refresh**.
3. You should see requests from the receiver's address, for example `192.168.1.50`.

If the requests show `127.0.0.1` instead of the receiver's address, DSM isn't passing the original address on. Then vTuner Radio can't tell receivers apart when a request has no MAC address. Fix it like this:

1. Edit the `vTuner` rule and open the **Custom Header** tab.
2. Click **Create** and choose **Custom header**.
3. Add two headers:

   | Header name | Value |
   |---|---|
   | `X-Real-IP` | `$remote_addr` |
   | `X-Forwarded-For` | `$proxy_add_x_forwarded_for` |

4. Click **Save**, open **Internet Radio** on the receiver again, and refresh **Receiver requests**.

## Add a rule for any other vTuner name

Each `vtuner.com` name the receiver uses needs its own rule, with the same settings and timeouts apart from the source hostname. One rule is enough if the receiver only uses `denon.vtuner.com`.

**Receiver requests** can't show you a missing name, because a request without a rule stops at DSM and never reaches vTuner Radio. So if the receiver reports an error and **Receiver requests** shows nothing new, look for the name elsewhere:

1. Check which names the receiver looks up, in your router's traffic view or in the DNS Server log if you have it turned on.
2. Test a name you find, replacing `other.vtuner.com` with it:

   ```sh
   curl -H "Host: other.vtuner.com" "http://192.168.1.10/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0"
   ```

   If you don't get `<EncryptedToken>`, that name has no working rule yet.
3. Add a rule for it, then open **Internet Radio** on the receiver again.

## Troubleshooting

| Symptom | What to check |
|---|---|
| `curl` shows a DSM page or "Sorry, the page you are looking for is not found" | The rule is saved, its source hostname matches the `Host` you used, and its source port is `80`. |
| `curl` shows "502 Bad Gateway" | vTuner Radio isn't running, or the destination port doesn't match the port you chose during install. Check Package Center and the **Log**. |
| `curl` gets no answer at all | The computer reaches the NAS (try `ping 192.168.1.10`), and no firewall rule on the NAS blocks port 80 (**Control Panel > Security > Firewall**). |
| DSM says the port is in use | Another rule already uses source `denon.vtuner.com:80`, or Web Station has a site on the same name and port. Remove the duplicate. |
| The receiver's requests don't show up at all | The receiver may be using a name with no rule (see [Add a rule for any other vTuner name](#add-a-rule-for-any-other-vtuner-name)). The receiver must also use the NAS (or your DNS server) to resolve `vtuner.com` to `192.168.1.10`. See [How to redirect Denon vTuner to a Synology NAS](redirect-denon-vtuner-to-synology.md). You can confirm the NAS answers with `nslookup denon.vtuner.com 192.168.1.10`, which should return `192.168.1.10`. |
| Relayed stations stop after a minute | Raise **Proxy send timeout** and **Proxy read timeout** (Step 3). |
| A relayed station starts late or stutters | Turn the relay off for that station if its URL also works over plain HTTP. |

## Remove the rule

When you uninstall vTuner Radio, delete the rule too: **Control Panel > Login Portal > Advanced > Reverse Proxy**, select `vTuner` and any other vTuner rules you added, and click **Delete**.
