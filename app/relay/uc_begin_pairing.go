package relay

import (
	"context"
	"net/mail"
	"os"
	"strings"

	"go.wdy.de/nago/application/permission"
)

// BeginPairing bittet den Upload-Dienst, einen Code an die Mailadresse des
// Besitzers zu schicken. Ob es zu der Adresse ein Konto gibt, erfährt die Box
// nicht; die Oberfläche sagt deshalb "falls es ein Konto gibt".
type BeginPairing func(subject permission.Auditable, ctx context.Context, cmd BeginPairingCmd) (Pairing, error)

// BeginPairingCmd ist die Eingabe an der Box.
type BeginPairingCmd struct {
	// URL ist die Adresse des Upload-Dienstes.
	URL string

	Mail string

	// Device ist der Name der Box für die Verwaltung des Dienstes. Leer
	// nimmt den Rechnernamen.
	Device string
}

// Pairing ist eine begonnene Kopplung.
type Pairing struct {
	ID   string
	URL  string
	Mail string
}

// Error ist ein Fehler mit einer Meldung für die Oberfläche.
type Error string

func (e Error) Error() string { return string(e) }

// ErrMail meldet eine Eingabe, die keine Mailadresse ist.
const ErrMail Error = "Das ist keine gültige Mailadresse."

// NewBeginPairing erzeugt den Anwendungsfall.
func NewBeginPairing() BeginPairing {
	return func(subject permission.Auditable, ctx context.Context, cmd BeginPairingCmd) (Pairing, error) {
		if err := subject.Audit(PermPair); err != nil {
			return Pairing{}, err
		}

		address := strings.TrimSpace(cmd.Mail)
		if a, err := mail.ParseAddress(address); err != nil || a.Address != address {
			return Pairing{}, ErrMail
		}

		device := strings.TrimSpace(cmd.Device)
		if device == "" {
			device, _ = os.Hostname()
		}

		c, err := NewClient(Options{URL: strings.TrimSpace(cmd.URL)})
		if err != nil {
			return Pairing{}, err
		}

		id, err := c.RequestPairing(ctx, address, device)
		if err != nil {
			return Pairing{}, err
		}

		return Pairing{ID: id, URL: strings.TrimSpace(cmd.URL), Mail: address}, nil
	}
}
