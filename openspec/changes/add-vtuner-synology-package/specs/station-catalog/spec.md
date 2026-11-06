# Spec Delta

## Purpose

Stores one receiver's radio list (folders and stations) on the NAS, so it can be changed at any time without rebuilding or reinstalling the package.

## ADDED Requirements

### Requirement: Station fields
Each station SHALL have a stable numeric id, a name and a stream URL, and SHALL support optional fields: a description, a format (for example MP3 or AAC), a bitrate, a country or location, a logo URL, and a relay setting.

#### Scenario: Minimal station
- **WHEN** a station is saved with name "Example News" and URL `http://stream.example.com/news.mp3`
- **THEN** it is stored with a new id and default values for the optional fields

### Requirement: Source of a station
A station added from the public directory SHALL keep the directory's station id, so it can be recognised and not added twice to the same list.

#### Scenario: Added twice
- **GIVEN** a directory station is already in the list
- **WHEN** the user adds it again from the directory to another folder
- **THEN** the same station is put in that folder and no duplicate is created

### Requirement: Folders with nesting and order
The catalog SHALL organise stations in folders, allow folders inside folders, allow one station to appear in more than one folder, and keep a user-defined order within each folder.

#### Scenario: Favourites by genre and by country
- **GIVEN** 50 stations in folders "Jazz", "News", "Poland" and "Germany"
- **WHEN** the receiver opens any of these folders
- **THEN** each lists its stations in the saved order

#### Scenario: Station in two folders
- **GIVEN** station "Example Jazz FM" is in "Jazz" and in "Favourites"
- **WHEN** the receiver opens either folder
- **THEN** the station is listed in both, with the same id

### Requirement: Stable ids
A station's id SHALL not change when the station is edited, moved, or when other stations are added or deleted, and SHALL not be reused after deletion.

#### Scenario: Edit keeps id
- **GIVEN** station id `1001`
- **WHEN** its name or URL is changed
- **THEN** its id is still `1001`

### Requirement: Validation on save
The system SHALL reject a station whose URL is not `http` or `https`, or whose name is empty, and SHALL report which field is wrong.

#### Scenario: Bad URL
- **WHEN** a station is saved with URL `ftp://example.com/stream`
- **THEN** the save is refused with an error on the URL field

### Requirement: Durable storage
The catalog SHALL be stored on the NAS in the package's data folder, written atomically, and SHALL survive package restart, upgrade and DSM reboot.

#### Scenario: Survives upgrade
- **GIVEN** a catalog with 20 stations
- **WHEN** the package is upgraded to a newer version
- **THEN** the same 20 stations, folders and order are served

#### Scenario: Interrupted write
- **WHEN** the NAS loses power while the catalog is being saved
- **THEN** on restart the catalog is either the old or the new version, never a corrupt file

### Requirement: Changes apply without restart
Changes to the catalog SHALL be visible to the receiver on its next request, without restarting the package.

#### Scenario: New station appears
- **WHEN** a station is added in the admin UI
- **THEN** the receiver lists it the next time it opens that folder

### Requirement: Import and export
The system SHALL export the whole catalog as one human-readable file and SHALL import such a file, either replacing the catalog or merging into it, after validating every entry.

#### Scenario: Round trip
- **WHEN** the catalog is exported and that file is imported with "replace"
- **THEN** the catalog is identical to the one exported, including ids and order

#### Scenario: Invalid import
- **WHEN** an import file holds one invalid station
- **THEN** nothing is imported and the error names the entry

### Requirement: Starter catalog
On first install the system SHALL create a small example catalog with clearly fictional placeholder entries, so the receiver shows a working menu straight away.

#### Scenario: First install
- **WHEN** the package is installed and started for the first time
- **THEN** the root menu shows at least one example folder

### Requirement: Invalid catalogs are refused whole
Validation SHALL refuse empty entries, ids below 0 or above 1073741823, a negative bitrate, fields beyond their length limit, and data after the end of the JSON document. The stored catalog SHALL stay unchanged when any of these is refused.

#### Scenario: Empty entry in an import
- **WHEN** an imported file has `null` in its list of folders
- **THEN** the import fails with an error and the receiver's list keeps working

### Requirement: Concurrent changes don't get lost
Changes to one catalog SHALL apply one at a time, so that every completed change is kept, and the file and the running service always hold the same catalog.

#### Scenario: Two imports at once
- **WHEN** two merge imports run at the same time
- **THEN** the catalog contains the stations of both

### Requirement: Virtual folders
A folder MAY hold a set of filters instead of stations, which makes it a virtual folder. The filters SHALL be country (ISO 3166-1 code), genre, format (MP3, AAC or AAC+) and text in the station name. All filters that are set SHALL match. A virtual folder SHALL set at least a country, a genre or a name, and SHALL hold no sub-folders and no stations. Its filters SHALL be saved, exported, imported and copied between receivers with the rest of the list, and it SHALL keep its place, id and rename and reorder behaviour like any folder.

#### Scenario: Valid virtual folder
- **WHEN** a list is saved with folder "Polish rock" and filters country `pl` and genre `rock`
- **THEN** the list is saved, with the country stored as `PL`

#### Scenario: No filter that selects stations
- **WHEN** a list is saved with a virtual folder that sets no filter, or only a format
- **THEN** the save is refused with a message naming the folder

#### Scenario: Virtual folder with stations
- **WHEN** a list is saved with a virtual folder that also holds stations or sub-folders
- **THEN** the save is refused with a message naming the folder
