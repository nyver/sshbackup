package secrets

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"vpsbackupmanager/internal/domain"
)

// referenceByteLen matches domain.NewID's 16-byte (32 hex character)
// identifiers. Store rejects any other reference shape before it reaches a
// file path, since a reference can originate from a database row.
const referenceByteLen = 16

// Store persists DPAPI-protected secret blobs, one file per credential
// reference, under a directory such as
// %ProgramData%\VPSBackupManager\secrets.
type Store struct {
	dir string
}

// NewStore constructs a Store rooted at dir. The caller is responsible for
// creating dir (see internal/config.SecretsDir).
func NewStore(dir string) *Store {
	return &Store{dir: dir}
}

// Save protects plaintext and writes it under a newly generated credential
// reference, returning that reference for storage in SQLite.
func (s *Store) Save(plaintext []byte) (string, error) {
	reference, err := domain.NewID()
	if err != nil {
		return "", err
	}
	if err := s.writeProtected(reference, plaintext); err != nil {
		return "", err
	}
	return reference, nil
}

// Replace re-protects plaintext under an existing credential reference,
// overwriting the previous blob in place.
func (s *Store) Replace(reference string, plaintext []byte) error {
	return s.writeProtected(reference, plaintext)
}

func (s *Store) writeProtected(reference string, plaintext []byte) error {
	path, err := s.path(reference)
	if err != nil {
		return err
	}
	protected, err := dpapiProtect(plaintext)
	if err != nil {
		return fmt.Errorf("protect secret %q: %w", reference, err)
	}
	if err := os.WriteFile(path, protected, 0o600); err != nil {
		return fmt.Errorf("write secret %q: %w", reference, err)
	}
	return nil
}

// Load reads and decrypts the secret at reference. A missing file or a
// decryption failure (the database was copied to another machine, or a
// different service account now owns it) is mapped to an actionable error
// per the credential-storage specification: the user must re-enter the
// credential for the affected server.
func (s *Store) Load(reference string) ([]byte, error) {
	path, err := s.path(reference)
	if err != nil {
		return nil, err
	}
	protected, err := os.ReadFile(path) //nolint:gosec // path is validated by s.path to be a 32-hex-char reference joined under s.dir
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, domain.NewCodedError(domain.ErrSSHAuthFailed,
				"credential not found on this machine; please re-enter the credential for this server", err)
		}
		return nil, fmt.Errorf("read secret %q: %w", reference, err)
	}
	plaintext, err := dpapiUnprotect(protected)
	if err != nil {
		return nil, domain.NewCodedError(domain.ErrSSHAuthFailed,
			"credential could not be decrypted on this machine; please re-enter the credential for this server", err)
	}
	return plaintext, nil
}

// Delete removes the secret at reference. Deleting an already-absent
// reference is not an error, so callers can delete unconditionally.
func (s *Store) Delete(reference string) error {
	path, err := s.path(reference)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete secret %q: %w", reference, err)
	}
	return nil
}

func (s *Store) path(reference string) (string, error) {
	raw, err := hex.DecodeString(reference)
	if err != nil || len(raw) != referenceByteLen {
		return "", fmt.Errorf("invalid credential reference %q", reference)
	}
	return filepath.Join(s.dir, reference), nil
}
