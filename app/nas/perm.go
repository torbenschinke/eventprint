package nas

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Je Anwendungsfall genau eine Berechtigung. Alle gehören dem Besitzer: Auf
// dem NAS liegt die private Fotosammlung, und ein Gast im Kiosk hat darin
// nichts zu suchen.
const (
	idShares    permission.ID = "de.torbenschinke.eventprint.nas.shares"
	idBrowse    permission.ID = "de.torbenschinke.eventprint.nas.browse"
	idThumbnail permission.ID = "de.torbenschinke.eventprint.nas.thumbnail"
	idRead      permission.ID = "de.torbenschinke.eventprint.nas.read"
)

var (
	PermShares = permission.Declare[Shares](idShares,
		permtext.Name(idShares, "Freigaben eines NAS anzeigen", "List the shares of a NAS"),
		permtext.Description(idShares,
			"Träger dieser Berechtigung können sich probeweise an einem NAS anmelden und seine Freigaben sehen.",
			"Holders of this authorisation can try to sign in to a NAS and see its shares."),
	)

	PermBrowse = permission.Declare[Browse](idBrowse,
		permtext.Name(idBrowse, "NAS durchsuchen", "Browse the NAS"),
		permtext.Description(idBrowse,
			"Träger dieser Berechtigung können Ordner und Bilder auf der eingerichteten Freigabe auflisten.",
			"Holders of this authorisation can list folders and images on the configured share."),
	)

	PermThumbnail = permission.Declare[Thumbnail](idThumbnail,
		permtext.Name(idThumbnail, "Vorschaubild vom NAS laden", "Load a preview from the NAS"),
		permtext.Description(idThumbnail,
			"Träger dieser Berechtigung können Vorschaubilder der Fotos auf dem NAS laden.",
			"Holders of this authorisation can load previews of the photos on the NAS."),
	)

	PermRead = permission.Declare[Read](idRead,
		permtext.Name(idRead, "Bild vom NAS laden", "Load an image from the NAS"),
		permtext.Description(idRead,
			"Träger dieser Berechtigung können ein Bild vom NAS laden, etwa um es zu drucken.",
			"Holders of this authorisation can load an image from the NAS, for example to print it."),
	)
)

// Permissions nennt alle Berechtigungen des Pakets.
func Permissions() []permission.ID {
	return []permission.ID{PermShares, PermBrowse, PermThumbnail, PermRead}
}
