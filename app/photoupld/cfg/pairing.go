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
	nagojson "go.wdy.de/nago/pkg/data/json"

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

	roles, err := cfg.RoleManagement()
	if err != nil {
		return pairing.UseCases{}, err
	}

	if _, err := roles.UseCases.Upsert(user.SU(), role.Role{ID: OwnerRole, Name: "Fotobox-Besitzer", Description: "Sieht und trennt die eigenen gekoppelten Fotoboxen. Wird beim ersten Koppeln vergeben."}); err != nil {
		return pairing.UseCases{}, err
	}

	if err := roles.UseCases.UpdatePermissions(user.SU(), OwnerRole, pairing.OwnerPermissions()); err != nil {
		return pairing.UseCases{}, err
	}

	store, err := cfg.EntityStore("de.torbenschinke.photoupld.boxes")
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
		Issuer: func(account pairing.Account, device string) (pairing.BoxID, string, error) {
			id, plain, err := tokens.UseCases.Create(user.SU(), token.CreationData{
				Name:        pairing.TokenName(device),
				Description: fmt.Sprintf("Gekoppelt von %s (Nutzer %s) am %s über den Einmalcode.", account.Mail, account.ID, time.Now().Format("02.01.2006 15:04")),
				Roles:       []role.ID{RelayRole},
			})

			return pairing.BoxID(id), string(plain), err
		},
		Revoker: func(id pairing.BoxID) error {
			// Ein Administrator hat das Token vielleicht schon gelöscht;
			// dann ist die Box ohnehin getrennt.
			opt, err := tokens.UseCases.FindByID(user.SU(), token.ID(id))
			if err != nil || opt.IsNone() {
				return err
			}

			return tokens.UseCases.Delete(user.SU(), token.ID(id))
		},
		Owner: func(accountID string) error {
			uid := user.ID(accountID)
			var have []role.ID
			for rid, err := range users.UseCases.ListRoles(user.SU(), uid) {
				if err != nil {
					return err
				}

				if rid == OwnerRole {
					return nil
				}

				have = append(have, rid)
			}

			// Die Rollenliste wird als Ganzes gesetzt; deshalb die vorhandenen
			// Rollen mitgeben, sonst gingen sie verloren.
			return users.UseCases.UpdateOtherRoles(user.SU(), uid, append(have, OwnerRole))
		},
		Boxes: nagojson.NewSloppyJSONRepository[pairing.Box, pairing.BoxID](store),
	}), nil
}
