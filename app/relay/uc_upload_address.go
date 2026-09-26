package relay

import (
	"net/url"

	"go.wdy.de/nago/application/permission"
)

// Address ist das, was der QR-Code zeigen soll.
type Address struct {
	// URL ist leer, solange kein brauchbarer Code gezeigt werden kann.
	URL string

	State State

	// Problem erklärt einen leeren Code in Worten.
	Problem string
}

// UploadAddress liefert die Adresse für den QR-Code.
//
// Im Heimbetrieb bekommt sie den Parameter m=inbox. Die Upload-Seite zeigt
// dann keine Layoutwahl, sondern nimmt mehrere Bilder auf einmal an – sie
// landen im Eingang, und gestaltet wird am Gerät.
type UploadAddress func(subject permission.Auditable, inbox bool) (Address, error)

// NewUploadAddress erzeugt den [UploadAddress] Anwendungsfall.
func NewUploadAddress(p *Poller) UploadAddress {
	return func(subject permission.Auditable, inbox bool) (Address, error) {
		if err := subject.Audit(PermUploadAddress); err != nil {
			return Address{}, err
		}

		a := Address{State: p.State()}
		switch a.State {
		case StateOff:
			a.Problem = "Kein Upload-Dienst eingerichtet."
			return a, nil
		case StateMissingToken:
			a.Problem = "Für den Upload-Dienst fehlt das Zugangstoken."
			return a, nil
		case StateConnecting:
			a.Problem = "Verbindung zum Upload-Dienst wird aufgebaut."
			if e := p.LastError(); e != "" {
				a.Problem = "Upload-Dienst nicht erreichbar."
			}

			return a, nil
		}

		a.URL = p.UploadURL()
		if inbox {
			a.URL = withInbox(a.URL)
		}

		return a, nil
	}
}

func withInbox(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}

	q := u.Query()
	q.Set("m", "inbox")
	u.RawQuery = q.Encode()

	return u.String()
}
