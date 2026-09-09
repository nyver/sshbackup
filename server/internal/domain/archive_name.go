package domain

import (
	"strings"
	"time"
)

// windowsInvalidChars are the characters Windows forbids in file names:
// < > : " / \ | ? * and ASCII control characters.
const windowsInvalidChars = `<>:"/\|?*`

// ArchiveFileName builds the archive file name for a run of the given job,
// following {job-name}_{yyyy-MM-dd_HH-mm-ss}.tar.gz with characters
// forbidden in Windows file names replaced by "_". The job name itself is
// never modified — only the derived file name is sanitized.
func ArchiveFileName(jobName string, startedAt time.Time) string {
	safeName := sanitizeWindowsFileNameComponent(jobName)
	timestamp := startedAt.Format("2006-01-02_15-04-05")
	return safeName + "_" + timestamp + ".tar.gz"
}

func sanitizeWindowsFileNameComponent(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r < 0x20:
			b.WriteRune('_')
		case strings.ContainsRune(windowsInvalidChars, r):
			b.WriteRune('_')
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}
