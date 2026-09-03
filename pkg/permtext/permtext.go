// Package permtext bildet die Texte einer Berechtigung als übersetzbare
// Zeichenketten.
//
// Ohne diesen Umweg stünden Name und Beschreibung als deutsche Literale im
// Quelltext. Sie erscheinen aber in der Rollenverwaltung, und die spricht die
// Sprache ihres Benutzers – ein festverdrahteter Text ist dort eine
// Voraussetzung, die niemand geprüft hat.
package permtext

import (
	"github.com/worldiety/i18n"
	"go.wdy.de/nago/application/permission"
	"golang.org/x/text/language"
)

// Name bildet den Anzeigenamen einer Berechtigung.
//
// Der Schlüssel leitet sich aus der Kennung ab, damit er eindeutig ist und
// niemand ihn getrennt pflegen muss.
//
// i18n.MustString steht hier unmittelbar und nicht hinter einer weiteren
// Hilfsfunktion: Die Prüfung, ob die Texte übersetzbar sind, folgt einem
// Helfer genau einen Schritt weit.
func Name(id permission.ID, de, en string) string {
	return i18n.MustString(i18n.Key(string(id)+"_perm_name"), i18n.Values{
		language.German:  de,
		language.English: en,
	}).String()
}

// Description bildet die Beschreibung einer Berechtigung.
func Description(id permission.ID, de, en string) string {
	return i18n.MustString(i18n.Key(string(id)+"_perm_desc"), i18n.Values{
		language.German:  de,
		language.English: en,
	}).String()
}
