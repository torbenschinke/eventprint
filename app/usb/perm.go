package usb

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Je Anwendungsfall genau eine Berechtigung. Alle gehören der Betreuung: Über
// den Stick verlassen sämtliche Fotos der Feier das Gerät, und ein Gast, der
// einen eigenen Stick einsteckt, soll nicht die Bilder aller anderen
// mitnehmen können. Auch das Durchsehen fremder Sticks ist nichts für den
// Kioskbetrieb.
const (
	idDrives permission.ID = "de.torbenschinke.eventprint.usb.drives"
	idExport permission.ID = "de.torbenschinke.eventprint.usb.export"
	idEject  permission.ID = "de.torbenschinke.eventprint.usb.eject"
	idImages permission.ID = "de.torbenschinke.eventprint.usb.images"
	idRead   permission.ID = "de.torbenschinke.eventprint.usb.read"
)

var (
	PermDrives = permission.Declare[Drives](idDrives,
		permtext.Name(idDrives, "USB-Sticks anzeigen", "Show USB drives"),
		permtext.Description(idDrives,
			"Träger dieser Berechtigung können sehen, welche USB-Sticks an der Fotobox stecken.",
			"Holders of this authorisation can see which USB drives are plugged into the photo booth."),
	)

	PermExport = permission.Declare[Export](idExport,
		permtext.Name(idExport, "Fotos auf USB-Stick kopieren", "Copy photos to a USB drive"),
		permtext.Description(idExport,
			"Träger dieser Berechtigung können Fotos der Fotobox auf einen USB-Stick kopieren.",
			"Holders of this authorisation can copy photos from the photo booth to a USB drive."),
	)

	PermEject = permission.Declare[Eject](idEject,
		permtext.Name(idEject, "USB-Stick auswerfen", "Eject a USB drive"),
		permtext.Description(idEject,
			"Träger dieser Berechtigung können einen USB-Stick sicher auswerfen.",
			"Holders of this authorisation can safely eject a USB drive."),
	)

	PermImages = permission.Declare[Images](idImages,
		permtext.Name(idImages, "Bilder auf USB-Stick durchsuchen", "Browse images on a USB drive"),
		permtext.Description(idImages,
			"Träger dieser Berechtigung können die Bilder auf einem USB-Stick auflisten.",
			"Holders of this authorisation can list the images on a USB drive."),
	)

	PermRead = permission.Declare[Read](idRead,
		permtext.Name(idRead, "Bild von USB-Stick laden", "Load an image from a USB drive"),
		permtext.Description(idRead,
			"Träger dieser Berechtigung können ein Bild von einem USB-Stick laden, etwa um es zu drucken.",
			"Holders of this authorisation can load an image from a USB drive, for example to print it."),
	)
)

// Permissions nennt alle Berechtigungen des Pakets, damit sie einer Rolle in
// einem Zug zugewiesen werden können.
func Permissions() []permission.ID {
	return []permission.ID{PermDrives, PermExport, PermEject, PermImages, PermRead}
}
