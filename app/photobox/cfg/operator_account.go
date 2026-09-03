package cfgphotobox

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"go.wdy.de/nago/application"
	"go.wdy.de/nago/application/role"
	"go.wdy.de/nago/application/user"
)

// OperatorMail ist das Konto, in das die PIN anmeldet.
//
// Die PIN meldet in ein echtes Benutzerkonto an, statt der Sitzung ein
// selbstgebautes Subject unterzuschieben. Der erste Versuch tat genau das und
// war zu schlau: Nagos Admin-Center prueft `Subject().Valid()`, und die
// Einstellungen verlangen `nago.settings.global.*`. Eine Huelle, die nur die
// Berechtigungen dieser Anwendung kennt, faellt dort durch - die Fotobox liess
// sich nicht einrichten, obwohl die PIN stimmte.
//
// Mit einem echten Konto regeln die Rollen alles, auch alles, was Nago
// spaeter dazubaut. Es gibt nichts von Hand nachzupflegen.
const OperatorMail user.Email = "betreuer@fotobox.local"

// ensureOperatorUser legt das Betreuerkonto an und haelt es brauchbar.
//
// Das Konto hat ein zufaelliges, niemandem bekanntes Passwort. Es wird nie
// gebraucht: Die Anmeldung laeuft ueber die PIN und session.LoginUser, das
// kein Passwort prueft. Ein Passwort, das jemand kennt, waere ein zweiter
// Zugang, den niemand pflegt.
func ensureOperatorUser(users application.UserManagement) (user.ID, error) {
	optUsr, err := users.UseCases.FindByMail(user.SU(), OperatorMail)
	if err != nil {
		return "", fmt.Errorf("cannot look up operator account: %w", err)
	}

	var uid user.ID

	if optUsr.IsSome() {
		uid = optUsr.Unwrap().ID
	} else {
		pwd, err := randomPassword()
		if err != nil {
			return "", err
		}

		usr, err := users.UseCases.Create(user.SU(), user.ShortRegistrationUser{
			Email:            OperatorMail,
			Firstname:        "Fotobox",
			Lastname:         "Betreuung",
			Password:         pwd,
			PasswordRepeated: pwd,

			// Ohne bestaetigte Mailadresse gilt das Konto Nago als ungueltig,
			// und jede Rechtepruefung scheitert - siehe viewImpl.Valid. Hier
			// gibt es keinen Postkasten, an den sich eine Bestaetigung
			// schicken liesse.
			Verified:   true,
			NotifyUser: false,
		})
		if err != nil {
			return "", fmt.Errorf("cannot create operator account: %w", err)
		}

		uid = usr.ID
	}

	// Bei jedem Start nachziehen: Ein bestehendes Konto kann aus einer
	// aelteren Fassung stammen oder von Hand veraendert worden sein.
	if err := users.UseCases.UpdateVerification(user.SU(), uid, true); err != nil {
		return "", fmt.Errorf("cannot verify operator account: %w", err)
	}

	if err := users.UseCases.UpdateAccountStatus(user.SU(), uid, user.Enabled{}); err != nil {
		return "", fmt.Errorf("cannot enable operator account: %w", err)
	}

	if err := users.UseCases.UpdateOtherRoles(user.SU(), uid, []role.ID{OperatorRole}); err != nil {
		return "", fmt.Errorf("cannot grant the operator role: %w", err)
	}

	return uid, nil
}

// randomPassword wuerfelt ein Passwort, das niemand kennen muss.
func randomPassword() (user.Password, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("no randomness available: %w", err)
	}

	// Gross- und Kleinbuchstaben, Ziffern und ein Sonderzeichen, damit die
	// Staerkepruefung von Nago das Passwort annimmt.
	return user.Password(base64.RawURLEncoding.EncodeToString(buf) + "aA1!"), nil
}
