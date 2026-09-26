package nas

import (
	"context"
	"io/fs"
)

// Client ist der Zugang zu Freigaben. Die Anwendungsfälle kennen nur diese
// Schnittstelle; die Tests setzen ein Dateisystem im Speicher ein, das Gerät
// den SMB-Client aus smb.go.
type Client interface {
	// Shares meldet sich an und listet die Freigaben, die der Benutzer
	// sehen darf, einschließlich verwaltungstechnischer wie "IPC$".
	Shares(ctx context.Context, cfg Config) ([]string, error)

	// Do führt fn auf der Wurzel der Freigabe cfg.Share aus. fn darf das
	// Dateisystem nur bis zu seiner Rückkehr benutzen.
	Do(ctx context.Context, cfg Config, fn func(fsys fs.FS) error) error
}
