# Spec Delta

## Purpose

Keeps rows without a name off the receiver and lists virtual folders with the wider filters.

## ADDED Requirements

### Requirement: Every menu item has a name
Every item in any answer to the receiver SHALL have a name that isn't empty and isn't made only of spaces or invisible characters: `Title` for directory, search and informational items, and `StationName` for stations. An item without such a name SHALL be left out, and `ItemCount` SHALL count only the items left. A folder with a name and no stations SHALL still be listed.

#### Scenario: Nameless entries from the directory
- **GIVEN** the directory returns a genre named with only spaces and a station named with a zero-width space
- **WHEN** the receiver opens the genre menu and a station listing
- **THEN** neither entry is listed and the item count doesn't include them

#### Scenario: Empty folder with a name
- **GIVEN** the receiver's list has folder "Romantic" with no stations
- **WHEN** the receiver opens the root menu
- **THEN** "Romantic" is listed

#### Scenario: Search turned on
- **GIVEN** the `-search` option is on
- **WHEN** the receiver opens the root menu
- **THEN** the Search item has a `Title`

#### Scenario: Login path of the Denon RCD-N7
- **WHEN** the receiver logs in at `/setupapp/denon/asp/browsexm2/loginXML.asp` and opens every menu it is offered
- **THEN** no answer holds an item without a name

## MODIFIED Requirements

### Requirement: Virtual folder listing
The system SHALL list a virtual folder like any other folder in the menus, at the top level or inside a folder, and when the receiver opens it SHALL answer with the public directory stations that match the folder's filters, in the folder's sort order, up to the smaller of the folder's maximum and the number of stations a directory listing shows. These stations SHALL page, play and relay like any other directory station. When the directory is turned off or not reachable, or nothing matches, the answer SHALL be an empty list, and a directory failure SHALL be noted in the request log.

#### Scenario: Open a virtual folder
- **GIVEN** virtual folder "Polish rock" filters on country `PL` and genre `rock`
- **WHEN** the receiver opens "Polish rock"
- **THEN** the answer lists the matching directory stations, paged like any folder

#### Scenario: Several genres
- **GIVEN** virtual folder "80s and 90s" filters on genres `80s` and `90s`
- **WHEN** the receiver opens "80s and 90s"
- **THEN** the answer lists the stations of both genres, each once, paged like any folder

#### Scenario: Maximum number of stations
- **GIVEN** virtual folder "Top 20" has a maximum of 20 and the directory holds more matches
- **WHEN** the receiver opens "Top 20"
- **THEN** the item count is 20

#### Scenario: Inside a folder
- **GIVEN** folder "Poland" holds virtual folder "Polish rock"
- **WHEN** the receiver opens "Poland"
- **THEN** "Polish rock" is listed as a directory item

#### Scenario: HTTPS station
- **GIVEN** a matching station streams only over HTTPS
- **WHEN** the receiver opens the virtual folder
- **THEN** that station's URL is a relay URL

#### Scenario: Station count in the menu
- **GIVEN** virtual folder "Polish rock" matches 12 playable stations
- **WHEN** the receiver opens the menu that holds it
- **THEN** the folder reports 12 items

#### Scenario: Directory slow or down while listing the menu
- **GIVEN** the directory doesn't answer within 2 seconds
- **WHEN** the receiver opens the menu that holds virtual folders
- **THEN** the menu is still shown, with those folders reporting 0 items

#### Scenario: Directory not available when opening
- **GIVEN** the directory is turned off or not reachable
- **WHEN** the receiver opens a virtual folder
- **THEN** the answer is an empty list and the reason is in the request log

#### Scenario: Nothing matches
- **GIVEN** no directory station matches the filters
- **WHEN** the receiver opens the virtual folder
- **THEN** the answer is an empty list
