package domain

import (
	"testing"
	"time"
)

func TestNewServer(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name    string
		params  NewServerParams
		wantErr bool
	}{
		{
			name: "valid with private key",
			params: NewServerParams{
				Name: "prod", Host: "1.2.3.4", Username: "deploy",
				AuthType: AuthPrivateKey, PrivateKeyPath: `C:\keys\id_ed25519`,
			},
			wantErr: false,
		},
		{
			name:    "empty host",
			params:  NewServerParams{Name: "prod", Host: "", Username: "deploy", AuthType: AuthPassword},
			wantErr: true,
		},
		{
			name:    "port out of range",
			params:  NewServerParams{Name: "prod", Host: "1.2.3.4", Port: 70000, Username: "deploy", AuthType: AuthPassword},
			wantErr: true,
		},
		{
			name:    "private key auth without a key path",
			params:  NewServerParams{Name: "prod", Host: "1.2.3.4", Username: "deploy", AuthType: AuthPrivateKey},
			wantErr: true,
		},
		{
			name: "valid with password",
			params: NewServerParams{
				Name: "prod", Host: "1.2.3.4", Username: "deploy",
				AuthType: AuthPassword, CredentialReference: "ref-1",
			},
			wantErr: false,
		},
		{
			name:    "password auth without a stored password",
			params:  NewServerParams{Name: "prod", Host: "1.2.3.4", Username: "deploy", AuthType: AuthPassword},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			s, err := NewServer(now, tt.params)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewServer() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if s.ID == "" {
				t.Error("expected a generated ID")
			}
			if s.Port != DefaultServerPort {
				t.Errorf("expected default port %d, got %d", DefaultServerPort, s.Port)
			}
			if s.HasTrustedHostKey() {
				t.Error("a newly created server must not have a trusted host key yet")
			}
		})
	}
}
