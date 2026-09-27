package embedding

import (
	"context"

	"github.com/go-i2p/go-sam-bridge/lib/i2cp"
	"github.com/go-i2p/go-sam-bridge/lib/session"
)

// i2cpProviderAdapter adapts the bridge I2CP client to the session package
// without leaking its configuration compatibility type to embedders.
type i2cpProviderAdapter struct {
	client *i2cp.Client
}

func newI2CPProviderAdapter(client *i2cp.Client) *i2cpProviderAdapter {
	return &i2cpProviderAdapter{client: client}
}

func (a *i2cpProviderAdapter) CreateSessionForSAM(ctx context.Context, id string, cfg *session.SessionConfig) (session.I2CPSessionHandle, error) {
	if cfg == nil {
		return a.client.CreateSessionForSAM(ctx, id, nil)
	}

	i2cpConfig := &i2cp.SessionConfigFromSession{
		SignatureType:          cfg.SignatureType,
		EncryptionTypes:        cfg.EncryptionTypes,
		InboundQuantity:        cfg.InboundQuantity,
		OutboundQuantity:       cfg.OutboundQuantity,
		InboundLength:          cfg.InboundLength,
		OutboundLength:         cfg.OutboundLength,
		InboundBackupQuantity:  cfg.InboundBackupQuantity,
		OutboundBackupQuantity: cfg.OutboundBackupQuantity,
		FastReceive:            cfg.FastReceive,
		ReduceIdleTime:         cfg.ReduceIdleTime,
		CloseIdleTime:          cfg.CloseIdleTime,
	}
	return a.client.CreateSessionForSAM(ctx, id, i2cpConfig)
}

func (a *i2cpProviderAdapter) IsConnected() bool {
	return a.client != nil && a.client.IsConnected()
}

var _ session.I2CPSessionProvider = (*i2cpProviderAdapter)(nil)
