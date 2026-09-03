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

	idInspectArchive permission.ID = "de.torbenschinke.eventprint.photo.archive.inspect"
	idExportArchive  permission.ID = "de.torbenschinke.eventprint.photo.archive.export"
	idPurgeEvent     permission.ID = "de.torbenschinke.eventprint.photo.purge_event"
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

	PermInspectArchive = permission.Declare[InspectArchive](idInspectArchive,
		permtext.Name(idInspectArchive, "Fotoarchiv einsehen", "Inspect the photo archive"),
		permtext.Description(idInspectArchive,
			"Träger dieser Berechtigung können sehen, wie viele Bilder im Archiv liegen und wie viel Platz sie belegen.",
			"Holders of this authorisation can see how many photos the archive holds and how much space they occupy."),
	)

	PermExportArchive = permission.Declare[ExportArchive](idExportArchive,
		permtext.Name(idExportArchive, "Fotoarchiv herunterladen", "Download the photo archive"),
		permtext.Description(idExportArchive,
			"Träger dieser Berechtigung können das gesamte Fotoarchiv als ZIP-Datei herunterladen.",
			"Holders of this authorisation can download the complete photo archive as a ZIP file."),
	)

	PermPurgeEvent = permission.Declare[PurgeEvent](idPurgeEvent,
		permtext.Name(idPurgeEvent, "Veranstaltung abschließen", "Finish the event"),
		permtext.Description(idPurgeEvent,
			"Träger dieser Berechtigung können alle Fotos, Bilddaten und Archivdateien endgültig entfernen, um die Fotobox für die nächste Veranstaltung freizuräumen.",
			"Holders of this authorisation can permanently remove all photos, image data and archive files to prepare the photo booth for the next event."),
	)
)
