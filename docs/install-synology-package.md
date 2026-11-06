# How to install the vTuner Radio package on a Synology NAS

This guide installs **vTuner Radio**, the self-hosted vTuner replacement, as a Synology package. It then connects it to port 80 so that your receiver can reach it.

Do the DNS steps in [How to redirect Denon vTuner to a Synology NAS](redirect-denon-vtuner-to-synology.md) first, so that the receiver sends its `vtuner.com` requests to the NAS.

> All addresses, MAC addresses and names in this guide are example values. Replace them with the values from your own network.

## What you need

- A Synology NAS with DSM 7.0 or later. This guide was written for a DS220+ (x86_64) on DSM 7.4.
- The package for your NAS from the project's **Releases** page. To build it yourself instead, you need a computer with [Go](https://go.dev/dl/) 1.27 or later and `make`. Linux and macOS work.
- A DSM account in the **administrators** group.

| Example value | Meaning |
|---|---|
| `192.168.1.10` | The NAS |
| `192.168.1.50` | The receiver |
| `00:11:22:33:44:55` | The receiver's MAC address |
| `8580` | The local port vTuner Radio listens on |

## Step 1: Get the package

Download the `.spk` file for your NAS from the **Releases** page: `x86_64` for Intel and AMD models, `armv8` for 64-bit ARM models. To find your NAS's CPU type, look up its model on Synology's website. Then go to Step 2.

To build it yourself instead:

1. In the repository folder, run:

   ```sh
   make spk
   ```

   This runs the tests, builds the program for x86_64, and checks the package for personal data.

2. The package is in `dist/`, for example `dist/vtunerradio-1.0.0-0001-x86_64.spk`.

For an ARM NAS, run `make spk ARCHES="armv8"` instead. To find your NAS's CPU type, look up its model on Synology's website.

## Step 2: Install the package

1. In DSM, open **Package Center** and click **Manual Install**.
2. Upload the `.spk` file and click **Next**.
3. DSM warns that the package is from an unknown publisher, because it isn't signed. Confirm to continue.
4. Keep the local port `8580`, or enter another free port. The installer stops if the port is in use.
5. Finish the wizard. The package should show as **Running**.

## Step 3: Forward port 80 to the package

DSM's own web server always uses port 80. A reverse proxy rule tells it to pass `denon.vtuner.com` to vTuner Radio and leave every other name alone.

The short version is below. For a step-by-step walk-through with timeouts, checks and fixes, see [How to set up the DSM reverse proxy](reverse-proxy.md).

1. Open **Control Panel > Login Portal > Advanced > Reverse Proxy** and click **Create**.
2. Enter these values:

   | Setting | Value |
   |---|---|
   | Reverse proxy name | `vTuner` |
   | Source protocol | HTTP |
   | Source hostname | `denon.vtuner.com` |
   | Source port | `80` |
   | Destination protocol | HTTP |
   | Destination hostname | `localhost` |
   | Destination port | `8580` |

3. Click **Save**.
4. Use the exact name, because DSM can refuse a wildcard such as `*.vtuner.com` as an invalid domain name. If your receiver also uses other `vtuner.com` names, each needs its own rule. **Receiver requests** can't show those names, so see [Add a rule for any other vTuner name](reverse-proxy.md#add-a-rule-for-any-other-vtuner-name).

> **Keep this on your home network.** vTuner Radio has no login on this route, because receivers can't log in. Don't forward port 80 from your router to the NAS for these host names. If port 80 is already forwarded for something else, such as certificate renewal, anyone on the internet could use the rule by sending a `vtuner.com` host name. Limit the forward, or block those names, before adding the rule.

## Step 4: Test from a computer

Run:

```sh
curl -H "Host: denon.vtuner.com" "http://192.168.1.10/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0"
```

The answer should be `<EncryptedToken>0000000000000000</EncryptedToken>`. Then open `http://192.168.1.10/` in a browser and check that DSM still works as before.

## Step 5: First run with the receiver

1. In DSM, open the main menu and click **vTuner Radio**.
2. On the receiver, open **Internet Radio**.
3. In vTuner Radio, open **Receiver requests** and click **Refresh**. You should see requests from `192.168.1.50`.
4. Open the **Receiver** list at the top. The receiver appears with a name like `Receiver 334455`, from the end of its MAC address. Click **Rename** to give it a better name.

Each receiver has its own station list. A new receiver starts with a copy of the **Default** list.

## Step 6: Add stations

- **From the public directory:** open **Public directory**, pick a country, a genre or type a name, and click **Search**. Click **Add** next to a station and choose the folder in the list that appears. It's saved right away. Click **Cancel** to close the list without adding.
- **Your own station:** open **My stations**, select a folder and click **Add station**. Enter the name and the stream URL, then click **Test URL**. If the test says the stream is HTTPS only, keep **Play through the NAS (relay)** on. Click **OK**, then **Save changes**.
- **Virtual folders:** a folder that lists public directory stations matching filters, so you don't add them one by one. Open **My stations** and click **Add virtual folder**, or set the filters on **Public directory** and click **Save as virtual folder**.
  - Set at least one of country, genres, language, format, minimum bitrate or station name, and optionally a sort order and a maximum number of stations. A station must match all the filters you set.
  - You can tick up to 10 genres, and a station needs only one of them, so ticking `80s` and `90s` gives one list with both. A station in both genres is listed once.
  - Under **Place in**, choose the top level or a folder. Use the arrows to change its position later.
  - Click **Preview** to see the matching stations before you save.
  - Virtual folders have a **virtual** badge in the folder list. On the receiver they look like any other folder, and their stations play like other directory stations.
  - The list follows the directory, so it can change over time. If the directory can't be reached, the folder opens empty; the reason is under **Receiver requests**.
  - A virtual folder can't hold your own stations or sub-folders. Export and import include virtual folders. An older package version can't load a list that has them.
