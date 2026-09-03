package wifi

import (
	"context"
	"time"

	"go.wdy.de/nago/application/permission"
)

// Current liefert die bestehende Verbindung.
type Current func(subject permission.Auditable, ctx context.Context) (Status, error)

// NewCurrent bindet die Abfrage an NetworkManager.
func NewCurrent() Current {
	return func(subject permission.Auditable, ctx context.Context) (Status, error) {
		if err := subject.Audit(PermStatus); err != nil {
			return Status{}, err
		}

		// Ohne erneute Suche: Der Zustand soll auch dann sofort dastehen, wenn
		// gerade niemand nach Netzen sucht. Die Feldstärke kommt trotzdem aus
		// der Netzliste, weil sie in der Geräteliste nicht steht.
		out, err := runNmcli(ctx, statusTimeout,
			"-t", "-f", "IN-USE,SSID,SIGNAL,SECURITY", "device", "wifi", "list", "--rescan", "no")
		if err != nil {
			return Status{}, err
		}

		nets := ParseNetworks(out)

		devices, err := runNmcli(ctx, statusTimeout, "-t", "-f", "DEVICE,TYPE,STATE,CONNECTION", "device", "status")
		if err != nil {
			return Status{}, err
		}

		return ParseStatus(devices, nets), nil
	}
}

// statusTimeout begrenzt die Abfrage. Sie fragt nur ab und sucht nicht, darf
// also kurz sein.
const statusTimeout = 10 * time.Second
