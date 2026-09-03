package cfgphotobox

import (
	"iter"

	"github.com/worldiety/i18n"
	"go.wdy.de/nago/application/permission"
	"go.wdy.de/nago/application/role"
	"go.wdy.de/nago/auth"
	"golang.org/x/text/language"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Configure ist der Anwendungsfall „die Fotobox einrichten".
//
// Er hat keine eigene Funktion, sondern benennt die Fähigkeit, die hinter den
// Einstellungen steht. Ohne einen solchen Namen fragte die Oberfläche weiter
// `Subject().Valid()` – also „ist angemeldet?" statt „darf einrichten?“. Das
// sind verschiedene Fragen, und die Verwechslung sperrt jede Anmeldeart aus,
// die kein Benutzerkonto benutzt.
type Configure func()

const idConfigure permission.ID = "de.torbenschinke.eventprint.booth.configure"

// PermConfigure trägt der Betreuer, nicht der Gast.
var PermConfigure = permission.Declare[Configure](idConfigure,
	permtext.Name(idConfigure, "Fotobox einrichten", "Configure the photo booth"),
	permtext.Description(idConfigure,
		"Träger dieser Berechtigung können Drucker, Veranstaltung und Zugänge der Fotobox einstellen.",
		"Holders of this authorisation can configure the printer, the event and the access of the photo booth."),
)

// unlockedSubject ist der Betreuer nach richtiger PIN.
//
// Es umhüllt das bestehende Subject der Sitzung – im Regelfall den Gast – und
// legt die Betreuerrechte darüber. Der Umweg über ein Benutzerkonto entfällt
// damit: Es gibt vor Ort niemanden, der eine Mailadresse bestätigen könnte,
// und ein Konto, das nie jemand benutzt, wäre nur ein weiteres Geheimnis.
//
// Alles, was nicht mit Rechten zu tun hat – Sprache, Textbündel, Kontext –
// beantwortet weiter das umhüllte Subject. Sonst spräche die Oberfläche nach
// der Anmeldung plötzlich eine andere Sprache.
type unlockedSubject struct {
	auth.Subject

	perms map[permission.ID]struct{}
	roles []role.ID
}

// newUnlockedSubject legt die Betreuerrechte über das Subject der Sitzung.
func newUnlockedSubject(base auth.Subject) auth.Subject {
	perms := map[permission.ID]struct{}{}
	for _, p := range operatorPermissions() {
		perms[p] = struct{}{}
	}

	return unlockedSubject{
		Subject: base,
		perms:   perms,
		roles:   []role.ID{OperatorRole},
	}
}

func (s unlockedSubject) HasPermission(p permission.ID) bool {
	if _, ok := s.perms[p]; ok {
		return true
	}

	return s.Subject.HasPermission(p)
}

func (s unlockedSubject) Audit(p permission.ID) error {
	if _, ok := s.perms[p]; ok {
		return nil
	}

	return s.Subject.Audit(p)
}

func (s unlockedSubject) HasRole(id role.ID) bool {
	for _, r := range s.roles {
		if r == id {
			return true
		}
	}

	return s.Subject.HasRole(id)
}

func (s unlockedSubject) Roles() iter.Seq[role.ID] {
	return func(yield func(role.ID) bool) {
		for _, r := range s.roles {
			if !yield(r) {
				return
			}
		}

		for r := range s.Subject.Roles() {
			if !yield(r) {
				return
			}
		}
	}
}

// SetLanguage und SetBundle reicht Nago beim Setzen des Subjects herein.
//
// Die beiden Methoden stehen in keinem exportierten Interface; Nago prüft sie
// strukturell. Ohne sie bliebe die Hülle stumm und die Oberfläche fiele nach
// der PIN-Eingabe auf die Standardsprache zurück.
func (s unlockedSubject) SetLanguage(tag language.Tag) {
	if setter, ok := s.Subject.(interface{ SetLanguage(language.Tag) }); ok {
		setter.SetLanguage(tag)
	}
}

func (s unlockedSubject) SetBundle(bundle *i18n.Bundle) {
	if setter, ok := s.Subject.(interface{ SetBundle(*i18n.Bundle) }); ok {
		setter.SetBundle(bundle)
	}
}
