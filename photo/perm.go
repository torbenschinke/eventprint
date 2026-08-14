package photo

import "go.wdy.de/nago/application/permission"

var (
	PermImport = permission.Declare[Import](
		"de.torbenschinke.eventprint.photo.import",
		"Fotos hinzufügen",
		"Träger dieser Berechtigung können neue Fotos in die Fotobox einspielen, z. B. per Upload oder Kamera.",
	)

	PermFindByID = permission.Declare[FindByID](
		"de.torbenschinke.eventprint.photo.find_by_id",
		"Ein Foto anzeigen",
		"Träger dieser Berechtigung können ein einzelnes Foto über seine ID anzeigen.",
	)

	PermFindAll = permission.Declare[FindAll](
		"de.torbenschinke.eventprint.photo.find_all",
		"Alle Fotos anzeigen",
		"Träger dieser Berechtigung können die gesamte Foto-Historie der Veranstaltung einsehen.",
	)

	PermDelete = permission.Declare[Delete](
		"de.torbenschinke.eventprint.photo.delete",
		"Fotos löschen",
		"Träger dieser Berechtigung können Fotos aus der Historie entfernen.",
	)
)
