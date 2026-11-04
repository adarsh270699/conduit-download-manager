//go:build !windows

package platform

// MarksDownloads reports that this OS tracks where a file came from.
const MarksDownloads = false

// MarkFromInternet does nothing here. A downloaded file on Linux is not
// executable until the user makes it so.
func MarkFromInternet(string) error {
	return nil
}
