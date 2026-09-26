package upld

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

const (
	idOpenSession permission.ID = "de.torbenschinke.photoupld.session"
	idPollJobs    permission.ID = "de.torbenschinke.photoupld.poll"
	idFetchImage  permission.ID = "de.torbenschinke.photoupld.fetch"
	idAckJob      permission.ID = "de.torbenschinke.photoupld.ack"

	idRegisterOAuth permission.ID = "de.torbenschinke.photoupld.oauth.register"
	idCollectOAuth  permission.ID = "de.torbenschinke.photoupld.oauth.collect"
)

var (
	PermOpenSession = permission.Declare[OpenSession](idOpenSession,
		permtext.Name(idOpenSession, "Upload-Sitzung öffnen", "Open an upload session"),
		permtext.Description(idOpenSession,
			"Darf eine neue, kurzlebige Upload-Identität anfordern.",
			"May request a new, short-lived upload identity."),
	)

	PermPollJobs = permission.Declare[FindPendingJobs](idPollJobs,
		permtext.Name(idPollJobs, "Uploads abrufen", "Fetch uploads"),
		permtext.Description(idPollJobs,
			"Darf die eigenen wartenden Druckaufträge abrufen.",
			"May fetch its own pending print jobs."),
	)

	PermFetchImage = permission.Declare[OpenJobImage](idFetchImage,
		permtext.Name(idFetchImage, "Upload-Bild laden", "Load an uploaded image"),
		permtext.Description(idFetchImage,
			"Darf Originalbilder wartender Druckaufträge laden.",
			"May load the original images of pending print jobs."),
	)

	PermAckJob = permission.Declare[AckJob](idAckJob,
		permtext.Name(idAckJob, "Upload bestätigen", "Acknowledge an upload"),
		permtext.Description(idAckJob,
			"Darf übernommene Aufträge bestätigen und damit löschen.",
			"May acknowledge and thereby delete jobs it has taken over."),
	)

	PermRegisterOAuth = permission.Declare[RegisterOAuth](idRegisterOAuth,
		permtext.Name(idRegisterOAuth, "Anmeldung vermitteln", "Relay a sign-in"),
		permtext.Description(idRegisterOAuth,
			"Darf eine Anmeldung bei einem Fremddienst über das Relais leiten.",
			"May route a sign-in with a third-party service through the relay."),
	)

	PermCollectOAuth = permission.Declare[CollectOAuth](idCollectOAuth,
		permtext.Name(idCollectOAuth, "Anmeldung abholen", "Collect a sign-in"),
		permtext.Description(idCollectOAuth,
			"Darf den Code einer selbst angemeldeten Anmeldung abholen.",
			"May collect the code of a sign-in it registered itself."),
	)
)

// RelayPermissions ist die Rolle, die eine Fotobox am Upload-Service braucht.
func RelayPermissions() []permission.ID {
	return []permission.ID{PermOpenSession, PermPollJobs, PermFetchImage, PermAckJob, PermRegisterOAuth, PermCollectOAuth}
}
