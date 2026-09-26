package lightroom

import (
	"go.wdy.de/nago/application/permission"
)

// Disconnect vergisst die Verbindung zu Lightroom.
//
// Die Schlüssel werden nur auf der Box gelöscht. Die Freigabe selbst widerruft
// der Besitzer bei Adobe; ohne Refresh-Token kann die Box sie aber nicht mehr
// nutzen, und darauf kommt es an, wenn die Box verliehen wird.
type Disconnect func(subject permission.Auditable) error

// NewDisconnect bindet das Trennen an den Client.
func NewDisconnect(client *Client) Disconnect {
	return func(subject permission.Auditable) error {
		if err := subject.Audit(PermDisconnect); err != nil {
			return err
		}

		client.tokenMu.Lock()
		defer client.tokenMu.Unlock()

		if err := client.store.Delete(); err != nil {
			return err
		}

		client.forget()

		return nil
	}
}
