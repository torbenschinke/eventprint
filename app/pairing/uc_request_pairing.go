package pairing

import (
	"fmt"
	"log/slog"

	"go.wdy.de/nago/auth"
)

// RequestPairing beginnt die Kopplung einer Box mit einem Konto.
//
// Die Antwort ist in jedem Fall eine Kennung, auch wenn es zu der Adresse
// kein Konto gibt, und sie kommt ohne Fehlermeldung. So lässt sich über die
// Box nicht herausfinden, wer beim Dienst registriert ist. Nur ein
// registrierter, bestätigter und aktiver Nutzer bekommt einen Code.
//
// Jeder darf das aufrufen: Die Box hat noch kein Token, genau darum geht es
// ja. Schutz bieten die Drosselung je Adresse, die Obergrenze offener
// Kopplungen und die begrenzten Versuche beim Bestätigen.
//
// Aufgerufen wird sie deshalb vom öffentlichen Endpunkt als System, wie Nago
// es für Registrierung und Passwort-Reset tut; die Berechtigung benennt, was
// geschieht, und lässt sich einer Rolle entziehen, falls der Dienst die
// Kopplung abschalten soll.
type RequestPairing func(subject auth.Subject, cmd RequestCmd) (ID, error)

// RequestCmd ist die Anfrage der Box.
type RequestCmd struct {
	// Mail ist die Adresse, die an der Box eingetippt wurde.
	Mail string

	// Device ist der Name der Box, etwa der Titel der letzten Feier. Er
	// steht im Namen des Tokens, damit der Nutzer seine Boxen auseinander
	// halten kann.
	Device string
}

// NewRequestPairing bindet die Anfrage an Konten und Mailversand.
func NewRequestPairing(s *store, accounts Accounts, mailer Mailer, now Clock) RequestPairing {
	return func(subject auth.Subject, cmd RequestCmd) (ID, error) {
		if err := subject.Audit(PermRequestPairing); err != nil {
			return "", err
		}

		id, err := newID()
		if err != nil {
			return "", err
		}

		mail := normalizeMail(cmd.Mail)
		device := normalizeDevice(cmd.Device)

		account, known, err := accounts(mail)
		if err != nil {
			return "", err
		}

		t := now()

		s.mu.Lock()
		s.sweep(t)
		if len(s.pending) >= MaxPending {
			s.mu.Unlock()
			// Nach außen sieht auch das aus wie jede andere Anfrage; der
			// Betreiber sieht es im Protokoll.
			slog.Warn("pairing: too many pending pairings, request dropped")
			return id, nil
		}

		if recent, ok := s.recent(mail, t); ok {
			s.mu.Unlock()
			return recent, nil
		}

		p := &pending{account: account, mail: mail, known: known, device: device, expires: t.Add(CodeTTL)}
		send := known && s.mayMail(mail, t)
		s.last[mail] = recentPairing{id: id, at: t}
		if send {
			if p.code, err = newCode(); err != nil {
				s.mu.Unlock()
				return "", err
			}
		}

		s.pending[id] = p
		s.mu.Unlock()

		if !send {
			return id, nil
		}

		body := fmt.Sprintf(`Hallo,

an der Fotobox „%s“ wurde deine Adresse eingegeben, um die Box mit deinem Konto zu verbinden.

Dein Code: %s

Tippe ihn an der Fotobox ein. Er ist 30 Minuten gültig.

Warst du das nicht, ignoriere diese Nachricht. Ohne den Code geschieht nichts.
`, device, p.code)

		if err := mailer(account.Mail, "Dein Code für die Fotobox: "+p.code, body); err != nil {
			slog.Error("pairing: cannot send code", "err", err)
		}

		return id, nil
	}
}
