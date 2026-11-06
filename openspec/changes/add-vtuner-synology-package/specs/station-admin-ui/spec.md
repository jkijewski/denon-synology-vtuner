# Spec Delta

## Purpose

Gives the NAS owner a browser page, opened from DSM, to manage each receiver's station list, browse the public directory and check what the receivers are doing.

## ADDED Requirements

### Requirement: Opened from DSM
The admin UI SHALL be reachable from an icon in the DSM main menu and SHALL open in a browser on the LAN.

#### Scenario: Open from DSM
- **WHEN** an administrator clicks the package icon in the DSM main menu
- **THEN** the admin UI opens and shows the catalog

### Requirement: Access limited to DSM administrators
The admin UI and its API SHALL only be usable by a DSM user who is logged in and is a member of the `administrators` group.

#### Scenario: Not logged in
- **WHEN** someone opens the admin UI URL without a DSM session
- **THEN** the UI refuses access and points to the DSM login

#### Scenario: Non-admin user
- **WHEN** a logged-in DSM user who is not an administrator opens the admin UI
- **THEN** access is refused

### Requirement: Manage folders and stations
The admin UI SHALL let the user create, rename, delete and reorder folders, and add, edit, delete, copy into another folder and reorder stations.

#### Scenario: Reorder stations
- **WHEN** the user moves "Example Smooth" above "Example Jazz FM" in folder "Jazz" and saves
- **THEN** the receiver lists "Example Smooth" first

#### Scenario: Delete a folder that is not empty
- **WHEN** the user deletes a folder that holds stations
- **THEN** the UI asks for confirmation before deleting

### Requirement: Choose the receiver
The admin UI SHALL show a receiver selector, and every list change SHALL apply only to the selected receiver.

#### Scenario: Edit one receiver
- **WHEN** the user selects "Living room" and adds a station
- **THEN** only the Living room receiver's list changes

### Requirement: Browse the public directory
The admin UI SHALL let the user filter the public directory by country, genre and name. Clicking Add on a result SHALL show a list of the selected receiver's folders, including sub-folders, and the station SHALL be saved in the folder the user picks.

#### Scenario: Add from the directory
- **GIVEN** the selected receiver has folders "Jazz" and "Poland"
- **WHEN** the user searches the directory, clicks Add next to a station, and picks "Jazz" in the folder list that appears
- **THEN** the station is saved in the receiver's "Jazz" folder, with the relay on if its URL uses HTTPS

#### Scenario: Cancel adding
- **WHEN** the user clicks Add and then Cancel without picking a folder
- **THEN** nothing is saved and the Add button shows again

#### Scenario: Station already in a folder
- **GIVEN** the station is already in "Jazz"
- **WHEN** the user clicks Add next to it
- **THEN** "Jazz" is shown as already holding the station and can't be picked, and the other folders can

#### Scenario: Receiver has no folders
- **GIVEN** the selected receiver has no folders
- **WHEN** the user clicks Add
- **THEN** the UI asks the user to create a folder under My stations first

### Requirement: Filter directory results by format
The public directory filter in the admin UI SHALL include a Format choice with "Any" and each stream format the receiver can play (MP3, AAC, AAC+). Choosing a format SHALL list only results in that format, combined with the country, genre and name filters.

#### Scenario: MP3 only
- **WHEN** the user picks country "Poland" and format "MP3" and searches
- **THEN** only Polish stations streaming in MP3 are listed

#### Scenario: Any format
- **WHEN** the format is "Any"
- **THEN** results in all playable formats are listed

### Requirement: Show where directory stations already are
Each public directory result that is already in the selected receiver's list SHALL show one label per folder holding it, with the folder's path, for example "By country › Poland". A result matches a station in the list when it has the same directory id or the same stream URL. The labels SHALL update right after the station is added to another folder or the receiver is changed.

#### Scenario: Station in two folders
- **GIVEN** "Example Jazz Radio" is in "Jazz" and in "By country › Poland" of the selected receiver
- **WHEN** the user searches the directory and the station is listed
- **THEN** its row shows the labels "Jazz" and "By country › Poland"

