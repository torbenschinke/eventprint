package photoupld

import (
	"fmt"
	netmail "net/mail"
	"time"

	"go.wdy.de/nago/application"
	"go.wdy.de/nago/application/mail"
	"go.wdy.de/nago/application/role"
	"go.wdy.de/nago/application/token"
	"go.wdy.de/nago/application/user"

	"github.com/torbenschinke/eventprint/app/pairing"
)

// enablePairing schließt die Kopplung an Nagos Nutzer, Mailversand und
// Zugangstoken an.
//
// Das Token einer gekoppelten Box trägt dieselbe Rolle, die ein Administrator
// bisher von Hand vergab, und nichts darüber hinaus: Es impersoniert den
// Nutzer nicht. Die Box kann damit Uploads abholen, aber nicht im Namen des
// Nutzers handeln. Name und Beschreibung sagen, wer die Box wann gekoppelt
// hat, damit sie sich in der Verwaltung wiederfinden und löschen lässt.
func enablePairing(cfg *application.Configurator, tokens application.TokenManagement) (pairing.UseCases, error) {
	users, err := cfg.UserManagement()
	if err != nil {
		return pairing.UseCases{}, err
	}

	mails, err := cfg.MailManagement()
	if err != nil {
		return pairing.UseCases{}, err
	}

	return pairing.NewUseCases(pairing.Options{
		Accounts: func(address string) (pairing.Account, bool, error) {
			opt, err := users.UseCases.FindByMail(user.SU(), user.Email(address))
			if err != nil || opt.IsNone() {
				return pairing.Account{}, false, err
			}

			// Registriert allein genügt nicht: Wer seine Adresse nicht
			// bestätigt hat oder gesperrt ist, koppelt keine Box.
			usr := opt.Unwrap()
			if !usr.EMailVerified || !usr.Enabled() {
				return pairing.Account{}, false, nil
			}

			return pairing.Account{ID: string(usr.ID), Mail: string(usr.Email)}, true, nil
		},
		Mailer: func(to, subject, body string) error {
			_, err := mails.UseCases.SendMail(user.SU(), mail.Mail{
				To:      []netmail.Address{{Address: to}},
				Subject: subject,
				Parts:   []mail.Part{mail.NewTextPart(body)},
			})

			return err
		},
		Issuer: func(account pairing.Account, device string) (string, error) {
			_, plain, err := tokens.UseCases.Create(user.SU(), token.CreationData{
				Name:        pairing.TokenName(device),
				Description: fmt.Sprintf("Gekoppelt von %s (Nutzer %s) am %s über den Einmalcode.", account.Mail, account.ID, time.Now().Format("02.01.2006 15:04")),
				Roles:       []role.ID{RelayRole},
			})

			return string(plain), err
		},
	}), nil
}
