package pairing

import (
	"crypto/subtle"
	"fmt"
	"log/slog"

	"go.wdy.de/nago/auth"
)

// ConfirmPairing prüft den Code, den jemand an der Box eingetippt hat, und
// stellt bei Erfolg das Zugangstoken der Box aus.
//
// Die Antwort ist ein Status statt eines Fehlers. Die Box muss "Code falsch",
// "abgelaufen" und "zu viele Versuche" unterscheiden können, um etwas
// Sinnvolles anzuzeigen; ein Fehler käme bei ihr nur als nacktes 400 an.
// Fehler bleiben dem vorbehalten, was tatsächlich schiefging, etwa das
// Ausstellen des Tokens.
type ConfirmPairing func(subject auth.Subject, cmd ConfirmCmd) (Result, error)

// ConfirmCmd ist die Bestätigung der Box.
type ConfirmCmd struct {
	Pairing ID
	Code    string
}

// NewConfirmPairing bindet die Bestätigung an Token-Ausgabe und Mailversand.
func NewConfirmPairing(s *store, issue Issuer, mailer Mailer, now Clock) ConfirmPairing {
	return func(subject auth.Subject, cmd ConfirmCmd) (Result, error) {
		if err := subject.Audit(PermConfirmPairing); err != nil {
			return Result{}, err
		}

		code := normalizeCode(cmd.Code)
		t := now()

		s.mu.Lock()
		p, ok := s.pending[cmd.Pairing]
		switch {
		case !ok:
			s.mu.Unlock()
			return Result{Status: StatusInvalid}, nil
		case t.After(p.expires):
			delete(s.pending, cmd.Pairing)
			s.mu.Unlock()
			return Result{Status: StatusExpired}, nil
		case p.attempts >= MaxAttempts:
			s.mu.Unlock()
			return Result{Status: StatusLocked}, nil
		}

		p.attempts++

		// Ohne Konto oder ohne verschickten Code gibt es nichts, was passen
		// könnte. Verglichen wird trotzdem, damit die Antwortzeit das nicht
		// verrät.
		match := subtle.ConstantTimeCompare([]byte(code), []byte(p.code)) == 1 && p.known && p.code != "" && len(code) == 6
		if !match {
			locked := p.attempts >= MaxAttempts
			s.mu.Unlock()

			if locked {
				return Result{Status: StatusLocked}, nil
			}

			return Result{Status: StatusInvalid}, nil
		}

		// Verbraucht, bevor das Token entsteht: Zwei gleichzeitige
		// Bestätigungen mit demselben Code bekommen nicht zwei Tokens.
		delete(s.pending, cmd.Pairing)
		s.mu.Unlock()

		token, err := issue(p.account, p.device)
		if err != nil {
			return Result{}, fmt.Errorf("cannot issue token: %w", err)
		}

		body := fmt.Sprintf(`Hallo,

die Fotobox „%s“ ist jetzt mit deinem Konto verbunden. Fotos, die Gäste über ihren QR-Code senden, erreichen ab sofort diese Box.

Warst du das nicht, lösche das Zugangstoken „%s“ in der Verwaltung des Upload-Dienstes oder bitte den Betreiber darum.
`, p.device, TokenName(p.device))

		if err := mailer(p.account.Mail, "Fotobox „"+p.device+"“ verbunden", body); err != nil {
			slog.Error("pairing: cannot send confirmation", "err", err)
		}

		return Result{Status: StatusPaired, Token: token}, nil
	}
}

// TokenName ist der Name, unter dem das Token einer Box in der Verwaltung
// erscheint.
func TokenName(device string) string {
	return "Fotobox „" + device + "“"
}