#### Scenario: Station not in the list
- **WHEN** a listed station is in none of the selected receiver's folders
- **THEN** its row shows no labels

#### Scenario: Labels follow the receiver
- **GIVEN** the station is in a folder of "Living room" but not of "Kitchen"
- **WHEN** the user switches the receiver from "Living room" to "Kitchen"
- **THEN** the station's labels disappear

### Requirement: Add a station by URL
The admin UI SHALL let the user add a station that isn't in the directory by entering its name and stream URL.

#### Scenario: Own station
- **WHEN** the user adds "My Example Station" with URL `http://stream.example.com/live.mp3`
- **THEN** it appears in the selected folder

### Requirement: Test a station URL
The admin UI SHALL let the user check a station's stream URL and SHALL show whether it is reachable, its content type, and whether the relay is needed.

#### Scenario: HTTPS-only stream
- **WHEN** the user tests `https://stream.example.com/live.aac`
- **THEN** the UI reports that the stream is reachable over HTTPS only and suggests turning on the relay

### Requirement: Show recent receiver requests
The admin UI SHALL show the request log, newest first, so the user can confirm the receiver is reaching the NAS.

#### Scenario: Receiver connected
- **WHEN** the receiver at `192.168.1.50` opens Internet Radio and its MAC address is in the login request
- **THEN** the request appears in the admin UI's request list with that client IP

### Requirement: Import and export from the UI
The admin UI SHALL offer catalog export as a download and import from an uploaded file, with a choice of replace or merge.

#### Scenario: Export
- **WHEN** the user clicks Export
- **THEN** the browser downloads the catalog file

### Requirement: A save can't overwrite newer changes
The admin UI SHALL save a list only if it hasn't changed since the page loaded it, and otherwise SHALL say so and leave the stored list as it is.

#### Scenario: Two windows
- **GIVEN** the same list is open in two windows
- **WHEN** both save, one after the other
- **THEN** the second save is refused with a message, and the first window's changes stay

### Requirement: Requests from other sites are refused
The admin UI SHALL refuse requests that change something when they come from another web site, even if the browser sends a valid DSM login with them. The admin API SHALL accept request bodies only as `application/json`.

#### Scenario: Form on another site
- **GIVEN** a DSM administrator is signed in
- **WHEN** a page on another site posts to the admin API
- **THEN** the request is refused and nothing changes

### Requirement: Adding from the directory keeps other edits unsaved
Adding a directory station SHALL save it immediately only when the list has no other unsaved changes. Otherwise the station SHALL join the unsaved changes, for the user to save or discard together.

#### Scenario: Unsaved edits open
- **GIVEN** the user has renamed a folder and not saved
- **WHEN** the user adds a directory station to a folder
- **THEN** nothing is saved yet, and the UI says the station needs saving

### Requirement: Import mode is explicit
The import API SHALL accept only the modes `merge` and `replace`, and SHALL refuse any other value without changing the list.

#### Scenario: Unknown mode
- **WHEN** an import is sent with an unknown mode
- **THEN** it fails with an error and the list is unchanged

### Requirement: Manage virtual folders
The admin UI SHALL let the user create a virtual folder with a name and filters for country, genre, format and station name, from the folder list or from the current filters on the public directory page, and edit, rename, reorder and delete it. Selecting a virtual folder SHALL show its filters and a preview of the stations that match now. The UI SHALL NOT offer a virtual folder as a place to add or copy stations.

#### Scenario: Save directory filters as a folder
- **GIVEN** the user has chosen country Poland and genre rock on the public directory page
- **WHEN** the user clicks Save as virtual folder and gives it a name
- **THEN** the folder is saved in the selected receiver's list with those filters

#### Scenario: Preview
- **WHEN** the user selects a virtual folder
- **THEN** the page shows its filters, the number of matching stations and the first ones

#### Scenario: Adding a station from the directory
- **GIVEN** the selected receiver has folder "Jazz" and virtual folder "Polish rock"
- **WHEN** the user clicks Add next to a directory station
- **THEN** the folder list offers "Jazz" but not "Polish rock"
