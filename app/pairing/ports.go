package pairing

import (
	"time"

	"go.wdy.de/nago/pkg/data"
)

// Accounts findet den Nutzer zu einer Adresse – aber nur, wenn er registriert,
// bestätigt und aktiv ist. Alle anderen Fälle sind "gibt es nicht".
type Accounts func(mail string) (Account, bool, error)

// Mailer verschickt eine Nachricht.
type Mailer func(to, subject, body string) error

// Issuer stellt ein Zugangstoken für eine Box aus und liefert seine Kennung
// und den Klartext. Den Klartext kennt danach nur noch die Box.
type Issuer func(account Account, device string) (BoxID, string, error)

// Revoker löscht das Zugangstoken einer Box. Ein schon gelöschtes Token ist
// kein Fehler.
type Revoker func(id BoxID) error

// OwnerGrant gibt einem Nutzer die Rolle, mit der er seine Boxen sieht und
// trennt. Sie wird beim ersten Koppeln vergeben und nicht pauschal jedem
// Nutzer, damit die Standardrollen des Betreibers unberührt bleiben.
type OwnerGrant func(accountID string) error

// Boxes speichert die gekoppelten Boxen.
type Boxes = data.Repository[Box, BoxID]

// Clock liefert die Zeit; die Tests stellen sie.
type Clock func() time.Time
