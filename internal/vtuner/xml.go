package vtuner

import (
	"bytes"
	"encoding/xml"
	"strconv"
)

// item is one row of a ListOfItems document.
type item struct {
	kind string // Dir, Station, Search, Display
	// Dir
	title, urlDir string
	dirCount      int
	// Station
	id                                     int
	name, streamURL, desc, logo, mime, loc string
	bitrate                                int
	// Search
	searchURL string
	// Display
	text string
}

// label is the text a receiver shows for the item.
func (it item) label() string {
	switch it.kind {
	case "Station":
		return it.name
	case "Display":
		return it.text
	}
	return it.title
}

func esc(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return b.String()
}

func tag(b *bytes.Buffer, name, val string) {
	b.WriteString("<" + name + ">" + esc(val) + "</" + name + ">")
}

// render writes a ListOfItems document. total is the length of the whole list
// before paging; items is the requested page.
func render(total int, items []item) []byte {
	var b bytes.Buffer
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`)
	b.WriteString("<ListOfItems>")
	tag(&b, "ItemCount", strconv.Itoa(total))
	for _, it := range items {
		b.WriteString("<Item>")
		tag(&b, "ItemType", it.kind)
		// The Denon RCD-N7 shows an item without a Title as a row with no name.
		if it.kind == "Search" || it.kind == "Display" {
			tag(&b, "Title", it.label())
		}
		switch it.kind {
		case "Dir":
			tag(&b, "Title", it.title)
			tag(&b, "UrlDir", it.urlDir)
			tag(&b, "UrlDirBackUp", it.urlDir)
			tag(&b, "DirCount", strconv.Itoa(it.dirCount))
		case "Station":
			tag(&b, "StationId", strconv.Itoa(it.id))
			tag(&b, "StationName", it.name)
			tag(&b, "StationUrl", it.streamURL)
			tag(&b, "StationDesc", it.desc)
			tag(&b, "Logo", it.logo)
			tag(&b, "StationFormat", it.mime)
			tag(&b, "StationLocation", it.loc)
			tag(&b, "StationBandWidth", strconv.Itoa(it.bitrate))
			tag(&b, "StationMime", it.mime)
			tag(&b, "Relia", "3")
			tag(&b, "Bookmark", "")
		case "Search":
			tag(&b, "SearchURL", it.searchURL)
			tag(&b, "SearchURLBackUp", it.searchURL)
			tag(&b, "SearchCaption", it.title)
			b.WriteString("<SearchTextbox/>")
			tag(&b, "SearchButtonGo", "Search")
			tag(&b, "SearchButtonCancel", "Cancel")
		case "Display":
			tag(&b, "Display", it.text)
		}
		b.WriteString("</Item>")
	}
	b.WriteString("</ListOfItems>")
	return b.Bytes()
}
