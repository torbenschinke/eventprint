package photo

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

// Je Anwendungsfall genau eine Berechtigung. Welche davon ein Gast im Kiosk
// hat, entscheidet die Rolle im Gerätekontext – nicht die Oberfläche.
const (
	idImport         permission.ID = "de.torbenschinke.eventprint.photo.import"
	idFindAll        permission.ID = "de.torbenschinke.eventprint.photo.find_all"
	idFindEvent      permission.ID = "de.torbenschinke.eventprint.photo.find_event"
	idFindByID       permission.ID = "de.torbenschinke.eventprint.photo.find_by_id"
	idDelete         permission.ID = "de.torbenschinke.eventprint.photo.delete"
	idSetFavorite    permission.ID = "de.torbenschinke.eventprint.photo.set_favorite"
	idMarkSeen       permission.ID = "de.torbenschinke.eventprint.photo.mark_seen"
	idMarkPrinted    permission.ID = "de.torbenschinke.eventprint.photo.mark_printed"
	idOpenOriginal   permission.ID = "de.torbenschinke.eventprint.photo.open_original"
	idLocate         permission.ID = "de.torbenschinke.eventprint.photo.locate"
	idInspectStorage permission.ID = "de.torbenschinke.eventprint.photo.inspect_storage"
	idPurgeEvent     permission.ID = "de.torbenschinke.eventprint.photo.purge_event"
)

var (
	PermImport = permission.Declare[Import](idImport,
		permtext.Name(idImport, "Fotos hinzufügen", "Add photos"),
		permtext.Description(idImport,
			"Träger dieser Berechtigung können Fotos aus Kamera, Handy, Lightroom oder USB-Stick übernehmen.",
			"Holders of this authorisation can add photos from the camera, a phone, Lightroom or a USB stick."),
	)

	PermFindAll = permission.Declare[FindAll](idFindAll,
		permtext.Name(idFindAll, "Mediathek ansehen", "Browse the library"),
		permtext.Description(idFindAll,
			"Träger dieser Berechtigung sehen alle Fotos, auch die privaten.",
			"Holders of this authorisation can see every photo, including private ones."),
	)

	PermFindEvent = permission.Declare[FindEvent](idFindEvent,
		permtext.Name(idFindEvent, "Fotos einer Feier ansehen", "Show the photos of an event"),
		permtext.Description(idFindEvent,
			"Träger dieser Berechtigung sehen die Fotos genau einer Feier und keine privaten.",
			"Holders of this authorisation can see the photos of exactly one event and no private ones."),
	)

	PermFindByID = permission.Declare[FindByID](idFindByID,
		permtext.Name(idFindByID, "Ein Foto anzeigen", "Show a single photo"),
		permtext.Description(idFindByID,
			"Träger dieser Berechtigung können ein einzelnes Foto über seine Kennung abrufen.",
			"Holders of this authorisation can fetch a single photo by its identifier."),
	)

	PermDelete = permission.Declare[Delete](idDelete,
		permtext.Name(idDelete, "Fotos löschen", "Delete photos"),
		permtext.Description(idDelete,
			"Träger dieser Berechtigung können Fotos samt Original endgültig entfernen.",
			"Holders of this authorisation can permanently remove photos including their original."),
	)

	PermSetFavorite = permission.Declare[SetFavorite](idSetFavorite,
		permtext.Name(idSetFavorite, "Favoriten markieren", "Mark favourites"),
		permtext.Description(idSetFavorite,
			"Träger dieser Berechtigung können Fotos als Favorit markieren.",
			"Holders of this authorisation can mark photos as favourites."),
	)

	PermMarkSeen = permission.Declare[MarkSeen](idMarkSeen,
		permtext.Name(idMarkSeen, "Eingang abarbeiten", "Clear the inbox"),
		permtext.Description(idMarkSeen,
			"Träger dieser Berechtigung können neue Fotos als gesehen markieren.",
			"Holders of this authorisation can mark new photos as seen."),
	)

	PermMarkPrinted = permission.Declare[MarkPrinted](idMarkPrinted,
		permtext.Name(idMarkPrinted, "Druck vermerken", "Record a print"),
		permtext.Description(idMarkPrinted,
			"Träger dieser Berechtigung vermerken, dass ein Foto gedruckt wurde.",
			"Holders of this authorisation record that a photo has been printed."),
	)

	PermOpenOriginal = permission.Declare[OpenOriginal](idOpenOriginal,
		permtext.Name(idOpenOriginal, "Originaldaten lesen", "Read original data"),
		permtext.Description(idOpenOriginal,
			"Träger dieser Berechtigung können die unveränderten Originaldaten eines Fotos lesen, etwa als Vorlage für den Druck.",
			"Holders of this authorisation can read the untouched original data of a photo, for instance as the source for printing."),
	)

	PermLocate = permission.Declare[Locate](idLocate,
		permtext.Name(idLocate, "Ablageort ermitteln", "Locate originals"),
		permtext.Description(idLocate,
			"Träger dieser Berechtigung erfahren, wo das Original eines Fotos liegt, etwa für Vorschaubilder oder den Export.",
			"Holders of this authorisation learn where the original of a photo is stored, for thumbnails or export."),
	)

	PermInspectStorage = permission.Declare[InspectStorage](idInspectStorage,
		permtext.Name(idInspectStorage, "Speicherplatz einsehen", "Inspect storage"),
		permtext.Description(idInspectStorage,
			"Träger dieser Berechtigung sehen, wie viel Platz die Fotos belegen und wie viel frei ist.",
			"Holders of this authorisation can see how much space the photos occupy and how much is free."),
	)

	PermPurgeEvent = permission.Declare[PurgeEvent](idPurgeEvent,
		permtext.Name(idPurgeEvent, "Feier abschließen", "Finish an event"),
		permtext.Description(idPurgeEvent,
			"Träger dieser Berechtigung können alle Fotos einer Feier endgültig entfernen.",
			"Holders of this authorisation can permanently remove all photos of an event."),
	)
)

// Permissions liefert alle Berechtigungen dieses Kontexts.
func Permissions() []permission.ID {
	return []permission.ID{
		PermImport, PermFindAll, PermFindEvent, PermFindByID, PermDelete, PermSetFavorite,
		PermMarkSeen, PermMarkPrinted, PermOpenOriginal, PermLocate, PermInspectStorage, PermPurgeEvent,
	}
}
