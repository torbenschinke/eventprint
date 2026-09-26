package nas

import (
	"context"
	"slices"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// Shares meldet sich probeweise an und listet die Freigaben, aus denen man in
// den Einstellungen eine wählt.
//
// Ein leeres Kennwort bedeutet "das gespeicherte", solange Host und Benutzer
// gleich bleiben. So muss niemand ein Kennwort erneut tippen, nur um eine
// andere Freigabe zu wählen, und die Oberfläche bekommt es nie zu sehen.
type Shares func(subject permission.Auditable, ctx context.Context, cfg Config) ([]string, error)

// NewShares bindet die Suche an einen Client und die gespeicherten Daten.
func NewShares(client Client, saved func() Config) Shares {
	return func(subject permission.Auditable, ctx context.Context, cfg Config) ([]string, error) {
		if err := subject.Audit(PermShares); err != nil {
			return nil, err
		}

		cfg = cfg.Normalized()
		if cfg.Host == "" || cfg.User == "" {
			return nil, ErrNotConfigured
		}

		if s := saved().Normalized(); cfg.Password == "" && s.Host == cfg.Host && s.User == cfg.User {
			cfg.Password = s.Password
		}

		names, err := client.Shares(ctx, cfg)
		if err != nil {
			return nil, err
		}

		// Verwaltungsfreigaben wie IPC$ oder ADMIN$ enden auf "$" und
		// enthalten keine Fotos.
		names = slices.DeleteFunc(names, func(n string) bool { return n == "" || strings.HasSuffix(n, "$") })
		slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })

		return slices.Compact(names), nil
	}
}
