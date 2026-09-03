package cfgphotobox

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Configure ist der Anwendungsfall „die Fotobox einrichten".
//
// Er hat keine eigene Funktion, sondern benennt die Fähigkeit, die hinter den
// Einstellungen steht. Ohne einen solchen Namen fragte die Oberfläche weiter
// `Subject().Valid()` – also „ist angemeldet?" statt „darf einrichten?". Das
// sind verschiedene Fragen.
type Configure func()

const idConfigure permission.ID = "de.torbenschinke.eventprint.booth.configure"

// PermConfigure trägt der Betreuer, nicht der Gast.
var PermConfigure = permission.Declare[Configure](idConfigure,
	permtext.Name(idConfigure, "Fotobox einrichten", "Configure the photo booth"),
	permtext.Description(idConfigure,
		"Träger dieser Berechtigung können Drucker, Veranstaltung und Zugänge der Fotobox einstellen.",
		"Holders of this authorisation can configure the printer, the event and the access of the photo booth."),
)
