package upld

import "go.wdy.de/nago/application/permission"

type OpenSession func()
type PollJobs func()
type FetchImage func()
type AckJob func()

var (
	PermOpenSession = permission.Declare[OpenSession]("de.torbenschinke.photoupld.session", "Upload-Sitzung öffnen", "Darf eine neue transiente Upload-Identität anfordern.")
	PermPollJobs    = permission.Declare[PollJobs]("de.torbenschinke.photoupld.poll", "Uploads abrufen", "Darf wartende Druckaufträge abrufen.")
	PermFetchImage  = permission.Declare[FetchImage]("de.torbenschinke.photoupld.fetch", "Upload-Bild laden", "Darf Originalbilder wartender Druckaufträge laden.")
	PermAckJob      = permission.Declare[AckJob]("de.torbenschinke.photoupld.ack", "Upload bestätigen", "Darf übernommene Aufträge bestätigen und löschen.")
)

func RelayPermissions() []permission.ID {
	return []permission.ID{PermOpenSession, PermPollJobs, PermFetchImage, PermAckJob}
}
