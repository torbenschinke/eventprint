package wifi

import (
	"context"

	"go.wdy.de/nago/application/permission"
)

// Scan sucht nach empfangbaren Funknetzen.
//
// Der Aufruf dauert mehrere Sekunden, weil das Funkgerät dafür die Kanäle
// abhört. Die Oberfläche muss ihn deshalb nebenläufig führen.
type Scan func(subject permission.Auditable, ctx context.Context) ([]Network, error)

// NewScan bindet die Suche an NetworkManager.
func NewScan() Scan {
	return func(subject permission.Auditable, ctx context.Context) ([]Network, error) {
		if err := subject.Audit(PermScan); err != nil {
			return nil, err
		}

		// --rescan yes erzwingt eine frische Suche. Ohne das liefert nmcli den
		// zuletzt bekannten Stand, und wer die Fotobox gerade an einem neuen
		// Ort aufgebaut hat, sähe die Netze des vorigen.
		out, err := runNmcli(ctx, scanTimeout,
			"-t", "-f", "IN-USE,SSID,SIGNAL,SECURITY", "device", "wifi", "list", "--rescan", "yes")
		if err != nil {
			return nil, err
		}

		return ParseNetworks(out), nil
	}
}
