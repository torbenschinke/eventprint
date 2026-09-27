package pairing

import "time"

// Accounts findet den Nutzer zu einer Adresse – aber nur, wenn er registriert,
// bestätigt und aktiv ist. Alle anderen Fälle sind "gibt es nicht".
type Accounts func(mail string) (Account, bool, error)

// Mailer verschickt eine Nachricht.
type Mailer func(to, subject, body string) error

// Issuer stellt ein Zugangstoken für eine Box aus und liefert es im
// Klartext. Den Klartext kennt danach nur noch die Box.
type Issuer func(account Account, device string) (string, error)

// Clock liefert die Zeit; die Tests stellen sie.
type Clock func() time.Time
