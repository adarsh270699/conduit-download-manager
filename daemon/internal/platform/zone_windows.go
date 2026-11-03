//go:build windows

package platform

import "os"

// MarksDownloads reports that this OS tracks where a file came from.
const MarksDownloads = true

// MarkFromInternet writes the Mark of the Web. A browser adds it to the files it
// downloads, and aria2 does not. It records only the zone: the source URL can
// hold a signed token. A volume without alternate data streams returns an error.
func MarkFromInternet(path string) error {
	return os.WriteFile(path+":Zone.Identifier", []byte("[ZoneTransfer]\r\nZoneId=3\r\n"), 0o644)
}
