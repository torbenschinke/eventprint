package printing

import "go.wdy.de/nago/application/permission"

var (
	PermPrint = permission.Declare[Print](
		"de.torbenschinke.eventprint.printing.print",
		"Fotos drucken",
		"Träger dieser Berechtigung können Fotos auf dem Fotodrucker ausgeben.",
	)

	PermFindAllJobs = permission.Declare[FindAllJobs](
		"de.torbenschinke.eventprint.printing.find_all_jobs",
		"Druckaufträge anzeigen",
		"Träger dieser Berechtigung können den Status aller Druckaufträge einsehen.",
	)

	PermFindJobByID = permission.Declare[FindJobByID](
		"de.torbenschinke.eventprint.printing.find_job_by_id",
		"Einen Druckauftrag anzeigen",
		"Träger dieser Berechtigung können einen einzelnen Druckauftrag über seine ID anzeigen.",
	)

	PermRetry = permission.Declare[Retry](
		"de.torbenschinke.eventprint.printing.retry",
		"Druckaufträge wiederholen",
		"Träger dieser Berechtigung können fehlgeschlagene Druckaufträge erneut starten, etwa nach einem Papierwechsel.",
	)
)
