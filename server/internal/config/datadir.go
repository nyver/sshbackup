// Package config resolves the application data directory and loads
// service-level configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
)

// AppDirName is the directory created under %ProgramData% that holds the
// database, secrets, and logs. It is never written next to the executable
// or into Program Files.
const AppDirName = "VPSBackupManager"

// DataDir resolves and creates the application data directory using the
// real %ProgramData% environment variable.
func DataDir() (string, error) {
	programData := os.Getenv("ProgramData")
	if programData == "" {
		return "", fmt.Errorf("resolve data directory: %%ProgramData%% is not set")
	}
	return EnsureDataDir(programData)
}

// EnsureDataDir joins root with AppDirName and creates it, and any missing
// parents, if it does not already exist. It is exported so tests can pass a
// temporary root instead of the real %ProgramData%.
func EnsureDataDir(root string) (string, error) {
	dir := filepath.Join(root, AppDirName)
	// root is the trusted %ProgramData% value (or a test-provided temp
	// dir); AppDirName is a fixed constant, so this is not attacker-tainted
	// path traversal despite the exported parameter.
	if err := os.MkdirAll(dir, 0o750); err != nil { //nolint:gosec // see comment above
		return "", fmt.Errorf("create data directory %q: %w", dir, err)
	}
	return dir, nil
}

// DatabasePath returns the SQLite database file path inside dataDir.
func DatabasePath(dataDir string) string {
	return filepath.Join(dataDir, "backup.db")
}

// SecretsDir returns the DPAPI-protected secrets directory inside dataDir,
// creating it if missing.
func SecretsDir(dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "secrets")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create secrets directory %q: %w", dir, err)
	}
	return dir, nil
}

// LogsDir returns the log directory inside dataDir, creating it if missing.
func LogsDir(dataDir string) (string, error) {
	dir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return "", fmt.Errorf("create logs directory %q: %w", dir, err)
	}
	return dir, nil
}
