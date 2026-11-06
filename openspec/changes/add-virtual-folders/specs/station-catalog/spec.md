# Spec Delta

## Purpose

Widens virtual folder filters, refuses blank names and keeps older files loading.

## ADDED Requirements

### Requirement: Names can't be blank
Saving a list SHALL be refused when a folder or station name is empty or made only of spaces or invisible characters. Spaces and invisible characters at either end of a name SHALL be removed on saving.

#### Scenario: Folder with a blank name
- **WHEN** a list is saved with a folder named with only spaces, or only a zero-width space
- **THEN** the save is refused and the stored list doesn't change

#### Scenario: Invisible character before a name
- **WHEN** a list is saved with a folder whose name is a zero-width space followed by "Jazz"
- **THEN** the folder is saved as "Jazz"

### Requirement: Lists from older versions load
Catalog files from earlier versions SHALL load, whether they have no virtual folders or virtual folders that use only country, genre, format and name. Files of data version 1 SHALL be upgraded to data version 2, in the data folder and on import, and a virtual folder's genre SHALL become a list with that one genre.

#### Scenario: File without virtual folders
- **GIVEN** a catalog file saved by package 0.1.0-0003
- **WHEN** the service starts
- **THEN** the list loads and its folders and stations are unchanged

#### Scenario: File with the first virtual folders
- **GIVEN** a catalog file with a virtual folder that sets country and genre only
- **WHEN** the service starts
- **THEN** the list loads, the folder has that one genre in its list of genres, and it has no language, minimum bitrate, sort order or maximum set

#### Scenario: Upgrading the data folder
- **GIVEN** a data folder of data version 1 with a virtual folder inside a folder, filtering on genre `rock`
- **WHEN** the service starts
- **THEN** the virtual folder's genres are `rock`, the data version is 2, and a copy of the old data is in the backups folder

#### Scenario: Importing an older file
- **WHEN** a file of data version 1 with a virtual folder filtering on genre `rock` is imported
- **THEN** the imported folder's genres are `rock`

## MODIFIED Requirements

### Requirement: Virtual folders
A folder MAY hold a set of filters instead of stations, which makes it a virtual folder. The filters SHALL be country (ISO 3166-1 code), genres (up to 10), format (MP3, AAC or AAC+), text in the station name, language, minimum bitrate, sort order (`clickcount`, `votes`, `clicktrend`, `bitrate` or `name`) and maximum number of stations (up to 1000). All filters that are set SHALL match, and a station SHALL match the genres filter when it has any one of them. Blank and repeated genres SHALL be dropped on saving, ignoring case. A virtual folder SHALL set at least one of them, and SHALL hold no sub-folders and no stations. It MAY be at the top level or inside a folder. Its filters SHALL be saved, exported, imported and copied between receivers with the rest of the list, and it SHALL keep its place, id and rename and reorder behaviour like any folder.

#### Scenario: Valid virtual folder
- **WHEN** a list is saved with folder "Polish rock" and filters country `pl` and genre `rock`
- **THEN** the list is saved, with the country stored as `PL`

#### Scenario: Several genres
- **WHEN** a list is saved with folder "80s and 90s" and genres `80s`, `90s` and `80S`
- **THEN** the list is saved with genres `80s` and `90s`

#### Scenario: Too many genres
- **WHEN** a list is saved with a virtual folder with 11 genres
- **THEN** the save is refused with a message naming the folder and the field

#### Scenario: One filter is enough
- **WHEN** a list is saved with a virtual folder that sets only a language
- **THEN** the list is saved

#### Scenario: No filter
- **WHEN** a list is saved with a virtual folder that sets no filter
- **THEN** the save is refused with a message naming the folder

#### Scenario: Value out of range
- **WHEN** a list is saved with a virtual folder with an unknown sort order, a negative minimum bitrate, or a maximum above 1000
- **THEN** the save is refused with a message naming the folder and the field

#### Scenario: Virtual folder with stations
- **WHEN** a list is saved with a virtual folder that also holds stations or sub-folders
- **THEN** the save is refused with a message naming the folder

#### Scenario: Export and import
- **GIVEN** a list with a virtual folder that sets every filter
- **WHEN** the list is exported and imported, by replacing or merging
- **THEN** the imported folder has the same filters