- **Folders:** use **Add folder** and **Add sub-folder**, for example `Jazz`, `News`, `Poland`. A station can be in several folders: use **Copy to…** next to it.

On the receiver, the top menu shows your folders, then **Browse by country** and **Browse by genre**.

## Back up your lists

Open **Import / export** and click **Download catalog**. Do this for each receiver. To restore a list, choose the file under **Import** and click **Import**.

Updating keeps all lists, see below. When you uninstall, the wizard asks whether to keep the lists on the NAS or delete them, so export first if you choose to delete.

## Update the package

You don't need to export or import anything to update, and you don't need to uninstall first.

1. In **Package Center**, click **Manual Install** and upload the newer `.spk`. Its build number must be higher than the installed one.
2. Finish the wizard, then check that the package shows as **Running**.

Station lists, receivers and settings are kept, because they live in `/var/packages/vtunerradio/var`, outside the program files that an update replaces. Export is only for a backup, or before you uninstall or go back to an older version.

The stored data has a version number, in `schema-version` in that folder and in each station list:

- **Older data:** if a new package changes the data format, the app converts your data the first time it starts. It first saves a copy in `backups/` in that folder, and keeps the last 3 copies.
- **Newer data:** a package that's too old for your data refuses to install, and nothing changes. Only packages from build 0005 on make this check. If the data is newer anyway, for example after a reinstall, the app won't start, and the log says why.
- **If the conversion fails:** the app stays stopped and the log says which file caused it and where the copy is.

The same check applies to **Import**: an older export is converted, and an export from a newer version is refused.

## Settings

The package registers its local port with DSM's firewall as **vTuner Radio**, so it appears in **Control Panel > Security > Firewall** when you create a rule from a list of built-in applications. The port only listens on the NAS itself, so you don't need a firewall rule for it. Receivers reach the package through the reverse proxy on port 80. If you change the port in `settings.env`, the firewall entry follows at the next package update.

The settings file is `/var/packages/vtunerradio/var/settings.env` on the NAS. Restart the package in Package Center after a change.

| Setting | Default | Meaning |
|---|---|---|
| `VTUNER_LISTEN` | `127.0.0.1:8580` | Local address and port |
| `VTUNER_HOST` | `denon.vtuner.com` | Host name used in links when a request has none |
| `VTUNER_DIRECTORY` | `auto` | `auto` uses the public radio-browser.info servers, `off` hides directory browsing |

## Troubleshooting

| Symptom | Check |
|---|---|
| Reverse proxy problems | See the troubleshooting table in [How to set up the DSM reverse proxy](reverse-proxy.md). |
| The `curl` test shows a DSM page | The reverse proxy rule is saved, and its source hostname matches the `Host` you used. |
| The `curl` test gets no answer | The package is running in Package Center, and the destination port in the rule matches the port you chose. |
| No requests in **Receiver requests** | The NAS answers DNS and the receiver uses the NAS as its DNS server (see [Test](redirect-denon-vtuner-to-synology.md#step-3-test) and [Troubleshooting](redirect-denon-vtuner-to-synology.md#troubleshooting) in the DNS guide). |
| A request has the note "unknown request" | The receiver asked for something vTuner Radio doesn't know yet. The menu still works. Please report the path and query in an issue. |
| **Public directory not reachable** | The NAS can reach the internet, and `VTUNER_DIRECTORY` isn't `off`. |
| A station doesn't play | Click **Edit** and **Test URL**. Turn on the relay for HTTPS streams. Pick an MP3 stream if the receiver can't play the format. |
| Opening vTuner Radio says it "couldn't confirm your DSM login" | You're signed in, but the package's login check failed. Open **Details** on that page and include them when you report the problem. They contain no passwords, cookies or tokens. |
| Opening vTuner Radio says "Please sign in" or "Only DSM administrators" | Sign in to DSM with an account in the **administrators** group, then open it from the DSM main menu. |
| The package doesn't start | Open **Log** in vTuner Radio, or read `/var/packages/vtunerradio/var/vtunerd.log` and `vtunerd.stderr.log`. |

## Uninstall

1. Export the lists you want to keep elsewhere.
2. Uninstall **vTuner Radio** in Package Center. The wizard asks what to do with the station lists and settings:
   - **Keep** (the default) leaves them in `/var/packages/vtunerradio/var`, and a later install of vTuner Radio uses them again.
   - **Delete** removes them.
3. Delete the `vTuner` rule in **Control Panel > Login Portal > Advanced > Reverse Proxy**.

DSM also removes the **vTuner Radio** entry that the package added to the firewall's list of applications.
