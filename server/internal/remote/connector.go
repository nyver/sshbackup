package remote

import (
	"context"
	"time"

	"vpsbackupmanager/internal/backup"
)

// NewConnector returns a backup.Connector that dials the server over
// SSH/SFTP via Dial, using clock for retry backoff. This is the
// production wiring between the run engine's narrow Transport port and
// this package's real implementation.
func NewConnector(clock Clock) backup.Connector {
	return func(ctx context.Context, p backup.ConnectParams) (backup.Transport, error) {
		client, err := Dial(ctx, ConnectionParams{
			Host: p.Server.Host, Port: p.Server.Port, Username: p.Server.Username,
			PrivateKeyPEM: p.PrivateKeyPEM, Passphrase: p.Passphrase,
			TrustedFingerprint: p.Server.HostKeyFingerprint,
			ConnectTimeout:     time.Duration(p.Server.ConnectionTimeoutSeconds) * time.Second,
			CommandTimeout:     time.Duration(p.Server.CommandTimeoutSeconds) * time.Second,
		}, clock)
		if err != nil {
			return nil, err
		}
		return client, nil
	}
}
