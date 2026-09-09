package domain

import (
	"errors"
	"time"
)

// Server is a registered VPS host reachable over SSH.
type Server struct {
	ID       string
	Name     string
	Host     string
	Port     int
	Username string

	AuthType            AuthenticationType
	CredentialReference string // opaque reference into internal/secrets; empty when the key has no passphrase
	PrivateKeyPath      string
	HostKeyAlgorithm    string // empty until trust-on-first-use has run
	HostKeyFingerprint  string // SHA-256 fingerprint, empty until trust-on-first-use has run

	ConnectionTimeoutSeconds int
	CommandTimeoutSeconds    int

	CreatedAt time.Time
	UpdatedAt time.Time
}

const (
	// DefaultServerPort is used when a server is created without an
	// explicit port.
	DefaultServerPort = 22
	// DefaultConnectionTimeoutSecs is used when a server is created
	// without an explicit connection timeout.
	DefaultConnectionTimeoutSecs = 10
	// DefaultCommandTimeoutSecs is used when a server is created without
	// an explicit default command timeout.
	DefaultCommandTimeoutSecs = 30
)

// NewServerParams carries the user-supplied fields for creating a Server.
// Timestamps and ID are assigned by NewServer.
type NewServerParams struct {
	Name                     string
	Host                     string
	Port                     int
	Username                 string
	AuthType                 AuthenticationType
	CredentialReference      string
	PrivateKeyPath           string
	ConnectionTimeoutSeconds int
	CommandTimeoutSeconds    int
}

// NewServer validates params and constructs a Server with a fresh ID and
// timestamps. The host key fields start empty: they are populated only
// after an explicit trust-on-first-use confirmation.
func NewServer(now time.Time, p NewServerParams) (*Server, error) {
	if p.Port == 0 {
		p.Port = DefaultServerPort
	}
	if p.ConnectionTimeoutSeconds == 0 {
		p.ConnectionTimeoutSeconds = DefaultConnectionTimeoutSecs
	}
	if p.CommandTimeoutSeconds == 0 {
		p.CommandTimeoutSeconds = DefaultCommandTimeoutSecs
	}

	s := &Server{
		Name:                     p.Name,
		Host:                     p.Host,
		Port:                     p.Port,
		Username:                 p.Username,
		AuthType:                 p.AuthType,
		CredentialReference:      p.CredentialReference,
		PrivateKeyPath:           p.PrivateKeyPath,
		ConnectionTimeoutSeconds: p.ConnectionTimeoutSeconds,
		CommandTimeoutSeconds:    p.CommandTimeoutSeconds,
		CreatedAt:                now,
		UpdatedAt:                now,
	}
	if err := s.Validate(); err != nil {
		return nil, err
	}
	id, err := NewID()
	if err != nil {
		return nil, err
	}
	s.ID = id
	return s, nil
}

// Validate checks the invariants required by the server-management
// specification: non-empty name/host/username and a valid port.
func (s *Server) Validate() error {
	var errs []error
	if err := ValidateName("server", s.Name); err != nil {
		errs = append(errs, err)
	}
	if s.Host == "" {
		errs = append(errs, errors.New("server host must not be empty"))
	}
	if s.Username == "" {
		errs = append(errs, errors.New("server username must not be empty"))
	}
	if err := ValidatePort(s.Port); err != nil {
		errs = append(errs, err)
	}
	if s.AuthType != AuthPrivateKey && s.AuthType != AuthPassword {
		errs = append(errs, errors.New("server authentication type must be PRIVATE_KEY or PASSWORD"))
	}
	if s.AuthType == AuthPrivateKey && s.PrivateKeyPath == "" {
		errs = append(errs, errors.New("private key path must not be empty for PRIVATE_KEY authentication"))
	}
	return errors.Join(errs...)
}

// HasTrustedHostKey reports whether trust-on-first-use has already
// confirmed a fingerprint for this server.
func (s *Server) HasTrustedHostKey() bool {
	return s.HostKeyFingerprint != ""
}
