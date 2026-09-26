package printing

import (
	"go.wdy.de/nago/application/permission"

	"github.com/torbenschinke/eventprint/pkg/permtext"
)

const (
	idPrint       permission.ID = "de.torbenschinke.eventprint.printing.print"
	idFindAllJobs permission.ID = "de.torbenschinke.eventprint.printing.find_all_jobs"
	idFindJobByID permission.ID = "de.torbenschinke.eventprint.printing.find_job_by_id"
	idRetry       permission.ID = "de.torbenschinke.eventprint.printing.retry"
	idPreview     permission.ID = "de.torbenschinke.eventprint.printing.preview"
	idDiagnose    permission.ID = "de.torbenschinke.eventprint.printing.diagnose"
	idResume      permission.ID = "de.torbenschinke.eventprint.printing.resume"
	idPrintSimple permission.ID = "de.torbenschinke.eventprint.printing.print_simple"
	idCancel      permission.ID = "de.torbenschinke.eventprint.printing.cancel"
)

var (
	PermPrint = permission.Declare[Print](idPrint,
		permtext.Name(idPrint, "Fotos drucken", "Print photos"),
		permtext.Description(idPrint,
			"Träger dieser Berechtigung können Fotos auf dem Fotodrucker ausgeben.",
			"Holders of this authorisation can send photos to the printer."),
	)

	PermFindAllJobs = permission.Declare[FindAllJobs](idFindAllJobs,
		permtext.Name(idFindAllJobs, "Druckaufträge anzeigen", "Show print jobs"),
		permtext.Description(idFindAllJobs,
			"Träger dieser Berechtigung können den Zustand aller Druckaufträge einsehen.",
			"Holders of this authorisation can inspect the state of every print job."),
	)

	PermFindJobByID = permission.Declare[FindJobByID](idFindJobByID,
		permtext.Name(idFindJobByID, "Einen Druckauftrag anzeigen", "Show a single print job"),
		permtext.Description(idFindJobByID,
			"Träger dieser Berechtigung können einen einzelnen Druckauftrag über seine Kennung anzeigen.",
			"Holders of this authorisation can show a single print job by its identifier."),
	)

	PermRetry = permission.Declare[Retry](idRetry,
		permtext.Name(idRetry, "Druckaufträge wiederholen", "Retry print jobs"),
		permtext.Description(idRetry,
			"Träger dieser Berechtigung können fehlgeschlagene Druckaufträge erneut starten, etwa nach einem Papierwechsel.",
			"Holders of this authorisation can restart failed print jobs, for instance after changing the paper."),
	)

	PermPreview = permission.Declare[Preview](idPreview,
		permtext.Name(idPreview, "Druckvorschau ansehen", "See the print preview"),
		permtext.Description(idPreview,
			"Träger dieser Berechtigung können sehen, wie ein Foto im gewählten Layout gedruckt würde.",
			"Holders of this authorisation can see how a photo would look in the chosen layout."),
	)

	PermDiagnose = permission.Declare[Diagnose](idDiagnose,
		permtext.Name(idDiagnose, "Druckerzustand ansehen", "See the printer state"),
		permtext.Description(idDiagnose,
			"Träger dieser Berechtigung können den Zustand des Druckers einsehen, etwa ein leeres Papierfach.",
			"Holders of this authorisation can inspect the state of the printer, for instance an empty paper tray."),
	)

	PermResume = permission.Declare[Resume](idResume,
		permtext.Name(idResume, "Drucker freigeben", "Resume the printer"),
		permtext.Description(idResume,
			"Träger dieser Berechtigung können einen angehaltenen Drucker wieder freigeben, etwa nach einem Papierwechsel.",
			"Holders of this authorisation can resume a stopped printer, for instance after changing the paper."),
	)

	PermPrintSimple = permission.Declare[PrintSimple](idPrintSimple,
		permtext.Name(idPrintSimple, "Im Kiosk drucken", "Print in the kiosk"),
		permtext.Description(idPrintSimple,
			"Träger dieser Berechtigung drucken ein Foto in einem der Kiosk-Layouts, in begrenzter Anzahl.",
			"Holders of this authorisation print one photo in one of the kiosk layouts, in limited quantity."),
	)

	PermCancel = permission.Declare[Cancel](idCancel,
		permtext.Name(idCancel, "Druckaufträge abbrechen", "Cancel print jobs"),
		permtext.Description(idCancel,
			"Träger dieser Berechtigung brechen Aufträge ab, die noch nicht gedruckt sind.",
			"Holders of this authorisation cancel jobs that have not been printed yet."),
	)
)

// Permissions liefert alle Berechtigungen dieses Kontexts.
func Permissions() []permission.ID {
	return []permission.ID{
		PermPrint, PermPrintSimple, PermFindAllJobs, PermFindJobByID, PermRetry,
		PermPreview, PermDiagnose, PermResume, PermCancel,
	}
}
