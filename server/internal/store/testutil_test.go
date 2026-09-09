package store

import (
	"os"
	"testing"
	"time"
)

// registerDBCleanup closes db and removes its file plus any WAL/SHM
// sidecar files, retrying briefly. On Windows, antivirus or indexing can
// hold a just-closed file open for a few milliseconds, which would
// otherwise make t.TempDir()'s own cleanup fail with "directory not empty".
func registerDBCleanup(t *testing.T, db *DB, path string) {
	t.Helper()
	t.Cleanup(func() {
		_ = db.Close()
		for _, suffix := range []string{"-wal", "-shm", ""} {
			p := path + suffix
			for i := 0; i < 20; i++ {
				err := os.Remove(p)
				if err == nil || os.IsNotExist(err) {
					break
				}
				time.Sleep(25 * time.Millisecond)
			}
		}
	})
}
