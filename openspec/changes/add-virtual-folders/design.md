# Design

## Context

A virtual folder is already a `Folder` with a `filter` (D10 in `add-vtuner-synology-package`), so ids, order, export, import and copying between receivers work unchanged. This change widens the filter and fixes rows with no name. The behaviour of the real receiver is only known from what it shows, so anything about its display is an inference until checked on the hardware (task 6.3).

## Goals / Non-Goals

**Goals:**
- A receiver never sees a row without a name, whatever the catalog or the directory holds.
- The preview shows exactly what the receiver will list.
- Lists saved by every earlier version keep loading.

**Non-Goals:**
- Storing matching stations, or combining filters with "or".
- Showing error text on the receiver for virtual folders.

## Decisions

### D1. Why rows had no name
The RCD-N7 shows a row with no name that lists nothing. Every `Dir` item has a `Title` and the catalog refuses empty names, but the top menu of packages up to 0003 always ended with a `Search` item, which uses `SearchCaption` and has no `Title`. The package code confirms this: 0003 has neither the `-search` option nor a way to turn the item off. The source has left it out by default since D-12, and the first package carrying that is 0004.

### D2. Defence in layers for blank names
- **Source:** the directory client drops nameless countries, genres, languages and stations, so counts stay right. The catalog refuses them.
- **Definition of blank:** `catalog.Blank` also counts invisible characters such as a zero-width space, because `strings.TrimSpace` keeps those and a display shows nothing.
- **Last step:** `named()` filters every answer before paging, so `ItemCount` stays correct and a future item source can't bring the problem back.
- **Items of other types:** `Search` and `Display` items now also carry a `Title`. Unknown tags are safe for other receivers, and the RCD-N7 appears to read `Title` for its rows.
- *Alternative:* removing `Display` items altogether. The empty-catalog message and the browse error are still useful on receivers that show them, so they stay, with a `Title`.

### D3. New filter fields are optional and additive
`language`, `minBitrate`, `order` and `limit` are `omitempty` fields of `Filter`, with zero meaning "not set". `Parse` refuses unknown fields, so a newer file doesn't load in an older package, but older files load here unchanged. There is no format version bump for these fields, because nothing needs migrating. `genres` (D8) does need one, as does every later structure change.

### D4. Mapping to radio-browser
`directory.FromFilter` is the only mapping, used by the receiver handler and the preview, so they can't disagree.

| Filter | `/json/stations/search` |
|---|---|
| `name` | `name` |
| `country` | `countrycode` |
| `genres` | one search per genre: `tag`, with `tagExact=true` (D8) |
| `language` | `language` (lower case, partial match, so `polish` matches `polish,english`) |
| `format` | `codec` |
| `minBitrate` | `bitrateMin` |
| `order` | `order`; default `clickcount` |
| (derived) | `reverse=false` for `name`, otherwise `true` |
| `limit` | `limit` |
| (always) | `hidebroken=true` |

### D5. Ask for twice the limit, then cut
HLS and unplayable codecs can only be removed after the answer arrives. Asking for exactly `limit` would make "at most 20" often return fewer than the directory has. `Stations` asks for twice as many and stops at the limit. This also applies to the browse menus, which fill up better for it. The effective limit is the smaller of the folder's `limit` and `-directory-limit`.

### D6. Empty list when the directory can't answer
The folder returns zero items and a note in the request log. An informational row risks being another row with no name on the RCD-N7. The menu holding the folder still appears, with a count of 0, under the existing 2-second limit. The preview, in contrast, reports the failure, because the person using it can act on it.

### D7. Preview through its own endpoint
`POST /admin/api/directory/preview` takes a filter as JSON, validates it with the same `Filter.Normalize` that saving uses, and answers with stations. A `POST` with a JSON body keeps the request behind the existing check on the content type, and there's no second list of parameters to keep in step with the filter. It uses the normal directory cache.

### D8. Several genres mean any of them, with one search per genre
The stored field becomes `genres`, a list of up to 10. Radio-browser's `tagList` finds only stations that have all the listed tags, which would make "80s and 90s" close to empty, so `Stations` runs one search per genre and merges them.

- **Order and limit.** Each search returns the best `limit` of its genre, so the merged list holds the best `limit` overall. The merge sorts by the folder's order, using `clickcount`, `votes`, `clicktrend` and `bitrate` from the answers, with ties going by name and id, so that pages stay in step between requests.
- **Duplicates.** A station in two genres is listed once, by its directory id.
- **Speed.** The searches run together, because the menu holding the folder waits only 2 seconds. Each search is cached on its own.
- **Failure.** If one search fails, the whole listing fails and D6 applies. A part of the list shown as the whole would be misleading, and failures aren't cached, so the next request tries again.
- **One genre** takes the same path as before, with the directory's order and no merge.
- **Data version 2.** A catalog step moves `genre` into a one-item `genres` at any depth and removes a blank one. It also runs on imported files. The public directory page keeps its single genre select, and "Save as virtual folder" starts the form with that genre.

## Risks / Trade-offs

- [The blank row has another cause too] → the fix covers every kind of item and the fake receiver now fails on blank names, but only the RCD-N7 can confirm it (task 6.3).
- [Many virtual folders in one menu] → counts share one 2-second limit and are cached for 30 minutes, so a cold start can show 0 for some. Opening the folder still works.
- [Twice as much asked of the public API] → answers are cached, and a listing is at most 400 stations by default.
- [An invisible-only name in an existing file] → that list now refuses to load, as a whitespace-only name already does. The receiver registry's handling of damaged lists keeps the service up. This is unlikely, since the UI never produced such names on purpose.

## Migration Plan

Install the new package over the old one. Lists load unchanged and nothing is rewritten until the next save. To go back to an older package, delete virtual folders that use the new fields first, because older versions refuse unknown fields.
