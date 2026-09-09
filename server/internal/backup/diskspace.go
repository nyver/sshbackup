package backup

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	"vpsbackupmanager/internal/domain"
)

// checkLocalDestinationWritable creates dir if missing and probes it with
// a temporary file, since Windows ACLs cannot be reliably checked without
// attempting the actual operation.
func checkLocalDestinationWritable(dir string) checkResult {
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // dir is job.LocalDestination, a user-chosen but locally-owned config value, not attacker-controlled input
		return failedCheck(domain.ErrLocalPermissionDenied, fmt.Sprintf("local destination %s could not be created: %v", dir, err))
	}
	probe, err := os.CreateTemp(dir, ".vpsbackup-write-check-*")
	if err != nil {
		return failedCheck(domain.ErrLocalPermissionDenied, fmt.Sprintf("local destination %s is not writable: %v", dir, err))
	}
	name := probe.Name()
	_ = probe.Close()
	_ = os.Remove(name)
	return passed(fmt.Sprintf("local destination %s is writable", dir))
}

func measureLocalFreeSpace(dir string) (bytes int64, known bool) {
	free, err := localFreeBytes(dir)
	if err != nil {
		return 0, false
	}
	return free, true
}

func checkLocalFreeSpace(dir string, localFree int64, localFreeKnown bool, sourceSize int64, sourceSizeKnown bool) checkResult {
	if !localFreeKnown {
		return passed("local free space could not be measured; continuing without a space check")
	}
	if sourceSizeKnown && sourceSize > localFree {
		return failedCheck(domain.ErrLocalNoSpace, fmt.Sprintf(
			"estimated source size %d bytes exceeds local free space %d bytes on %s", sourceSize, localFree, dir))
	}
	return passed(fmt.Sprintf("local free space: %d bytes", localFree))
}

// localFreeBytes returns free space on the volume holding dir, via the
// Win32 GetDiskFreeSpaceEx API (no Go stdlib equivalent exists).
func localFreeBytes(dir string) (int64, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return 0, fmt.Errorf("resolve absolute path for %q: %w", dir, err)
	}
	ptr, err := windows.UTF16PtrFromString(abs)
	if err != nil {
		return 0, fmt.Errorf("encode path %q: %w", abs, err)
	}
	var freeBytesAvailable uint64
	if err := windows.GetDiskFreeSpaceEx(ptr, &freeBytesAvailable, nil, nil); err != nil {
		return 0, fmt.Errorf("GetDiskFreeSpaceEx(%q): %w", abs, err)
	}
	return int64(freeBytesAvailable), nil //nolint:gosec // disk sizes stay far below the int64/uint64 boundary
}
