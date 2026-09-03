package photo

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Je Anwendungsfall genau eine Berechtigung. Das macht sie zuteilbar und
// prüfbar: Wer wissen will, was eine Rolle darf, liest die Liste der
// Anwendungsfälle und nicht den Quelltext der Oberfläche.
const (
	idImport       permission.ID = "de.torbenschinke.eventprint.photo.import"
	idFindByID     permission.ID = "de.torbenschinke.eventprint.photo.find_by_id"
	idFindAll      permission.ID = "de.torbenschinke.eventprint.photo.find_all"
	idFindLatest   permission.ID = "de.torbenschinke.eventprint.photo.find_latest"
	idDelete       permission.ID = "de.torbenschinke.eventprint.photo.delete"
	idOpenOriginal permission.ID = "de.torbenschinke.eventprint.photo.open_original"
)

var (
	PermImport = permission.Declare[Import](idImport,
		permtext.Name(idImport, "Fotos hinzufügen", "Add photos"),
		permtext.Description(idImport,
			"Träger dieser Berechtigung können neue Fotos in die Fotobox einspielen, z. B. per Upload oder Kamera.",
			"Holders of this authorisation can add new photos, for example by upload or from the camera."),
	)

	PermFindByID = permission.Declare[FindByID](idFindByID,
		permtext.Name(idFindByID, "Ein Foto anzeigen", "Show a single photo"),
		permtext.Description(idFindByID,
			"Träger dieser Berechtigung können ein einzelnes Foto über seine Kennung anzeigen.",
			"Holders of this authorisation can show a single photo by its identifier."),
	)

	PermFindAll = permission.Declare[FindAll](idFindAll,
		permtext.Name(idFindAll, "Alle Fotos anzeigen", "Show all photos"),
		permtext.Description(idFindAll,
			"Träger dieser Berechtigung können die gesamte Foto-Historie der Veranstaltung einsehen.",
			"Holders of this authorisation can browse the complete photo history of the event."),
	)

	PermFindLatest = permission.Declare[FindLatest](idFindLatest,
		permtext.Name(idFindLatest, "Neueste Fotos anzeigen", "Show the latest photos"),
		permtext.Description(idFindLatest,
			"Träger dieser Berechtigung können die jüngsten Fotos sehen, wie sie der Startbildschirm zeigt.",
			"Holders of this authorisation can see the most recent photos as shown on the booth screen."),
	)

	PermDelete = permission.Declare[Delete](idDelete,
		permtext.Name(idDelete, "Fotos löschen", "Delete photos"),
		permtext.Description(idDelete,
			"Träger dieser Berechtigung können Fotos aus der Historie entfernen.",
			"Holders of this authorisation can remove photos from the history."),
	)

	PermOpenOriginal = permission.Declare[OpenOriginal](idOpenOriginal,
		permtext.Name(idOpenOriginal, "Originaldaten lesen", "Read original data"),
		permtext.Description(idOpenOriginal,
			"Träger dieser Berechtigung können die unveränderten Originaldaten eines Fotos lesen, etwa als Vorlage für den Druck.",
			"Holders of this authorisation can read the untouched original data of a photo, for instance as the source for printing."),
	)
)
