# Spec Delta

## Purpose

Answers the HTTP requests a Denon receiver sends to vTuner hosts, so its Internet Radio menu lists and plays the stations from the local catalog.

## ADDED Requirements

### Requirement: Serve vTuner host names over plain HTTP
The system SHALL answer HTTP requests on port 80 whose `Host` header is `denon.vtuner.com` or any other `*.vtuner.com` name, using plain HTTP without redirects to HTTPS.

#### Scenario: Request to the Denon vTuner host
- **WHEN** the receiver sends `GET http://denon.vtuner.com/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0` to the NAS at `192.168.1.10`
- **THEN** the system answers with HTTP status 200 and an XML body, without a redirect

#### Scenario: Request for a different vTuner host
- **WHEN** a request arrives with `Host: example.vtuner.com`
- **THEN** the system handles it exactly as it handles `denon.vtuner.com`

#### Scenario: Request for DSM itself
- **WHEN** a browser opens `http://192.168.1.10/` or a host name that is not a vTuner name
- **THEN** the request is not handled by this system and DSM behaves as it did before installation

### Requirement: Path matching is case-insensitive
The system SHALL treat request paths and query parameter names as case-insensitive, and SHALL accept the vendor segment of the path (for example `denon`) with any value.

#### Scenario: Mixed-case path
- **WHEN** the receiver requests `/setupapp/Denon/asp/browsexml/LOGINXML.ASP?Token=0`
- **THEN** the system answers as it does for `/setupapp/denon/asp/BrowseXML/loginXML.asp?token=0`

### Requirement: Login handshake
The system SHALL answer the login request with a fixed token, so the receiver continues to the menu without contacting any other service.

#### Scenario: Token request
- **WHEN** the receiver requests `loginXML.asp?token=0`
- **THEN** the body is `<EncryptedToken>` holding a 16-character placeholder value such as `0000000000000000`

#### Scenario: Login with receiver details
- **WHEN** the receiver requests `loginXML.asp` with `mac`, `dlang` and `fver` parameters, for example `mac=001122334455`
- **THEN** the system answers with the root menu, and records the parameters in the request log

### Requirement: Root menu
The system SHALL return the receiver's top-level folders in their saved order, then "Browse by country" and "Browse by genre" when the public directory is on. The "Search" entry SHALL be left out unless the `-search` option turns it on, and every entry shown SHALL have a name.

#### Scenario: Catalog with two folders
- **GIVEN** the catalog has folders "News" and "Jazz", in that order
- **WHEN** the receiver opens the root menu
- **THEN** the answer lists "News", then "Jazz", each as a directory item with its number of children, followed by the browse entries

#### Scenario: No row without a name
- **GIVEN** the `-search` option is off
- **WHEN** the receiver opens the root menu
- **THEN** the answer has no Search item, because the Denon RCD-N7 shows that item as a row with no name that lists nothing

#### Scenario: Search turned on
- **GIVEN** the `-search` option is on
- **WHEN** the receiver opens the root menu
- **THEN** the answer ends with the Search item

#### Scenario: Empty catalog
- **GIVEN** the catalog has no folders or stations and the public directory is off
- **WHEN** the receiver opens the root menu
- **THEN** the answer is a valid list with one informational item telling the user to add stations in the admin UI

### Requirement: Folder listing
The system SHALL list a folder's sub-folders and stations, in catalog order, with each directory item linking to its own listing and each station item carrying its id, name, stream URL, format and description.

#### Scenario: Open a folder
- **GIVEN** folder "Jazz" holds stations "Example Jazz FM" and "Example Smooth"
- **WHEN** the receiver opens "Jazz"
- **THEN** the answer lists both stations as station items, in that order

### Requirement: Paging of long lists
The system SHALL honour the receiver's `startItems` and `endItems` parameters (1-based, inclusive) and SHALL report the total count of the full list.

#### Scenario: Second page
- **GIVEN** a folder holds 25 stations
- **WHEN** the receiver requests it with `startItems=11&endItems=20`
- **THEN** the answer holds stations 11 to 20 and reports a total of 25

#### Scenario: Range past the end
- **WHEN** the receiver requests `startItems=21&endItems=40` on a folder of 25
- **THEN** the answer holds stations 21 to 25

### Requirement: Station lookup by id
The system SHALL return a single station item when the receiver asks for a station by its id, and SHALL give the stream URL the receiver can play (the original URL, or the relay URL when the station uses the relay).

#### Scenario: Known station
- **WHEN** the receiver requests the station with id `1001`
- **THEN** the answer holds one station item for that station

#### Scenario: Unknown station
- **WHEN** the receiver requests a station id that is not in the catalog
- **THEN** the answer is a valid empty list and the event is logged

### Requirement: Virtual folder listing
The system SHALL list a virtual folder like any other folder in the menus, and when the receiver opens it SHALL answer with the public directory stations that match the folder's filters, most popular first, up to the number of stations a directory listing shows. These stations SHALL play and relay like any other directory station.

#### Scenario: Open a virtual folder
- **GIVEN** virtual folder "Polish rock" filters on country `PL` and genre `rock`
- **WHEN** the receiver opens "Polish rock"
- **THEN** the answer lists the matching directory stations, paged like any folder

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
- **THEN** the answer is one informational item saying so, and the event is logged

#### Scenario: Nothing matches
- **GIVEN** no directory station matches the filters
- **WHEN** the receiver opens the virtual folder
- **THEN** the answer is one informational item saying that no stations match

### Requirement: Search over the local catalog
The system SHALL answer the receiver's search request with stations from the receiver's list whose name or description contains the search text, ignoring case and diacritics.

#### Scenario: Search hit
- **GIVEN** the catalog holds "Example Jazz FM"
- **WHEN** the receiver searches for `jazz`
- **THEN** the answer lists "Example Jazz FM"

#### Scenario: Search miss
- **WHEN** the receiver searches for text that matches nothing in its list
- **THEN** the answer is a valid empty list

### Requirement: Well-formed XML for the receiver
The system SHALL return well-formed XML with escaped special characters, a `text/xml` content type and an explicit content length, and SHALL keep titles within the receiver's display limits by shortening them.

#### Scenario: Station name with an ampersand
- **GIVEN** a station named "Rock & Roll Example"
- **WHEN** it appears in a listing
- **THEN** the name is escaped as `Rock &amp; Roll Example` and the document parses as XML

### Requirement: Safe answer for unknown requests
The system SHALL answer any unrecognised path under a vTuner host with a valid empty list and status 200, and SHALL record the request in the request log.

#### Scenario: Unknown path
- **WHEN** the receiver requests `/setupapp/denon/asp/BrowseXML/unknown.asp`
- **THEN** the answer is a valid empty list and the request appears in the request log

### Requirement: Request log
The system SHALL keep the most recent receiver requests (time, client IP, host, path, query, status) in a bounded log that the admin UI can show, and SHALL not keep more than a configured number of entries.

#### Scenario: Log rotation
- **GIVEN** the log limit is 500 entries
- **WHEN** the 501st request arrives
- **THEN** the oldest entry is dropped
