package pairing

import (
	"crypto/rand"
	"encoding/base32"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"
)

// ID kennzeichnet eine Kopplung. Die Box bekommt sie beim Anfordern und
// schickt sie mit dem Code zurück; der Code allein genügt nicht.
type ID string

// Account ist ein Nutzer, der Boxen koppeln darf.
type Account struct {
	ID   string
	Mail string
}

// Status ist das Ergebnis einer Bestätigung.
type Status string

const (
	// StatusPaired: Der Code stimmte, die Box hat ihr Token.
	StatusPaired Status = "paired"

	// StatusInvalid: Der Code stimmt nicht – oder es gab zu der Adresse gar
	// kein Konto. Die beiden Fälle sehen absichtlich gleich aus.
	StatusInvalid Status = "invalid"

	// StatusExpired: Die 30 Minuten sind um.
	StatusExpired Status = "expired"

	// StatusLocked: zu viele falsche Versuche; ein neuer Code muss her.
	StatusLocked Status = "locked"
)

// Result ist die Antwort auf eine Bestätigung.
type Result struct {
	Status Status `json:"status"`

	// Token ist das Zugangstoken der Box, nur bei [StatusPaired]. Es wird
	// genau einmal herausgegeben und nirgends im Klartext gespeichert.
	Token string `json:"token,omitempty"`
}

const (
	// CodeTTL ist die Lebensdauer eines Codes.
	CodeTTL = 30 * time.Minute

	// MaxAttempts begrenzt die Versuche je Kopplung. Bei einer Million
	// möglicher Codes ist die Chance, mit fünf Versuchen zu raten, eins zu
	// zweihunderttausend – und danach ist die Kopplung verbraucht.
	MaxAttempts = 5

	// MailCooldown ist der Mindestabstand zwischen zwei Codes an dieselbe
	// Adresse. Wer an der Box zweimal tippt, bekommt nicht zwei Mails; wer
	// fremde Adressen zuschütten will, kommt nicht weit.
	MailCooldown = time.Minute

	// MailsPerHour begrenzt die Codes je Adresse und Stunde.
	MailsPerHour = 5

	// MaxPending begrenzt die offenen Kopplungen im Speicher. Eine Anfrage
	// kostet nichts außer diesem Eintrag; ohne Obergrenze ließe sich der
	// Dienst damit füllen.
	MaxPending = 10000

	// maxDeviceName ist die längste Gerätebezeichnung, die übernommen wird.
	maxDeviceName = 60
)

// normalizeMail macht Adressen vergleichbar.
func normalizeMail(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

// normalizeDevice macht aus einer Eingabe der Box einen Namen für das Token.
func normalizeDevice(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	for utf8.RuneCountInString(s) > maxDeviceName {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}

	if s == "" {
		return "Fotobox"
	}

	return s
}

// normalizeCode lässt nur Ziffern stehen: "123 456" und "123-456" sind
// derselbe Code.
func normalizeCode(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}

	return b.String()
}

// newID erzeugt eine unratbare Kennung.
func newID() (ID, error) {
	var buf [20]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}

	return ID(strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf[:]))), nil
}

// newCode erzeugt einen gleichverteilten sechsstelligen Code.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%06d", n.Int64()), nil
}
