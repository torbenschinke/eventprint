package upld

import "go.wdy.de/nago/auth"

// OpenSession legt eine neue, kurzlebige Upload-Identität für die anfragende
// Fotobox an und liefert deren Kennung.
//
// Eine bereits bestehende Sitzung derselben Fotobox wird dabei verworfen. Zwei
// gültige Adressen für dieselbe Box wären eine zu viel: Der QR-Code auf dem
// Startbildschirm zeigt nur eine davon.
type OpenSession func(subject auth.Subject) (UploadID, error)

// NewOpenSession erzeugt den [OpenSession] Anwendungsfall.
func NewOpenSession(registry *Registry) OpenSession {
	return func(subject auth.Subject) (UploadID, error) {
		if err := subject.Audit(PermOpenSession); err != nil {
			return "", err
		}

		return registry.Open(tokenOf(subject))
	}
}
