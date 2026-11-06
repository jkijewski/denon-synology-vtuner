# Proposal

## Why

Two things came up on the real Denon RCD-N7:

- **A folder with no name.** The menu shows a row with an empty name that opens to no stations. Packages up to 0.1.0-0003 always send a `Search` item in the top menu. That item has no `Title`, and it is the only top-menu item without one. The source already leaves it out by default (D-12 in `add-vtuner-synology-package`), but no package with that change has been built yet. Other rows could go blank the same way: directory genres and stations have no name check, and the catalog accepts names made only of invisible characters.
- **Virtual folders are too narrow.** The first version (D-11) filters on country, genre, format and name, always sorts by popularity and always lists as many as it can. Users want folders like "the 20 highest-voted Polish-language stations of at least 128 kbit/s", and want to see the result before saving.

## What Changes

- No receiver menu contains an item with an empty or blank name. Nameless directory countries, genres, languages and stations are dropped, the catalog refuses names with only spaces or invisible characters, the menu code drops anything that still gets through, and `Search` and informational items carry a `Title`.
- Virtual folders gain filters for language and minimum bitrate, a sort order and a maximum number of stations. Any single filter is enough to define one.
- A virtual folder can be created at the top level or inside a plain folder.
- The admin UI's virtual folder form gets the new fields and a **Preview** button that shows matching stations before saving. Virtual folders get a badge in the folder list.
- **BREAKING (receiver behaviour only):** when the directory is off or not reachable, or nothing matches, a virtual folder shows an empty list instead of an informational row. The reason goes to the request log.
- A virtual folder can hold several genres, and a station needs only one of them, so one list can combine 80s and 90s. All other filters still have to match together.
- Not in scope: filters over the receiver's own stations, "or" between different filters or for fields other than genre, saving a copy of the matching stations.

### Product decisions

Answered by the user on 2026-11-06 unless marked as an assumption.

- **V-1 Filters.** Name contains, country code, genre, language, codec, minimum bitrate, sort order and maximum results. Any one of them defines a folder.
- **V-2 Unavailable directory.** The folder shows an empty list and never fails.
- **V-3 Stored with the list.** Virtual folders stay in the receiver's catalog file, and files from older versions must still load.
- **V-4 Several genres.** The genre filter is a multi-select, for example a list of 80s and 90s (requested on 2026-11-06).
- *Assumption:* the sort orders offered are most played (the default), most votes, trending, highest bitrate and name. Random is left out, because a list that changes between requests breaks paging.
- *Assumption:* several genres mean "any of them", because a station tagged with both 80s and 90s is rare and the request is for one list combining both.
- *Assumption:* a folder holds at most 10 genres, because each one is a directory request.
- *Assumption:* a filter with only a sort order or a maximum is allowed, and lists the top stations worldwide up to the listing limit.

## Capabilities

### Modified Capabilities

- `vtuner-protocol`: every menu item has a name; virtual folder listing follows the new filters and shows an empty list when it can't answer.
- `public-directory`: station queries take the new filters; nameless entries are dropped; a language list is available.
- `station-catalog`: new optional filter fields, any one filter is enough, blank names refused, older files load.
- `station-admin-ui`: form fields, placement, Preview, badge.

## Impact

- Code: `internal/catalog`, `internal/directory`, `internal/vtuner`, `internal/admin` and its UI, `cmd/fakereceiver`, `cmd/vtunerd`.
- Data: data version 2 replaces the filter's `genre` with a `genres` list, and a migration converts existing lists and imported files. Older packages refuse data version 2, so going back means restoring the copy in the `backups` folder.
- Depends on `add-vtuner-synology-package`, which isn't archived yet and adds the requirements this change modifies. Archive that change first.
