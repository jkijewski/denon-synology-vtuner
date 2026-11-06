# Spec Delta

## Purpose

Adds the wider filters, placement, a preview before saving and a badge to virtual folder management.

## MODIFIED Requirements

### Requirement: Manage virtual folders
The admin UI SHALL let the user create a virtual folder with a name and filters for country, any number of genres up to 10, language, format, minimum bitrate, station name, sort order and maximum number of stations, at the top level or inside a plain folder, from the folder list or from the current filters on the public directory page, and edit, rename, reorder and delete it. The form SHALL have a Preview button that shows the stations matching the current filters without saving anything. Virtual folders SHALL have a distinct badge in the folder list. Selecting a virtual folder SHALL show its filters and the stations that match now. The UI SHALL NOT offer a virtual folder as a place to add or copy stations.

#### Scenario: Save directory filters as a folder
- **GIVEN** the user has chosen country Poland and genre rock on the public directory page
- **WHEN** the user clicks Save as virtual folder and gives it a name
- **THEN** the folder is saved in the selected receiver's list with those filters

#### Scenario: Choosing several genres
- **GIVEN** the virtual folder form is open
- **WHEN** the user types `80` in the genre search, ticks `80s`, types `90` and ticks `90s`
- **THEN** both genres stay chosen and are shown as chosen, the form says a station needs any one of them, and Preview and saving use both

#### Scenario: Removing a chosen genre
- **GIVEN** the form shows `80s` and `90s` as chosen
- **WHEN** the user removes `90s`
- **THEN** only `80s` stays chosen

#### Scenario: Saved genre no longer in the list
- **GIVEN** a virtual folder has a genre the directory's top genres no longer include
- **WHEN** the user edits the folder and clicks OK
- **THEN** the genre is kept

#### Scenario: Preview before saving
- **GIVEN** the virtual folder form is open with country Poland and genre rock
- **WHEN** the user clicks Preview
- **THEN** the form shows the number of matching stations and the first ones, the same ones the receiver would list, and the list isn't changed

#### Scenario: Preview when the directory is down
- **GIVEN** the directory isn't reachable
- **WHEN** the user clicks Preview
- **THEN** the form says so and stays open

#### Scenario: Inside a folder
- **WHEN** the user creates a virtual folder and chooses plain folder "Poland" under Place in
- **THEN** the virtual folder is saved inside "Poland"

#### Scenario: Blank folder name
- **WHEN** the user enters only spaces as the name and clicks OK
- **THEN** the form asks for a name and stays open

#### Scenario: Badge
- **WHEN** the folder list shows a virtual folder
- **THEN** it has a "virtual" badge in place of a station count

#### Scenario: Selecting a virtual folder
- **WHEN** the user selects a virtual folder
- **THEN** the page shows its filters, the number of matching stations and the first ones, with several genres shown as `80s or 90s`

#### Scenario: Adding a station from the directory
- **GIVEN** the selected receiver has folder "Jazz" and virtual folder "Polish rock"
- **WHEN** the user clicks Add next to a directory station
- **THEN** the folder list offers "Jazz" but not "Polish rock"
