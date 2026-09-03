package upld

import "go.wdy.de/nago/application/permission"

// Jede Berechtigung hängt an genau einem Anwendungsfall. Das macht sie
// zuteilbar und prüfbar: Wer wissen will, was ein Zugangstoken darf, liest die
// Liste der Anwendungsfälle und nicht den Quelltext der Routen.
var (
	PermOpenSession = permission.Declare[OpenSession]("de.torbenschinke.photoupld.session", "Upload-Sitzung öffnen", "Darf eine neue transiente Upload-Identität anfordern.")
	PermPollJobs    = permission.Declare[FindPendingJobs]("de.torbenschinke.photoupld.poll", "Uploads abrufen", "Darf wartende Druckaufträge abrufen.")
	PermFetchImage  = permission.Declare[OpenJobImage]("de.torbenschinke.photoupld.fetch", "Upload-Bild laden", "Darf Originalbilder wartender Druckaufträge laden.")
	PermAckJob      = permission.Declare[AckJob]("de.torbenschinke.photoupld.ack", "Upload bestätigen", "Darf übernommene Aufträge bestätigen und löschen.")
)

// RelayPermissions ist die Rolle, die eine Fotobox am Upload-Service braucht.
func RelayPermissions() []permission.ID {
	return []permission.ID{PermOpenSession, PermPollJobs, PermFetchImage, PermAckJob}
}
